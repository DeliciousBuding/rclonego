package desktop

import (
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

type desktopCommand struct {
	label string
	icon  *ui.SVG
	run   func()
}

func commandMatches(label, query, language string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	for _, locale := range []string{language, "en", "zh-Hans"} {
		if strings.Contains(strings.ToLower(locales.TextIn(locale, label)), query) {
			return true
		}
	}
	return false
}

func (s *nativeUI) commands() []desktopCommand {
	result := []desktopCommand{
		{"新建任务", iconPlus, func() { s.beginJobForm(nil) }},
		{"新建工作流", iconWorkflow, func() { s.beginWorkflowEditor(core.WorkflowDefinition{}, false) }},
	}
	for _, page := range []struct {
		id, label string
		icon      *ui.SVG
	}{
		{"overview", "总览", iconOverview}, {"queue", "任务队列", iconQueue},
		{"workflows", "工作流", iconWorkflow}, {"mounts", "盘符映射", iconMounts},
		{"automations", "自动任务", iconSchedule},
		{"remotes", "云端账户", iconRemotes}, {"resources", "运行与资源", iconResources},
		{"files", "文件", iconFolder},
		{"settings", "设置", iconSettings},
	} {
		id := page.id
		result = append(result, desktopCommand{page.label, page.icon, func() { s.page = id }})
	}
	return result
}

func (s *nativeUI) commandPalette(c *ui.Context) {
	ui.DialogBase(c.Key("command-palette"), &s.commandOpen, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(480).MaxWidthPercent(90).MaxHeightPercent(80).Padding(16).Gap(8).
			Radius(16).Background(t.Background).Shadow(0, 12, 36, 0, t.Text.Alpha(0.12))
		search := ui.SearchField(c.Key("command-search"), &s.commandQuery).AutoFocus().Placeholder(text(c, "搜索页面与操作…")).Label(text(c, "搜索命令")).
			OnChange(func() { s.commandIndex = 0 })
		var matches []desktopCommand
		for _, command := range s.commands() {
			if commandMatches(command.label, s.commandQuery, c.Locale()) {
				matches = append(matches, command)
			}
		}
		if len(matches) == 0 {
			ui.Text(c, text(c, "没有匹配的命令")).FontSize(12).TextColor(t.TextMuted)
			return
		}
		if c.Shortcut(0, ui.KeyDown) {
			s.commandIndex = (s.commandIndex + 1) % len(matches)
		}
		if c.Shortcut(0, ui.KeyUp) {
			s.commandIndex = (s.commandIndex + len(matches) - 1) % len(matches)
		}
		s.commandIndex = min(max(s.commandIndex, 0), len(matches)-1)
		activate := func(command desktopCommand) { s.commandOpen = false; command.run() }
		if search.Submitted() || c.Shortcut(0, ui.KeyEnter) {
			activate(matches[s.commandIndex])
		}
		contentScroll(c).MaxHeight(360).Gap(4).Children(func() {
			for i, command := range matches {
				button := ui.Button(c.Key(command.label), "").Height(36).Padding(6, 10).Gap(10).Label(text(c, command.label)).
					OnClick(func() { activate(command) })
				if i == s.commandIndex {
					button.Background(t.Accent.Alpha(0.1))
				}
				button.Children(func() {
					ui.Icon(c, command.icon).FontSize(17).TextColor(t.Accent)
					ui.Text(c, text(c, command.label)).Grow(1).MinWidth(0).SingleLine()
					if i == s.commandIndex {
						ui.Text(c, "↵").TextColor(t.TextMuted)
					}
				})
			}
		})
	})
}
