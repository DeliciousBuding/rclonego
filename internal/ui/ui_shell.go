package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"time"
)

func modernTheme(dark bool, prefs ui.Preferences) *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f5f8f9")
	t.Surface = ui.RGBA(255, 255, 255, 0.88)
	t.SurfaceHover = ui.RGBA(0, 0, 0, 0.04)
	t.SurfacePressed = ui.RGBA(0, 122, 255, 0.10)
	t.Border = ui.RGBA(60, 60, 67, 0.11)
	t.Text = ui.Hex("#26373d")
	t.TextMuted = ui.Hex("#6e6e73")
	t.Accent = ui.Hex("#318b9d")
	t.AccentHover = ui.Hex("#267789")
	t.AccentPressed = ui.Hex("#216979")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#c9342c")
	t.Warning = ui.Hex("#a66b08")
	t.Success = ui.Hex("#248a3d")
	t.Focus = ui.RGBA(0, 122, 255, 0.35)
	t.Radius = 7
	t.Spacing = 4
	t.FontSize = 13
	t.Font = ""
	if dark {
		t.Dark = true
		t.Background = ui.Hex("#111a1e")
		t.Surface = ui.Hex("#1b282d")
		t.SurfaceHover = ui.RGBA(255, 255, 255, 0.07)
		t.SurfacePressed = ui.RGBA(10, 132, 255, 0.22)
		t.Text = ui.Hex("#f5f5f7")
		t.TextMuted = ui.Hex("#aeaeb2")
		t.Border = ui.RGBA(255, 255, 255, 0.12)
		t.Accent = ui.Hex("#55aebe")
		t.AccentHover = ui.Hex("#73c0ce")
		t.AccentPressed = ui.Hex("#4197a8")
		t.Danger = ui.Hex("#ff6961")
		t.Warning = ui.Hex("#ffd60a")
		t.Success = ui.Hex("#6dde86")
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
	dark := s.appearance == "dark" || (s.appearance == "system" && c.Theme().Dark)
	t := modernTheme(dark, c.Preferences())
	c.Root().Background(ui.Transparent)
	ui.Themed(c, t).Fill().Background(ui.Transparent).Children(func() {
		vibrant := c.Vibrancy() && !c.Preferences().ReduceTransparency
		shell := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(t.Background)
		if vibrant {
			// One material tint behind navigation, toolbar, gutters and the
			// canvas corners. Separate tints leave dark rectangular notches.
			shell.Background(t.Background.Alpha(0.62))
		}
		shell.Children(func() {
			s.sidebar(c, vibrant)
			s.main(c, vibrant)
		})
		s.editOverlays(c)
	})
}

func (s *nativeUI) sidebar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	side := ui.Column(c).Width(208).Shrink(0).FillHeight()
	side.Background(ui.Transparent)
	if !vibrant {
		color := ui.Hex("#f1f3f7")
		if t.Dark {
			color = ui.Hex("#252528")
		}
		side.Background(color)
	}
	side.Children(func() {
		ui.Row(c).Height(56).Shrink(0).Padding(0, 20, 0, max(20, c.TitleBar().Left)).
			DragWindow().Children(func() {
			if c.TitleBar().Left == 0 {
				ui.Icon(c, appMark).FontSize(19).TextColor(t.Accent)
				ui.Box(c).Width(8)
				ui.Text(c, "RcloneGo").FontSize(15).FontWeight(600)
			}
		})
		ui.Sidebar(c.Key("navigation"), &s.page, func() {
			for _, item := range []struct {
				id, label string
				icon      *ui.SVG
			}{
				{"overview", "总览", iconOverview},
				{"queue", "任务队列", iconQueue},
				{"workflows", "工作流", iconWorkflow},
				{"mounts", "盘符映射", iconMounts},
				{"remotes", "云端账户", iconRemotes},
				{"resources", "运行与资源", iconResources},
			} {
				entry := ui.SidebarItem(c, item.id, item.icon, item.label).
					Height(36).Padding(0, 10).Radius(8).FontWeight(500)
				if item.id == "queue" && len(s.jobs) > 0 {
					entry.Children(func() {
						ui.Text(c, fmt.Sprint(len(s.jobs))).FontSize(11).TextColor(t.TextMuted)
					})
				}
			}
		}).Grow(1).Padding(8, 12).Gap(4).Background(ui.Transparent).Label("主导航")
		ui.Column(c).Padding(16, 20, 20, 20).Gap(6).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Box(c).Size(5, 5).Radius(3).Background(t.Success)
				ui.Text(c, "本地服务已连接").FontSize(11).TextColor(t.TextMuted)
			})
			ui.Row(c).Gap(8).Margin(10, 0, 0, 0).Children(func() {
				ui.Icon(c, iconAppearance).FontSize(16).TextColor(t.TextMuted)
				if ui.Select(c.Key("appearance"), &s.appearanceChoice, []string{"浅色", "深色", "跟随系统"}).Grow(1).
					Height(28).Label("外观").Disabled(s.busy).Changed() {
					mode := map[string]string{"浅色": "light", "深色": "dark", "跟随系统": "system"}[s.appearanceChoice]
					s.setAppearance(mode)
				}
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
	s.run("保存外观", func() error {
		return s.app.SaveSettings(core.SettingsRequest{Appearance: &mode})
	})
}

func (s *nativeUI) main(c *ui.Context, vibrant bool) {
	t := c.Theme()
	pane := ui.Column(c).Grow(1).MinWidth(0).FillHeight()
	pane.Children(func() {
		ui.Row(c).Height(56).Shrink(0).Padding(0, c.TitleBar().Right+16, 0, 28).Gap(12).AlignItems(ui.Center).
			Background(ui.Transparent).
			DragWindow().Children(func() {
			title, _ := s.pageTitle()
			ui.Text(c, title).FontSize(15).FontWeight(600)
			ui.Spacer(c)
			iconButton(c.Key("refresh"), iconRefresh, "刷新状态").OnClick(func() {
				s.app.RefreshRuntime()
				go s.refresh(context.Background(), true)
			}).Disabled(s.busy)
		})
		body := ui.Column(c).Grow(1).MinHeight(0).Padding(24, 28, 24, 28).Gap(16).
			Radius(14).Margin(0, 12, 12, 0).Background(t.Background)
		body.Children(func() {
			if s.notice != "" && s.isError {
				color := t.Success
				if s.isError {
					color = t.Danger
				}
				ui.Text(c, s.notice).FontSize(12).TextColor(color).SingleLine()
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
			case "resources":
				s.resourcesPage(c)
			default:
				s.overviewPage(c)
			}
		})
		s.statusBar(c)
		if s.notice != "" && !s.isError && time.Now().Before(s.noticeUntil) {
			ui.Overlay(c, func() {
				ui.Row(c).Absolute().Bottom(24).Right(28).Padding(9, 14).Gap(7).
					Radius(10).Background(t.Surface).Border(0.5, t.Border).
					Shadow(0, 3, 12, 0, ui.RGBA(0, 0, 0, 0.08)).Role(ui.RoleStatus).Children(func() {
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
	default:
		return "总览", "查看当前运行状态"
	}
}
