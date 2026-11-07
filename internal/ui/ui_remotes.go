package desktop

import (
	"github.com/DeliciousBuding/rclonego/internal/core"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) remotesPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(18).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "连接云端账户，供映射和传输任务使用。").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.PrimaryButton(c.Key("show-remote-form"), "添加账户…").Height(30).
				Disabled(s.busy).OnClick(func() { s.showRemoteForm = true })
		})
		ui.Scroll(c).Grow(1).Gap(0).Children(func() {
			if len(s.remotes) == 0 {
				emptyState(c, iconRemotes, "暂无云端账户", "添加 rclone 远端后即可开始使用")
			}
			for _, remote := range s.remotes {
				ui.Row(c).Padding(18, 0).Gap(14).BorderWidth(0, 0, 0.5, 0).
					BorderColor(t.Border).Children(func() {
					ui.Box(c).Size(36, 36).Radius(10).Center().Background(t.Accent.Alpha(0.07)).
						Children(func() { remoteLogo(c, remote.Type, 24) })
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Text(c, remote.Name).FontSize(13).FontWeight(600)
						label := remote.Type
						if label == "drive" {
							label = "Google Drive"
						}
						ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
					})
					ui.Button(c.Key("oauth-"+remote.Name), "重新授权…").Height(28).Padding(4, 10).
						Disabled(s.busy).OnClick(func() {
						name := remote.Name
						s.run("重新授权", func() error {
							_, err := s.app.AddJob(core.JobCreateRequest{Name: "重新授权 " + name, Operation: "oauth-reconnect", Source: name})
							return err
						})
					})
				})
			}
		})
	})
}

func (s *nativeUI) remoteEditor(c *ui.Context) {
	if s.newRemoteType == "drive" {
		ui.Row(c).Gap(10).Children(func() {
			brandIcon(c, googleGLogo, 20).Label("Google")
			ui.Text(c, "连接 Google Drive").FontSize(13).FontWeight(500)
		})
	}
	ui.Form(c, func() {
		ui.Field(c, "远端名称", func() { ui.TextInput(c.Key("new-remote-name"), &s.newRemoteName).Placeholder("drive-work").AutoFocus() })
		ui.Field(c, "服务类型", func() {
			if ui.Select(c.Key("new-remote-type"), &s.remoteTypeChoice, []string{"Google Drive", "OneDrive"}).Changed() {
				s.newRemoteType = map[string]string{"Google Drive": "drive", "OneDrive": "onedrive"}[s.remoteTypeChoice]
			}
		})
	})
	ui.Text(c, "通过 rclone 完成授权；需要补充配置时可在任务详情中查看。").
		FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		ui.Button(c.Key("cancel-remote"), "取消").Height(30).OnClick(func() { s.showRemoteForm = false })
		ui.PrimaryButton(c.Key("create-remote"), "开始授权").Height(30).Disabled(s.busy).OnClick(func() {
			name, kind := strings.TrimSpace(s.newRemoteName), s.newRemoteType
			s.runAfter("添加云端账户", func() error {
				_, err := s.app.AddJob(core.JobCreateRequest{Name: "添加账户 " + name, Operation: "oauth-create", Source: name, Destination: kind})
				return err
			}, func() { s.showRemoteForm = false })
		})
	})
}
