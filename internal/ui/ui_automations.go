package desktop

import (
	"fmt"
	"strconv"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

type automationUIState struct {
	open     bool
	value    core.Automation
	kind     string
	workflow string
	minutes  string
	advanced bool
}

func (s *nativeUI) beginAutomation(value core.Automation) {
	if value.Kind == "" {
		value.Kind, value.Settings = "watch", s.defaults
		value.DebounceSeconds, value.StableSeconds, value.RescanSeconds = 3, 2, 300
		value.IntervalSeconds = 3600
	}
	s.automationUI = automationUIState{open: true, value: value, kind: map[string]string{"watch": "目录变化", "interval": "定期执行"}[value.Kind],
		minutes: strconv.FormatInt(value.IntervalSeconds/60, 10)}
	for _, workflow := range s.workflows {
		if workflow.ID == value.WorkflowID {
			s.automationUI.workflow = workflow.Name + " · " + workflow.ID[:min(6, len(workflow.ID))]
		}
	}
}

func automationLabel(value core.Automation) string {
	if !value.Enabled {
		return "未启用"
	}
	return core.ValueOr(map[string]string{"watching": "正在监听", "waiting": "等待触发", "queued": "已加入队列", "running": "正在执行", "paused": "任务已暂停",
		"interrupted": "等待恢复", "attention": "需要处理"}[value.Status], value.Status)
}

func (s *nativeUI) automationsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		pageHeading(c, "自动任务", "变化合并后执行，任务之间不会重叠", func() {
			actionButton(c.Key("new-automation"), iconPlus, "添加自动任务…", true).Disabled(s.busy).OnClick(func() { s.beginAutomation(core.Automation{}) })
		})
		contentScroll(c).Grow(1).Gap(12).Children(func() {
			if len(s.automations) == 0 {
				emptyState(c, iconSchedule, "按需启用自动运行", "监听本机目录变化，或定期执行已保存的工作流")
			}
			for _, value := range s.automations {
				ui.Column(c.Key(value.ID)).Padding(14).Gap(9).Radius(10).Background(t.Surface).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Icon(c, iconSchedule).FontSize(20).TextColor(t.Accent)
						ui.Text(c, value.Name).Grow(1).MinWidth(0).FontWeight(600).SingleLine()
						statusDot(c, automationLabel(value), t.TextMuted)
						iconButton(c.Key("edit"), iconEngine, "编辑自动任务").Disabled(s.busy).OnClick(func() { s.beginAutomation(value) })
					})
					if value.Kind == "watch" {
						ui.Text(c, value.Source+" → "+value.Destination).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(value.Source + " → " + value.Destination)
					} else {
						ui.Text(c, fmt.Sprintf(text(c, "每 %d 分钟执行一次"), value.IntervalSeconds/60)).FontSize(11).TextColor(t.TextMuted)
					}
					if value.NextRunAt != nil {
						ui.Text(c, text(c, "下次执行：")+dateTime(c, *value.NextRunAt)).FontSize(11).TextColor(t.TextMuted)
					}
					if value.Error != "" || value.WatchWarning != "" {
						ui.Text(c, core.ValueOr(value.Error, value.WatchWarning)).FontSize(11).TextColor(t.Warning).MaxLines(3)
					}
					ui.Row(c).Gap(8).Wrap().Children(func() {
						label := "启用"
						if value.Enabled {
							label = "停用"
						}
						ui.Button(c.Key("enable"), text(c, label)).Disabled(s.busy).OnClick(func() {
							next := value
							next.Enabled = !value.Enabled
							s.run(text(c, "更新自动任务"), func() error { _, err := s.app.SaveAutomation(next); return err })
						})
						ui.Button(c.Key("run"), text(c, "执行一次")).Disabled(s.busy).OnClick(func() { s.run(text(c, "触发任务"), func() error { return s.app.TriggerAutomation(value.ID) }) })
						ui.Spacer(c)
						iconButton(c.Key("delete"), iconTrash, "删除自动任务，保留队列记录").Disabled(s.busy).OnClick(func() {
							s.run(text(c, "删除自动任务"), func() error { return s.app.DeleteAutomation(value.ID) })
						})
					})
				})
			}
		})
	})
}

func (s *nativeUI) automationOverlays(c *ui.Context) {
	w := &s.automationUI
	ui.DialogBase(c.Key("automation-editor"), &w.open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(520).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(14).Radius(16).Background(t.Background)
		ui.Row(c).Children(func() {
			sectionHeading(c, "自动任务")
			ui.Spacer(c)
			iconButton(c.Key("close"), iconClose, "关闭自动任务配置").OnClick(func() { w.open = false })
		})
		contentScroll(c).Gap(12).Children(func() {
			ui.Form(c, func() {
				ui.Field(c, text(c, "名称"), func() { ui.TextInput(c.Key("name"), &w.value.Name).AutoFocus() })
				ui.Field(c, text(c, "触发方式"), func() {
					if translatedSelect(c.Key("kind"), &w.kind, []string{"目录变化", "定期执行"}).Changed() {
						w.value.Kind = map[string]string{"目录变化": "watch", "定期执行": "interval"}[w.kind]
					}
				})
				if w.value.Kind == "watch" {
					ui.Field(c, text(c, "本机源目录"), func() {
						s.pathInput(c.Key("source"), &w.value.Source, "本机源目录", "选择需要监听的目录", true, false, nil)
					})
					ui.Field(c, text(c, "目标目录"), func() { s.storagePathInput(c.Key("destination"), &w.value.Destination, "目标目录", false) })
				} else {
					var labels []string
					for _, workflow := range s.workflows {
						labels = append(labels, workflow.Name+" · "+workflow.ID[:min(6, len(workflow.ID))])
					}
					ui.Field(c, text(c, "工作流"), func() { ui.Select(c.Key("workflow"), &w.workflow, labels).Label(text(c, "选择工作流")) })
					ui.Field(c, text(c, "间隔（分钟）"), func() { ui.TextInput(c.Key("minutes"), &w.minutes).Placeholder("60") })
				}
			})
			ui.Checkbox(c.Key("enabled"), &w.value.Enabled, text(c, "保存后启用"))
			ui.Text(c, text(c, "目录监听只复制新增与修改的文件；保留目标已有文件。定期扫描补足监听遗漏。")).FontSize(11).TextColor(t.TextMuted)
			if s.isError && s.notice != "" {
				ui.Text(c, s.notice).FontSize(12).TextColor(t.Danger).MaxLines(3)
			}
		})
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel"), text(c, "取消")).OnClick(func() { w.open = false })
			ui.PrimaryButton(c.Key("save"), text(c, "保存")).Disabled(s.busy).OnClick(func() {
				value := w.value
				if value.Kind == "interval" {
					minutes, err := strconv.ParseInt(w.minutes, 10, 64)
					if err != nil || minutes < 1 || minutes > 525600 {
						s.setError(fmt.Errorf("%s", text(c, "间隔须为 1–525600 分钟")))
						return
					}
					value.IntervalSeconds = minutes * 60
					for _, workflow := range s.workflows {
						if workflow.Name+" · "+workflow.ID[:min(6, len(workflow.ID))] == w.workflow {
							value.WorkflowID = workflow.ID
						}
					}
				}
				s.runAfter(text(c, "保存自动任务"), func() error { _, err := s.app.SaveAutomation(value); return err }, func() { w.open = false })
			})
		})
	})
}
