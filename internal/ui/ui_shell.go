package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"time"
)

func modernTheme(dark bool, prefs ui.Preferences) *ui.Theme {
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
	}
	t.Background = ui.Hex("#f6f9fb")
	t.Surface = ui.RGBA(255, 255, 255, 0.88)
	t.SurfaceHover = ui.RGBA(0, 0, 0, 0.04)
	t.SurfacePressed = ui.RGBA(73, 148, 177, 0.12)
	t.Border = ui.RGBA(60, 60, 67, 0.11)
	t.Text = ui.Hex("#26373d")
	t.TextMuted = ui.Hex("#6e6e73")
	t.Accent = ui.Hex("#2b7b8e")
	t.AccentHover = ui.Hex("#256c7e")
	t.AccentPressed = ui.Hex("#1d6173")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#c9342c")
	t.Warning = ui.Hex("#a66b08")
	t.Success = ui.Hex("#248a3d")
	t.Selection = t.Accent.Alpha(0.22)
	t.Focus = t.Accent.Alpha(0.45)
	t.Radius = 7
	t.Spacing = 4
	t.FontSize = 13
	t.Font = ""
	if dark {
		t.Background = ui.Hex("#2d353c")
		t.Surface = ui.Hex("#3a434b")
		t.SurfaceHover = ui.RGBA(255, 255, 255, 0.06)
		t.Text = ui.Hex("#edf3f6")
		t.TextMuted = ui.Hex("#b6c3cb")
		t.Border = ui.RGBA(220, 235, 244, 0.13)
		t.Accent = ui.Hex("#85bed0")
		t.AccentHover = ui.Hex("#9bcbd9")
		t.AccentPressed = ui.Hex("#70adbf")
		t.AccentText = ui.Hex("#18303b")
		t.SurfacePressed = t.Accent.Alpha(0.14)
		t.Selection = t.Accent.Alpha(0.24)
		t.Focus = t.Accent.Alpha(0.55)
		t.Danger = ui.Hex("#ff6961")
		t.Warning = ui.Hex("#ffd60a")
		t.Success = ui.Hex("#74cca4")
	}
	if prefs.HighContrast {
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Border = t.Border.Mix(t.Text, 0.4)
	}
	if prefs.TextScale > 0 {
		t.FontSize *= prefs.TextScale
	}
	return t
}

func (s *nativeUI) view(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		s.page = "settings"
	}
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		s.commandOpen, s.commandQuery, s.commandIndex = true, "", 0
	}
	dark := s.appearance == "dark" || (s.appearance == "system" && c.Theme().Dark)
	t := modernTheme(dark, c.Preferences())
	c.Root().Background(ui.Transparent)
	ui.Themed(c, t).Fill().Background(ui.Transparent).Children(func() {
		vibrant := c.Vibrancy() && !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast
		chrome, tint := ui.Hex("#eaf3f8"), ui.Hex("#f2f7fa").Alpha(0.14)
		if t.Dark {
			chrome, tint = ui.Hex("#354149"), ui.Hex("#c4dce8").Alpha(0.16)
		}
		if vibrant && c.WindowActive() {
			// Let the native material do the blur. Keep one light tint across
			// navigation, title bar and the exposed canvas corners.
			chrome = tint
		}
		// Windows replaces inactive Acrylic with gray. An opaque themed
		// fallback keeps inactive and reduced-transparency chrome readable.
		shell := ui.Column(c).Fill().Background(chrome)
		shell.Children(func() {
			if c.TitleBar().Height > 0 {
				s.titleBar(c)
			}
			ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				s.sidebar(c)
				s.main(c)
			})
		})
		s.editOverlays(c)
		s.workflowOverlays(c)
		s.automationOverlays(c)
		s.cacheOverlays(c)
		s.commandPalette(c)
		s.browserOverlay(c)
	})
}

func (s *nativeUI) titleBar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(max(52, bar.Height)).Shrink(0).
		Padding(0, bar.Right+16, 0, max(20, bar.Left+12)).Gap(8).
		DragWindow().Children(func() {
		ui.Icon(c, appMark).FontSize(17).TextColor(c.Theme().Accent)
		ui.Text(c, "RcloneGo").FontSize(14).FontWeight(600)
		ui.Spacer(c)
		s.navigationCommands(c)
	})
}

func (s *nativeUI) navigationCommands(c *ui.Context) {
	command := ui.ButtonBase(c.Key("commands")).Height(30).Padding(0, 8).Gap(7).
		Radius(7).Role(ui.RoleButton).Label(text(c, "搜索命令")).Tooltip(text(c, "搜索命令")).
		KeyShortcuts(ui.Cmd, ui.KeyK).
		OnClick(func() { s.commandOpen, s.commandQuery, s.commandIndex = true, "", 0 })
	if command.Hovered() {
		command.Background(c.Theme().SurfaceHover)
	}
	if command.Pressed() {
		command.Background(c.Theme().SurfacePressed)
	}
	command.Children(func() {
		ui.Icon(c, iconSearch).FontSize(15).TextColor(c.Theme().TextMuted)
		ui.Text(c, text(c, "搜索命令")).FontSize(12).TextColor(c.Theme().TextMuted).SingleLine()
	})
	iconButton(c.Key("refresh"), iconRefresh, "刷新状态").OnClick(func() {
		s.app.RefreshRuntime()
		go s.refresh(context.Background(), true)
	}).Disabled(s.busy)
}

func (s *nativeUI) sidebar(c *ui.Context) {
	t := c.Theme()
	queued := 0
	for _, job := range s.jobs {
		if job.Status == "pending" || job.Status == "running" || job.Status == "handoff" {
			queued++
		}
	}
	side := ui.Column(c).Width(208).Shrink(0).FillHeight()
	side.Background(ui.Transparent)
	side.Children(func() {
		if c.TitleBar().Height == 0 {
			ui.Row(c).Height(48).Shrink(0).Padding(0, 12).Gap(4).Children(func() {
				s.navigationCommands(c)
			})
		}
		ui.Sidebar(c.Key("navigation"), &s.page, func() {
			for _, item := range []struct {
				id, label string
				icon      *ui.SVG
			}{
				{"overview", "总览", iconOverview},
				{"queue", "任务队列", iconQueue},
				{"workflows", "工作流", iconWorkflow},
				{"automations", "自动任务", iconSchedule},
				{"mounts", "盘符映射", iconMounts},
				{"remotes", "云端账户", iconRemotes},
				{"resources", "运行与资源", iconResources},
			} {
				entry := ui.SidebarItem(c, item.id, item.icon, text(c, item.label)).
					Height(36).Padding(0, 10).Radius(8).FontWeight(500)
				if item.id == "queue" && queued > 0 {
					entry.Children(func() {
						ui.Text(c, fmt.Sprint(queued)).FontSize(11).TextColor(t.TextMuted)
					})
				}
			}
		}).Grow(1).Padding(8, 12).Gap(4).Background(ui.Transparent).Label(text(c, "主导航"))
		ui.Sidebar(c.Key("preferences-navigation"), &s.page, func() {
			ui.SidebarItem(c, "settings", iconSettings, text(c, "设置")).
				Height(36).Padding(0, 10).Radius(8).FontWeight(500)
		}).Padding(8, 12).Background(ui.Transparent)
		ui.Column(c).Padding(8, 20, 20, 20).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Box(c).Size(5, 5).Radius(3).Background(t.Success)
				ui.Text(c, text(c, "本地服务已连接")).FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *nativeUI) setAppearance(mode string) {
	if !core.ValidAppearance(mode) {
		return
	}
	s.appearance = mode
	s.appearanceChoice = appearanceLabel(mode)
	mygo.Theme.SetSource(mygo.ThemeSource(mode))
	s.run(locales.Text("保存外观"), func() error {
		return s.app.SaveSettings(core.SettingsRequest{Appearance: &mode})
	})
}

func (s *nativeUI) main(c *ui.Context) {
	t := c.Theme()
	pane := ui.Column(c).Grow(1).MinWidth(0).FillHeight()
	pane.Children(func() {
		top := float32(0)
		if c.TitleBar().Height == 0 {
			top = 12
		}
		body := ui.Column(c).Grow(1).MinHeight(0).Padding(24, 28, 24, 28).Gap(16).
			Radius(12).Margin(top, 12, 12, 0).Background(t.Background).
			Border(0.5, t.Border.Alpha(0.45))
		body.Children(func() {
			if s.notice != "" && s.isError {
				ui.Row(c).Shrink(0).Padding(10, 12).Gap(10).Radius(8).
					Background(t.Danger.Alpha(0.06)).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, iconWarning).FontSize(16).TextColor(t.Danger)
					ui.Text(c, s.notice).Grow(1).MinWidth(0).FontSize(12).TextColor(t.Danger).
						MaxLines(3).Tooltip(s.notice).Selectable()
					iconButton(c.Key("dismiss-error"), iconClose, "关闭错误提示").OnClick(func() {
						s.notice, s.isError = "", false
					})
				})
			}
			switch s.page {
			case "remotes":
				s.remotesPage(c)
			case "mounts":
				s.mountsPage(c)
			case "queue":
				s.queuePage(c)
			case "workflows":
				s.workflowsPage(c)
			case "automations":
				s.automationsPage(c)
			case "resources":
				s.resourcesPage(c)
			case "settings":
				s.settingsPage(c)
			default:
				s.overviewPage(c)
			}
		})
		s.statusBar(c)
		if s.notice != "" && !s.isError && time.Now().Before(s.noticeUntil) {
			ui.Overlay(c, func() {
				toast := ui.Row(c).Absolute().Bottom(24).Right(28).Padding(9, 14).Gap(7).
					Radius(10).Background(t.Surface).
					Shadow(0, 3, 12, 0, ui.RGBA(0, 0, 0, 0.08)).Role(ui.RoleStatus)
				if !c.Preferences().ReduceTransparency && !c.Preferences().HighContrast {
					toast.Material(glass.Glass{})
				}
				toast.Children(func() {
					ui.Icon(c, iconCheck).FontSize(14).TextColor(t.Success)
					ui.Text(c, s.notice).FontSize(12)
				})
			})
		}
	})
}

func appearanceLabel(mode string) string {
	return map[string]string{"light": "浅色", "dark": "深色", "system": "跟随系统"}[mode]
}

func (s *nativeUI) pageTitle() (string, string) {
	switch s.page {
	case "automations":
		return "自动任务", "目录变化与周期计划，共用任务队列"
	case "workflows":
		return "工作流", "保存定义，重复执行，查看结果"
	case "remotes":
		return "云端账户", "管理 rclone 账户与授权"
	case "mounts":
		return "盘符映射", "管理本机可用的云端目录"
	case "queue":
		return "任务队列", "查看传输、校验和授权任务"
	case "resources":
		return "运行与资源", "rclone 参数与资源占用"
	case "settings":
		return "设置", "外观、语言与应用偏好"
	default:
		return "总览", "查看当前运行状态"
	}
}
