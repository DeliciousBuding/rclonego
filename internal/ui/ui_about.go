package desktop

import (
	"github.com/DeliciousBuding/rclonego/internal/version"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const projectURL = "https://github.com/DeliciousBuding/rclonego"

func (s *nativeUI) aboutContent(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(14).Children(func() {
		ui.Box(c).Size(48, 48).Radius(12).Background(t.Accent.Alpha(.08)).Center().Children(func() {
			ui.Icon(c, appMark).FontSize(32).TextColor(t.Accent)
		})
		ui.Column(c).Gap(4).MinWidth(0).Children(func() {
			ui.Text(c, "RcloneGo").FontSize(22).FontWeight(600)
			ui.Text(c, version.Version+" · Apache-2.0").FontSize(12).TextColor(t.TextMuted)
		})
	})
	ui.Text(c, "可靠传输，过程可见。").FontSize(13)
	ui.Text(c, "由 rclone 驱动，使用 MyGo 原生 UI。").FontSize(12).TextColor(t.TextMuted)
	ui.Text(c, "独立社区项目；存储服务的标志仅用于标识集成。").FontSize(11).TextColor(t.TextMuted)
	ui.Divider(c)
	link := func(key, title, url string, icon *ui.SVG) {
		actionButton(c.Key(key), icon, title, false).OnClick(func() {
			if err := mygo.Shell.OpenExternal(url); err != nil {
				s.setError(err)
			}
		})
	}
	ui.Row(c).Gap(8).Wrap().Children(func() {
		link("repository", "GitHub", projectURL, githubLogo)
		link("issues", "反馈问题", projectURL+"/issues/new/choose", iconInfo)
		link("releases", "版本发布", projectURL+"/releases", iconDownload)
	})
	ui.Text(c, "WinFsp / FUSE 是单独安装的挂载组件。").FontSize(11).TextColor(t.TextMuted)
	ui.Text(c, "许可证和图标来源见仓库 docs/third-party.md。").FontSize(11).TextColor(t.TextMuted)
}
