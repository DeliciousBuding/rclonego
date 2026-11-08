package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
	"runtime"
	"strings"
)

func (s *nativeUI) mountEditor(c *ui.Context) {
	ui.Form(c, func() {
		ui.Field(c, text(c, "名称"), func() { ui.TextInput(c.Key("mount-name"), &s.mountName).AutoFocus() })
		ui.Field(c, text(c, "远端"), func() { s.storagePathInput(c.Key("mount-remote"), &s.mountRemote, "远端", false) })
		label, placeholder := "挂载目录", "/path/to/empty-directory"
		if runtime.GOOS == "windows" {
			label, placeholder = "盘符", "K:"
		}
		ui.Field(c, text(c, label), func() {
			if runtime.GOOS == "windows" {
				targets := s.app.MountTargets().Available
				if s.mountDrive != "" && !containsTarget(targets, s.mountDrive) {
					targets = append([]string{s.mountDrive}, targets...)
				}
				ui.Select(c.Key("mount-drive"), &s.mountDrive, targets).Label(text(c, "盘符"))
			} else {
				s.pathInput(c.Key("mount-drive"), &s.mountDrive, label, placeholder, true, false, nil)
			}
		})
		ui.Field(c, text(c, "缓存目录"), func() {
			s.pathInput(c.Key("mount-cache"), &s.mountCache, "缓存目录", "选择本机 SSD 目录", true, false, nil)
		})
	})
	ui.Collapsible(c.Key("mount-advanced"), text(c, "更多选项"), &s.showMountAdvanced, func() {
		ui.Form(c, func() {
			ui.Field(c, text(c, "卷标"), func() { ui.TextInput(c.Key("mount-volume"), &s.mountVolume) })
			ui.Field(c, text(c, "缓存上限"), func() { ui.TextInput(c.Key("mount-max"), &s.mountMax).Placeholder("15G") })
			ui.Field(c, text(c, "缓存保留时间"), func() { ui.TextInput(c.Key("mount-age"), &s.mountAge).Placeholder("24h") })
		})
	})
}

func (s *nativeUI) mountFooter(c *ui.Context) {
	ui.Row(c).Shrink(0).Gap(8).Justify(ui.End).Children(func() {
		ui.Button(c.Key("cancel-mount"), text(c, "取消")).Height(30).OnClick(func() { s.editingMount = false })
		ui.PrimaryButton(c.Key("save-mount"), text(c, "保存")).Height(30).Disabled(s.busy).OnClick(s.saveMount)
	})
}

func containsTarget(targets []string, target string) bool {
	for _, candidate := range targets {
		if core.MountTargetsEqual(candidate, target) {
			return true
		}
	}
	return false
}

func (s *nativeUI) saveMount() {
	target := core.NormalizeMountTarget(s.mountDrive)
	cfg := core.MountConfig{ID: s.mountID, Name: strings.TrimSpace(s.mountName), Remote: core.EnsureColonRemote(strings.TrimSpace(s.mountRemote)), Drive: target, VolumeName: strings.TrimSpace(s.mountVolume), CacheDir: strings.TrimSpace(s.mountCache), CacheMax: strings.TrimSpace(s.mountMax), CacheMaxAge: strings.TrimSpace(s.mountAge)}
	if cfg.ID == "" {
		id, err := core.RandomID()
		if err != nil {
			s.setError(err)
			return
		}
		cfg.ID = "mount-" + id[:6]
	}
	s.runAfter(locales.Text("保存映射"), func() error {
		if err := core.ValidateMountConfig(cfg); err != nil {
			return err
		}
		mounts := s.app.Snapshot().Mounts
		for i := range mounts {
			if mounts[i].ID == cfg.ID {
				mounts[i] = cfg
				return s.app.SaveMounts(mounts)
			}
			if core.MountTargetsEqual(mounts[i].Drive, cfg.Drive) {
				return fmt.Errorf("target %s is already mapped", cfg.Drive)
			}
		}
		return s.app.SaveMounts(append(mounts, cfg))
	}, func() { s.editingMount = false })
}
