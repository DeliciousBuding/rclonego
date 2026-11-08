package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/internal/version"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	nativeupdater "github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

// MyGo owns update downloads, signature verification, installation and the
// update window. This only keeps the application's stable/preview preference.
// Like MyGo's plugin, its UI state is used on the native main thread.
var updateSettings struct {
	ready   bool
	enabled bool
	renamed bool
	channel string
	file    string
	err     error
}

type updateChannelPreference struct {
	Channel string `json:"channel"`
}

func configureUpdates(_ *core.App) {
	mygo.Use(nativeupdater.New(updater.Options{
		Icon: trayIcon, DisableAutomaticChecks: true,
	}))
	mygo.App.WhenReady(func() {
		updateSettings.ready = true
		updateSettings.channel = defaultUpdateChannel()
		directory, err := mygo.App.Path(mygo.PathUserData)
		if err == nil {
			updateSettings.file = filepath.Join(directory, "update-channel.json")
			updateSettings.channel, err = loadUpdateChannel(updateSettings.file)
		}
		updateSettings.err = err
		mygo.Updater.SetPrereleases(updateSettings.channel == "preview")
		executable, executableErr := os.Executable()
		updateSettings.renamed = !mygo.IsDev() && executableErr == nil &&
			!selfUpdateExecutableSupported(executable, runtime.GOOS)
		updateSettings.enabled = mygo.Updater.Enabled() && !updateSettings.renamed
		if updateSettings.renamed {
			// MyGo replaces the archive's executable by its original name;
			// relaunch uses the executable this process started from. A bare
			// portable executable renamed by its download cannot self-update.
			updater.SetAutomaticChecks(false)
			updater.SetAutomaticDownloads(false)
		}
		updater.OnChange(func() {
			for _, window := range mygo.Windows() {
				window.Invalidate()
			}
		})
	})
}

func updatesMenuItem() *mygo.MenuItem {
	item := updater.MenuItem()
	click := item.Click
	item.Click = func(item *mygo.MenuItem, window *mygo.Window) {
		if updateSettings.ready && !updateSettings.enabled {
			_ = mygo.Shell.OpenExternal(projectURL + "/releases")
			return
		}
		click(item, window)
	}
	return item
}

func selfUpdateExecutableSupported(path, operatingSystem string) bool {
	return operatingSystem != "windows" || strings.EqualFold(filepath.Base(path), "RcloneGo.exe")
}

// updateRestartBlocked is used before quitting for an update. The explicit
// Quit menu action may bypass it. MyGo stages downloaded updates without
// stopping this process, then applies them on relaunch/next start.
func updateRestartBlocked(app *core.App) bool {
	if !updateSettings.ready || updater.AvailableVersion() == "" {
		return false
	}
	return !app.Health().Ready || updateBusyReason(app.Snapshot(), app.RuntimeSnapshot().Mounts) != ""
}

func updateBusyReason(state core.State, mounts []core.MountView) string {
	for _, job := range state.Jobs {
		if job.Status == "running" {
			return "任务正在运行，更新将在空闲后重启应用"
		}
	}
	for _, mount := range mounts {
		if mount.Running && state.OwnedMounts[mount.ID] > 0 &&
			(mount.PID == 0 || state.OwnedMounts[mount.ID] == mount.PID) {
			return "映射仍在使用，卸载后可重启应用完成更新"
		}
	}
	return ""
}

func (s *nativeUI) updatesPreferences(c *ui.Context) {
	t := c.Theme()
	sectionHeading(c, "客户端更新")
	ui.Row(c).Gap(10).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			if updateSettings.enabled {
				ui.Text(c, text(c, "支持签名更新")).FontSize(12).FontWeight(500)
				ui.Text(c, text(c, "安装包经过签名校验；正在运行的任务不会因更新而重启。")).
					FontSize(11).TextColor(t.TextMuted).MaxLines(3)
			} else {
				ui.Text(c, text(c, "此构建使用手动更新")).FontSize(12).FontWeight(500)
				explanation := "开发构建、未配置签名的构建和受保护的安装目录使用发行页或包管理器更新。"
				if updateSettings.renamed {
					explanation = "独立下载的 Windows exe 使用发行页更新；解压版和安装版支持签名更新。"
				}
				ui.Text(c, text(c, explanation)).
					FontSize(11).TextColor(t.TextMuted).MaxLines(3)
			}
		})
		actionButton(c.Key("check-client-updates"), iconDownload, "检查更新…", false).
			Disabled(!updateSettings.ready).OnClick(func() {
			if !updateSettings.enabled {
				if err := mygo.Shell.OpenExternal(projectURL + "/releases"); err != nil {
					s.setError(err)
				}
				return
			}
			updater.CheckForUpdates()
		})
	})
	if updateSettings.err != nil {
		ui.Text(c, text(c, "无法读取更新通道设置")).FontSize(11).TextColor(t.Warning)
	}
	if !updateSettings.ready {
		return
	}
	choice := "稳定版"
	if updateSettings.channel == "preview" {
		choice = "预览版"
	}
	translatedSelect(c.Key("update-channel"), &choice, []string{"稳定版", "预览版"}).
		Label(text(c, "更新通道")).Width(180).OnChange(func() {
		channel := "stable"
		if choice == "预览版" {
			channel = "preview"
		}
		if err := saveUpdateChannel(updateSettings.file, channel); err != nil {
			s.setError(err)
			return
		}
		updateSettings.channel = channel
		mygo.Updater.SetPrereleases(channel == "preview")
	})
	ui.Text(c, text(c, "切回稳定版会等待更新的稳定版本，不会降级当前应用。")).
		FontSize(11).TextColor(t.TextMuted)
	automatic := updater.AutomaticChecks()
	ui.Checkbox(c.Key("automatic-update-checks"), &automatic, text(c, "自动检查更新")).
		Disabled(!updateSettings.enabled).OnChange(func() {
		updater.SetAutomaticChecks(automatic)
	})
	if checked := updater.LastCheck(); !checked.IsZero() {
		ui.Text(c, text(c, "最近检查 · ")+dateTime(c, checked)).
			FontSize(11).TextColor(t.TextMuted)
	}
	for _, job := range s.jobs {
		if job.Status == "running" {
			ui.Text(c, text(c, "更新重启会等待当前任务结束")).FontSize(11).TextColor(t.TextMuted)
			break
		}
	}
}

func loadUpdateChannel(path string) (string, error) {
	fallback := defaultUpdateChannel()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fallback, nil
	}
	if err != nil {
		return fallback, err
	}
	var preference updateChannelPreference
	if err := json.Unmarshal(data, &preference); err != nil {
		return fallback, err
	}
	if preference.Channel != "stable" && preference.Channel != "preview" {
		return fallback, errors.New("invalid update channel")
	}
	return preference.Channel, nil
}

func defaultUpdateChannel() string {
	return updateChannelForVersion(version.Version)
}

func updateChannelForVersion(value string) string {
	if strings.Contains(strings.SplitN(value, "+", 2)[0], "-") {
		return "preview"
	}
	return "stable"
}

func saveUpdateChannel(path, channel string) error {
	if channel != "stable" && channel != "preview" || path == "" {
		return errors.New("invalid update channel")
	}
	data, err := json.Marshal(updateChannelPreference{Channel: channel})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".update-channel-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	writeErr := file.Chmod(0o600)
	if writeErr == nil {
		_, writeErr = file.Write(append(data, '\n'))
	}
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
