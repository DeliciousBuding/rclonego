package desktop

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) workflowsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.Text(c, "保存常用步骤，按需执行").FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			ui.PrimaryButton(c.Key("save-workflow"), "新建工作流…").Height(30).Disabled(s.busy).OnClick(func() {
				s.beginJobForm(nil)
				s.saveDefinition = true
				s.jobWorkflow = true
			})
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Gap(18).Children(func() {
			sectionHeading(c, "已保存的工作流")
			if len(s.workflows) == 0 {
				emptyState(c, iconWorkflow, "保存一次，重复执行", "按目录分步复制，所有步骤完成后执行最终校验")
			}
			for _, definition := range s.workflows {
				ui.Row(c.Key("workflow-"+definition.ID)).Gap(12).Padding(14, 0).BorderWidth(0, 0, .5, 0).BorderColor(t.Border).Children(func() {
					ui.Icon(c, iconWorkflow).FontSize(23).TextColor(t.Accent)
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, definition.Name).FontSize(13).FontWeight(600)
						ui.Text(c, definition.Source+" → "+definition.Destination).FontSize(11).TextColor(t.TextMuted).SingleLine()
					})
					ui.Button(c.Key("run-"+definition.ID), "执行").Height(28).Disabled(s.busy).OnClick(func() {
						id := definition.ID
						s.run("启动工作流", func() error { _, err := s.app.StartWorkflow(id, ""); return err })
					})
				})
			}
			sectionHeading(c, "执行记录")
			for i := len(s.runs) - 1; i >= 0; i-- {
				run := s.runs[i]
				report, err := s.app.ReportRun(run.ID)
				if err != nil {
					continue
				}
				ui.Row(c.Key("run-report-"+run.ID)).Gap(10).Padding(12, 0).Children(func() {
					ui.Column(c).Grow(1).Gap(3).Children(func() {
						ui.Text(c, run.Name).FontSize(13).FontWeight(500)
						ui.Text(c, run.CreatedAt.Local().Format("01月02日 15:04")+" · "+fmt.Sprint(len(report.Steps))+" 个步骤").
							FontSize(11).TextColor(t.TextMuted)
					})
					statusDot(c, statusLabel(report.Status), statusColor(c, report.Status))
					ui.Button(c.Key("report-"+run.ID), "结果").Height(28).OnClick(func() {
						var lines []string
						lines = append(lines, run.Name+" · "+statusLabel(report.Status))
						lines = append(lines, "校验："+report.Verification)
						for _, step := range report.Steps {
							lines = append(lines, fmt.Sprintf("%d/%d %s · %s · %d 次尝试",
								step.WorkflowStep, step.WorkflowSize, step.Name, statusLabel(step.Status), step.Attempt))
						}
						s.runReport = strings.Join(lines, "\n")
					})
				})
			}
			if len(s.runs) == 0 {
				ui.Text(c, "暂无执行记录").FontSize(12).TextColor(t.TextMuted)
			}
			if s.runReport != "" {
				ui.Column(c).Padding(14).Radius(8).Background(t.Surface).Gap(8).Children(func() {
					ui.Row(c).Children(func() {
						sectionHeading(c, "执行结果")
						ui.Spacer(c)
						iconButton(c.Key("close-run-report"), iconClose, "关闭结果").OnClick(func() { s.runReport = "" })
					})
					ui.Text(c, s.runReport).FontSize(12)
				})
			}
		})
	})
}
