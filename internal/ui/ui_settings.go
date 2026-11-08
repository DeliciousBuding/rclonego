package desktop

import (
	"context"
	"fmt"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) settingsPage(c *ui.Context) {
	t := c.Theme()
	loginAvailable := s.canConfigureLogin()
	contentScroll(c).Grow(1).Gap(18).Children(func() {
		pageHeading(c, "设置", "外观、语言与应用偏好", func() {
			actionButton(c.Key("about"), iconInfo, "关于 RcloneGo", false).OnClick(func() { s.showAbout = true })
		})
		ui.Column(c).Gap(18).Children(func() {
			sectionHeading(c, locales.In(c.Locale()).GeneralSettings())
			settingsPreferenceRow(c.Key("theme-preferences"), text(c, "外观"), "", func() {
				if translatedSelect(c.Key("appearance"), &s.appearanceChoice, []string{"跟随系统", "浅色", "深色"}).
					FillWidth().Label(text(c, "外观")).Disabled(s.busy).Changed() {
					mode := map[string]string{"浅色": "light", "深色": "dark", "跟随系统": "system"}[s.appearanceChoice]
					s.setAppearance(mode)
				}
			})
			s.languagePreferences(c)
			settingsPreferenceRow(c.Key("background-preferences"), text(c, "后台运行"),
				text(c, map[bool]string{true: "关闭窗口后继续在托盘运行，选择“退出”才停止服务。", false: "当前桌面无法创建托盘，关闭窗口将退出服务。"}[s.trayAvailable]), func() {
					if ui.Checkbox(c.Key("open-at-login"), &s.openAtLogin, text(c, "登录时启动")).
						Disabled(!loginAvailable).Changed() {
						if err := mygo.App.SetOpenAtLogin(s.openAtLogin); err != nil {
							s.openAtLogin = mygo.App.OpenAtLogin()
							s.setError(err)
						}
					}
					if !loginAvailable {
						ui.Text(c, locales.In(c.Locale()).LoginUnavailableForProfile()).
							FontSize(11).TextColor(t.TextMuted).MaxLines(2)
					}
				})
		})
		ui.Divider(c)
		ui.Column(c).Gap(8).Children(func() { s.updatesPreferences(c) })
		ui.Divider(c)
		sectionHeading(c, locales.In(c.Locale()).AdvancedSettings())
		s.handoffPreferences(c)
		ui.Collapsible(c.Key("engine-settings"), text(c, "rclone 引擎"), &s.showEngineSettings, func() {
			ui.Text(c, core.ValueOr(s.version, text(c, "尚未安装引擎 · 窗口和设置仍可使用"))).FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				actionButton(c.Key("install-engine"), iconDownload, "下载推荐版本", true).Disabled(s.busy).OnClick(func() {
					s.run(text(c, "安装引擎"), func() error {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
						defer cancel()
						_, err := s.app.InstallEngine(ctx, "")
						return err
					})
				})
				ui.Text(c, text(c, "校验官方发行包，不修改系统 PATH")).Grow(1).MinWidth(0).FontSize(11).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					s.pathInput(c.Key("engine-path"), &s.enginePath, "引擎路径", "已有 rclone 可执行文件路径", false, false, nil)
				})
				ui.Button(c.Key("select-engine"), text(c, "使用此引擎")).Shrink(0).Disabled(s.busy || s.enginePath == "").OnClick(func() {
					path := s.enginePath
					s.run(text(c, "选择引擎"), func() error { _, err := s.app.SelectEngine(context.Background(), path); return err })
				})
			})
			ui.Row(c).Gap(8).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					s.pathInput(c.Key("engine-archive"), &s.engineArchive, "离线发行包路径", "官方推荐版本 ZIP（离线安装）", false, false, []mygo.FileFilter{{Name: "rclone ZIP", Extensions: []string{"zip"}}})
				})
				ui.Button(c.Key("import-engine"), text(c, "导入发行包")).Shrink(0).Disabled(s.busy || s.engineArchive == "").OnClick(func() {
					path := s.engineArchive
					s.run(text(c, "导入引擎"), func() error { _, err := s.app.InstallEngine(context.Background(), path); return err })
				})
			})
			ui.Text(c, text(c, "活动任务保留启动时的引擎；新选择用于后续任务。")).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Collapsible(c.Key("default-settings"), text(c, "新任务默认参数"), &s.showDefaults, func() {
			ui.Text(c, fmt.Sprintf(text(c, "%d 路传输 · %d 路校验 · %s 缓冲"), s.defaults.Transfers, s.defaults.Checkers, s.defaults.BufferSize)).FontSize(12)
			ui.Form(c, func() {
				ui.Field(c, text(c, "传输并发"), func() { ui.TextInput(c.Key("default-transfers"), &s.defaultTransfers) })
				ui.Field(c, text(c, "校验并发"), func() { ui.TextInput(c.Key("default-checkers"), &s.defaultCheckers) })
				ui.Field(c, text(c, "Google Drive 上传分块"), func() { ui.TextInput(c.Key("default-chunk"), &s.defaultChunk) })
				ui.Field(c, text(c, "每路缓冲"), func() { ui.TextInput(c.Key("default-buffer"), &s.defaultBuffer) })
				ui.Field(c, text(c, "CPU 并行度"), func() { ui.TextInput(c.Key("default-cpu"), &s.defaultCPU) })
				ui.Field(c, text(c, "默认限速"), func() {
					ui.TextInput(c.Key("default-bandwidth"), &s.defaultBandwidth).Placeholder(text(c, "留空表示不限速"))
				})
			})
			edited, err := s.editedDefaults()
			if err == nil {
				base := edited.BufferBudgetFor(core.CapabilitiesForBackend("local"))
				drive := edited.BufferBudgetFor(core.CapabilitiesForBackend("drive"))
				ui.Text(c, fmt.Sprintf(text(c, "传输缓冲估算：%s；Google Drive：%s"), formatBytes(base), formatBytes(drive))).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, text(c, "Google Drive 上传分块仅在目标支持时使用；估算不包含校验、引擎和界面开销。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
			}
			ui.Text(c, text(c, "CPU 并行度限制同时执行 Go 代码的核心数；0 为自动，系统线程数仍由 rclone 管理。")).FontSize(11).TextColor(t.TextMuted)
			ui.Button(c.Key("save-defaults"), text(c, "保存默认参数")).Disabled(s.busy).OnClick(func() {
				settings, err := s.editedDefaults()
				if err != nil {
					s.setError(err)
					return
				}
				s.run(text(c, "保存参数"), func() error { return s.app.SaveSettings(core.SettingsRequest{Defaults: &settings}) })
			})
		})
	})
}
