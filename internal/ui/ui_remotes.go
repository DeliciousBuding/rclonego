package desktop

import (
	"context"
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
				Disabled(s.busy).OnClick(func() {
				if s.account.ID != "" {
					s.showRemoteForm = true
					return
				}
				var providers []core.Provider
				s.runAfter("获取服务列表", func() error {
					var err error
					providers, err = s.app.Providers(context.Background())
					return err
				}, func() {
					s.providers = providers
					s.account = core.AccountView{}
					s.showRemoteForm = true
				})
			})
		})
		contentScroll(c).Grow(1).Gap(0).Children(func() {
			if len(s.remotes) == 0 {
				emptyState(c, iconRemotes, "暂无云端账户", "添加 rclone 远端后即可开始使用")
			}
			for _, remote := range s.remotes {
				ui.Row(c).Padding(18, 0).Gap(14).BorderWidth(0, 0, 0.5, 0).
					BorderColor(t.Border).Children(func() {
					ui.Box(c).Size(36, 36).Radius(10).Center().Background(t.Accent.Alpha(0.07)).
						Children(func() { remoteLogo(c, remote.Type, 24) })
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, remote.Name).FontSize(13).FontWeight(600).SingleLine().Tooltip(remote.Name)
						label := remote.Type
						if label == "drive" {
							label = "Google Drive"
						}
						ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
					})
					ui.Button(c.Key("oauth-"+remote.Name), "重新授权…").Height(28).Padding(4, 10).
						Disabled(s.busy).OnClick(func() {
						name := remote.Name
						var account core.AccountView
						s.runAfter("重新授权", func() error {
							var err error
							account, err = s.app.BeginAccount(context.Background(), core.AccountRequest{Name: name, Reconnect: true})
							return err
						}, func() { s.account = account; s.showRemoteForm = true })
					})
				})
			}
		})
	})
}

func (s *nativeUI) remoteEditor(c *ui.Context) {
	if s.account.ID != "" {
		s.accountEditor(c)
		return
	}
	if s.newRemoteType == "drive" {
		ui.Row(c).Gap(10).Children(func() {
			brandIcon(c, googleGLogo, 20).Label("Google")
			ui.Text(c, "连接 Google Drive").FontSize(13).FontWeight(500)
		})
	}
	ui.Form(c, func() {
		ui.Field(c, "远端名称", func() { ui.TextInput(c.Key("new-remote-name"), &s.newRemoteName).Placeholder("drive-work").AutoFocus() })
		ui.Field(c, "服务类型", func() {
			choices := []string{}
			kinds := map[string]string{}
			for _, provider := range s.providers {
				label := provider.Description
				choices = append(choices, label)
				kinds[label] = provider.Name
				if provider.Name == s.newRemoteType {
					s.remoteTypeChoice = label
				}
			}
			if ui.Select(c.Key("new-remote-type"), &s.remoteTypeChoice, choices).Changed() {
				s.newRemoteType = kinds[s.remoteTypeChoice]
			}
		})
		if s.newRemoteType == "drive" {
			ui.Field(c, "OAuth Client ID", func() {
				ui.TextInput(c.Key("account-client-id"), &s.accountClientID).Placeholder("自己的 Google OAuth 客户端 ID")
			})
			ui.Field(c, "Client secret", func() { ui.TextInput(c.Key("account-client-secret"), &s.accountClientSecret).Password() })
		}
	})
	ui.Text(c, "按服务要求逐步配置并验证读取权限；取消时保留原配置。").
		FontSize(12).TextColor(c.Theme().TextMuted)
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		ui.Button(c.Key("cancel-remote"), "取消").Height(30).OnClick(func() { s.showRemoteForm = false })
		ui.PrimaryButton(c.Key("create-remote"), "开始授权").Height(30).Disabled(s.busy).OnClick(func() {
			name, kind := strings.TrimSpace(s.newRemoteName), s.newRemoteType
			parameters := map[string]string{}
			if kind == "drive" {
				parameters["client_id"], parameters["client_secret"] = s.accountClientID, s.accountClientSecret
			}
			s.accountClientID, s.accountClientSecret = "", ""
			var account core.AccountView
			s.runAfter("开始账户配置", func() error {
				var err error
				account, err = s.app.BeginAccount(context.Background(), core.AccountRequest{Name: name, Type: kind, Parameters: parameters})
				return err
			}, func() { s.account = account })
		})
	})
}

func (s *nativeUI) accountEditor(c *ui.Context) {
	account := s.account
	labels := map[string]string{"configuring": "正在配置 · 如浏览器已打开，请完成授权", "needs_input": "需要补充配置",
		"verifying": "正在验证读取权限", "completed": "账户已连接", "failed": "账户配置未完成", "cancelled": "已取消"}
	ui.Text(c, account.Name).FontSize(17).FontWeight(600)
	ui.Text(c, labels[account.Status]).FontSize(12).TextColor(c.Theme().TextMuted)
	if account.Message != "" {
		ui.Text(c, account.Message).FontSize(12).TextColor(c.Theme().TextMuted)
	}
	if q := account.Question; q != nil {
		ui.Text(c, q.Name).FontSize(13).FontWeight(600)
		contentScroll(c).MaxHeight(170).Children(func() { ui.Text(c, q.Help).FontSize(12) })
		for _, example := range q.Examples {
			if len(q.Examples) > 12 {
				break
			}
			ui.Text(c, example.Value+" · "+example.Help).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		input := ui.TextInput(c.Key("account-answer-"+q.Name), &s.accountAnswer).Placeholder(q.DefaultStr).AutoFocus()
		if q.IsPassword || q.Sensitive {
			input.Password()
		}
		if q.DefaultStr != "" {
			ui.Text(c, "留空使用默认值："+q.DefaultStr).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		if q.IsPassword || q.Sensitive {
			ui.Text(c, "敏感值不会回显；留空保留服务提供的默认值。").FontSize(11).TextColor(c.Theme().TextMuted)
		}
		ui.PrimaryButton(c.Key("answer-account"), "继续").Disabled(s.busy).OnClick(func() {
			answer, id := s.accountAnswer, account.ID
			s.accountAnswer = ""
			s.run("提交配置", func() error { return s.app.AnswerAccount(id, answer) })
		})
	}
	ui.Row(c).Justify(ui.End).Children(func() {
		terminal := account.Status == "completed" || account.Status == "failed" || account.Status == "cancelled"
		label := "取消配置"
		if terminal {
			label = "关闭"
		}
		ui.Button(c.Key("cancel-account"), label).OnClick(func() {
			id := account.ID
			s.accountAnswer, s.accountClientID, s.accountClientSecret = "", "", ""
			if terminal {
				s.showRemoteForm = false
				s.account = core.AccountView{}
				return
			}
			s.runAfter("取消配置", func() error { return s.app.CancelAccount(id) }, func() { s.showRemoteForm = false; s.account = core.AccountView{} })
		})
	})
}
