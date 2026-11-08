package desktop

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func mountedRoot(value string) string {
	if runtime.GOOS == "windows" && len(value) == 2 && strings.HasSuffix(value, ":") {
		return value + `\`
	}
	return filepath.Clean(value)
}

func (s *nativeUI) openMountedPath(mount core.MountView) {
	if !mount.DriveAvailable {
		return
	}
	if err := mygo.Shell.OpenPath(mountedRoot(mount.Drive)); err != nil {
		s.setError(err)
	}
}

func mountBrowsePath(mount core.MountView) string {
	if mount.External && mount.DriveAvailable {
		return mountedRoot(mount.Drive)
	}
	return mount.Remote
}

func (s *nativeUI) mountDependencyHint(c *ui.Context) {
	targets := s.app.MountTargets()
	if targets.Dependency.Status == "ready" {
		return
	}
	url, label := "https://rclone.org/commands/rclone_mount/", "查看挂载安装说明"
	if runtime.GOOS == "windows" {
		url, label = "https://winfsp.dev", "安装 WinFsp"
	}
	ui.Row(c).Shrink(0).Padding(10, 12).Gap(10).Radius(8).Background(c.Theme().Warning.Alpha(0.08)).Children(func() {
		ui.Icon(c, iconInfo).FontSize(16).TextColor(c.Theme().Warning)
		ui.Text(c, text(c, "挂载需要系统文件系统组件，文件传输仍可使用。")).Grow(1).MinWidth(0).FontSize(12).MaxLines(2)
		ui.Button(c, text(c, label)).OnClick(func() { mygo.Shell.OpenExternal(url) })
	})
}
