package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/locales"
	"sort"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

const workflowHistoryPageSize = 20

var (
	workflowEditIcon = lineIcon(`<path d="m16 3 5 5-12 12H4v-5Z"/><path d="m14 5 5 5"/>`)
)

type workflowRunSummary struct {
	status    string
	steps     int
	completed int
}

// One pass over the already refreshed jobs replaces one full snapshot clone
// for every run in the list. Archived runs load their complete report on demand.
func workflowRunSummaries(jobs []core.Job) map[string]workflowRunSummary {
	summaries := make(map[string]workflowRunSummary)
	rank := map[string]int{"completed": 0, "pending": 1, "running": 2, "paused": 3, "cancelled": 4, "interrupted": 5, "failed": 6}
	for _, job := range jobs {
		if job.WorkflowID == "" {
			continue
		}
		summary, exists := summaries[job.WorkflowID]
		if !exists {
			summary.status = "completed"
		}
		summary.steps++
		if job.Status == "completed" {
			summary.completed++
		}
		jobRank, known := rank[job.Status]
		if !known {
			jobRank = 7
		}
		summaryRank, known := rank[summary.status]
		if !known {
			summaryRank = 7
		}
		if jobRank > summaryRank {
			summary.status = job.Status
		}
		summaries[job.WorkflowID] = summary
	}
	return summaries
}

func workflowDefinitionsMatching(definitions []core.WorkflowDefinition, query string) []core.WorkflowDefinition {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]core.WorkflowDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if query == "" || strings.Contains(strings.ToLower(definition.Name+"\n"+definition.Source+"\n"+definition.Destination), query) {
			filtered = append(filtered, definition)
		}
	}
	return filtered
}

func recentWorkflowRuns(runs []core.Run) ([]core.Run, bool) {
	rows := append([]core.Run(nil), runs...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	hasNext := len(rows) > workflowHistoryPageSize
	if hasNext {
		rows = rows[:workflowHistoryPageSize]
	}
	return rows, hasNext
}

func workflowRunMatches(run core.Run, summary workflowRunSummary, query, category string) bool {
	if query = strings.ToLower(strings.TrimSpace(query)); query != "" && !strings.Contains(strings.ToLower(run.Name+"\n"+run.ID), query) {
		return false
	}
	switch category {
	case "活动":
		return summary.status == "running" || summary.status == "pending"
	case "需处理":
		return summary.status == "failed" || summary.status == "paused" || summary.status == "interrupted" || summary.status == "cancelled"
	case "已完成":
		return summary.status == "completed"
	case "已归档":
		return summary.status == ""
	default:
		return true
	}
}

func (s *nativeUI) workflowsPage(c *ui.Context) {
	t := c.Theme()
	w := &s.workflowUI
	definitions := workflowDefinitionsMatching(s.workflows, w.definitionSearch)
	summaries := workflowRunSummaries(s.jobs)
	rows, hasNext := recentWorkflowRuns(s.runs)
	if w.historyLoaded {
		rows, hasNext = w.historyRows, w.historyHasNext
	}
	if w.historyStatus == "" {
		w.historyStatus = "全部状态"
	}
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		pageHeading(c, "工作流", "保存定义，重复执行，查看结果", func() {
			iconButton(c.Key("import-workflow"), iconDownload, "导入工作流").Disabled(s.busy).OnClick(s.importWorkflow)
			actionButton(c.Key("save-workflow"), iconPlus, "新建工作流…", true).Disabled(s.busy).
				OnClick(func() { s.beginWorkflowEditor(core.WorkflowDefinition{}, false) })
		})
		ui.TextInput(c.Key("workflow-search"), &w.definitionSearch).FillWidth().
			Label(text(c, "搜索工作流")).Placeholder(text(c, "搜索名称或路径"))
		contentScroll(c).Grow(1).Gap(18).Children(func() {
			ui.Row(c).Children(func() {
				sectionHeading(c, "已保存的工作流")
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf(text(c, "%d 项"), len(definitions))).FontSize(11).TextColor(t.TextMuted)
			})
			if len(definitions) == 0 {
				title, detail := "保存一次，重复执行", "按目录分步复制，所有步骤完成后执行最终校验"
				if w.definitionSearch != "" {
					title, detail = "没有匹配的工作流", "尝试其他名称或路径"
				}
				emptyState(c, iconWorkflow, title, detail)
			} else {
				// MyGo's native virtual list keeps large definition collections
				// bounded. History pages below are limited to twenty records.
				w.definitionList.Key = func(i int) any { return definitions[i].ID }
				w.definitionList.Label = func(i int) string { return definitions[i].Name }
				ui.List(c.Key("workflow-definitions"), &w.definitionList, len(definitions), func(i int) {
					s.workflowDefinitionRow(c.Key(definitions[i].ID), definitions[i])
				}).Height(float32(min(len(definitions)*66, 264))).Padding(0, 16, 0, 0)
			}
			ui.Row(c).Gap(8).Children(func() {
				sectionHeading(c, "执行记录")
				ui.Spacer(c)
				iconButton(c.Key("refresh-workflow-history"), iconRefresh, "刷新执行记录").Disabled(s.busy).
					OnClick(func() { s.loadWorkflowHistory(w.historyOffset) })
			})
			if len(rows) > 0 || w.historyOffset > 0 || w.historySearch != "" || w.historyStatus != "全部状态" {
				ui.Row(c).Gap(8).Children(func() {
					ui.TextInput(c.Key("history-search"), &w.historySearch).Grow(1).MinWidth(0).
						Label(text(c, "筛选本页执行记录")).Placeholder(text(c, "筛选本页名称或运行 ID"))
					translatedSelect(c.Key("history-status"), &w.historyStatus, []string{"全部状态", "活动", "需处理", "已完成", "已归档"}).
						Width(112).Label(text(c, "执行状态筛选"))
				})
			}
			visible := 0
			for _, run := range rows {
				summary := summaries[run.ID]
				if !workflowRunMatches(run, summary, w.historySearch, w.historyStatus) {
					continue
				}
				visible++
				s.workflowRunRow(c.Key("run-"+run.ID), run, summary)
			}
			if visible == 0 {
				label := "暂无执行记录"
				if len(rows) > 0 {
					label = "本页没有匹配的执行记录"
				}
				ui.Text(c, text(c, label)).FontSize(12).TextColor(t.TextMuted)
			}
			if hasNext || w.historyOffset > 0 {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, fmt.Sprintf(text(c, "第 %d 页 · 每页 %d 项"), w.historyOffset/workflowHistoryPageSize+1, workflowHistoryPageSize)).
						FontSize(11).TextColor(t.TextMuted).Grow(1).MinWidth(0)
					ui.Button(c.Key("history-previous"), text(c, "上一页")).Height(28).Disabled(s.busy || w.historyOffset == 0).
						OnClick(func() { s.loadWorkflowHistory(max(0, w.historyOffset-workflowHistoryPageSize)) })
					ui.Button(c.Key("history-next"), text(c, "下一页")).Height(28).Disabled(s.busy || !hasNext).
						OnClick(func() { s.loadWorkflowHistory(w.historyOffset + workflowHistoryPageSize) })
				})
			}
		})
	})
}

func (s *nativeUI) workflowDefinitionRow(c *ui.Context, definition core.WorkflowDefinition) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(12, 0).MinWidth(0).BorderWidth(0, 0, .5, 0).BorderColor(t.Border).Children(func() {
		ui.Icon(c, iconWorkflow).FontSize(22).TextColor(t.Accent).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, definition.Name).FontSize(13).FontWeight(600).SingleLine().Tooltip(definition.Name)
			pathSummary(c, definition.Source, definition.Destination)
		})
		actionButton(c.Key("run"), iconPlay, "执行", false).Disabled(s.busy).OnClick(func() {
			s.run(text(c, "启动工作流"), func() error { _, err := s.app.StartWorkflow(definition.ID, ""); return err })
		})
		iconButton(c.Key("edit"), workflowEditIcon, "编辑工作流").Disabled(s.busy).OnClick(func() { s.beginWorkflowEditor(definition, false) })
		iconButton(c.Key("more"), iconMore, "更多工作流操作").Disabled(s.busy).Menu(func(m *ui.Menu) {
			if m.Item(text(c, "复制工作流")).Chosen() {
				s.beginWorkflowEditor(definition, true)
			}
			if m.Item(text(c, "导出工作流")).Chosen() {
				s.exportWorkflow(definition)
			}
			m.Separator()
			if m.Item(text(c, "删除定义，保留执行记录")).Chosen() {
				s.run(text(c, "删除工作流"), func() error { return s.app.DeleteDefinition(definition.ID) })
			}
		})
	})
}

func (s *nativeUI) workflowRunRow(c *ui.Context, run core.Run, summary workflowRunSummary) {
	t := c.Theme()
	ui.Row(c).Gap(10).Padding(10, 0).MinWidth(0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, run.Name).FontSize(13).FontWeight(500).SingleLine().Tooltip(run.Name)
			detail := dateTime(c, run.CreatedAt)
			if summary.steps > 0 {
				detail += fmt.Sprintf(text(c, " · %d / %d 步完成"), summary.completed, summary.steps)
			} else {
				detail += text(c, " · 打开结果查看完整记录")
			}
			ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		if summary.status == "" {
			statusDot(c, "已归档", t.TextMuted)
		} else {
			statusDot(c, statusLabel(summary.status), statusColor(c, summary.status))
		}
		if summary.status == "failed" || summary.status == "paused" || summary.status == "interrupted" || summary.status == "cancelled" {
			ui.Button(c.Key("resume"), text(c, "恢复")).Height(28).Disabled(s.busy).OnClick(func() {
				s.run(text(c, "恢复执行"), func() error { return s.app.ActOnRun(run.ID, "resume") })
			})
		}
		ui.Button(c.Key("report"), text(c, "结果")).Height(28).Disabled(s.busy).OnClick(func() { s.openWorkflowReport(run.ID) })
	})
}

func (s *nativeUI) loadWorkflowHistory(offset int) {
	var rows []core.Run
	s.runAfter(locales.Text("读取执行记录"), func() error {
		var err error
		rows, err = s.app.ListRuns(offset, workflowHistoryPageSize+1)
		return err
	}, func() {
		w := &s.workflowUI
		w.historyOffset, w.historyLoaded = offset, true
		w.historyHasNext = len(rows) > workflowHistoryPageSize
		if w.historyHasNext {
			rows = rows[:workflowHistoryPageSize]
		}
		w.historyRows = rows
	})
}

func (s *nativeUI) openWorkflowReport(id string) {
	var report core.RunReport
	s.runAfter(locales.Text("读取执行结果"), func() error {
		var err error
		report, err = s.app.ReportRun(id)
		return err
	}, func() {
		sort.Slice(report.Steps, func(i, j int) bool { return report.Steps[i].WorkflowStep < report.Steps[j].WorkflowStep })
		s.workflowUI.report = &report
	})
}

func workflowVerificationLabel(c *ui.Context, mode string) string {
	var message string
	switch mode {
	case "content_compared":
		message = "已下载并比较文件内容"
	case "size_compared":
		message = "已比较文件大小"
	case "not_run":
		message = "尚未执行最终校验"
	case "backend_check_completed: size and available hashes; hash availability not attested":
		message = "已比较大小和后端可用哈希；哈希可用性未单独验证"
	case "":
		message = "未提供校验结果"
	default:
		return fmt.Sprintf(text(c, "校验结果：%s"), mode)
	}
	return text(c, message)
}

func (s *nativeUI) workflowReport(c *ui.Context) {
	if s.workflowUI.report == nil {
		return
	}
	t := c.Theme()
	report := s.workflowUI.report
	open := true
	ui.DialogBase(c.Key("workflow-report"), &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(25, 30, 40, .16))
		panel.Width(600).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(14).
			Radius(16).Background(t.Background).Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, .12))
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			sectionHeading(c, "执行结果")
			ui.Spacer(c)
			iconButton(c.Key("close-run-report"), iconClose, "关闭结果").OnClick(func() { open = false })
		})
		contentScroll(c).MinHeight(0).Shrink(1).Gap(12).Children(func() {
			ui.Text(c, report.Run.Name).FontSize(17).FontWeight(600).MaxLines(2).Tooltip(report.Run.Name)
			completed := 0
			var problem *core.Job
			for i := range report.Steps {
				step := &report.Steps[i]
				if step.Status == "completed" {
					completed++
				}
				if problem == nil && step.Error != "" {
					problem = step
				}
			}
			ui.Row(c).Gap(12).Wrap().Children(func() {
				statusDot(c, statusLabel(report.Status), statusColor(c, report.Status))
				ui.Text(c, fmt.Sprintf(text(c, "%d / %d 步完成"), completed, len(report.Steps))).
					FontSize(12).TextColor(t.TextMuted)
			})
			for _, step := range report.Steps {
				if step.Operation == "check" {
					pathSummary(c, step.Source, step.Destination)
					break
				}
			}
			ui.Text(c, workflowVerificationLabel(c, report.Verification)).FontSize(12).FontWeight(500).MaxLines(3)
			scope := "范围：只比较源目录中的文件；目标多余文件不属于复制校验结果。"
			if strings.Contains(report.Scope, "selected top-level") {
				scope = "范围：所选顶层条目及其内容；目标多余文件不属于校验结果。"
			}
			ui.Text(c, text(c, scope)).FontSize(11).TextColor(t.TextMuted).MaxLines(3)
			if problem != nil {
				ui.Column(c).Padding(10, 12).Gap(5).Radius(8).Background(t.Danger.Alpha(.05)).Children(func() {
					ui.Text(c, fmt.Sprintf(text(c, "需要处理：%s"), problem.Name)).FontSize(12).FontWeight(500).SingleLine().Tooltip(problem.Name)
					ui.Text(c, problem.Error).FontSize(12).TextColor(t.Danger).MaxLines(3).Tooltip(problem.Error).Selectable()
				})
			}
			ui.Divider(c)
			s.workflowUI.reportList.Key = func(i int) any { return report.Steps[i].ID }
			s.workflowUI.reportList.Label = func(i int) string { return report.Steps[i].Name }
			ui.List(c.Key("report-steps"), &s.workflowUI.reportList, len(report.Steps), func(i int) {
				step := report.Steps[i]
				ui.Row(c.Key("report-step-" + step.ID)).Gap(8).MinWidth(0).Children(func() {
					ui.Text(c, fmt.Sprintf("%d", step.WorkflowStep)).Width(24).FontSize(11).TextColor(t.TextMuted)
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						ui.Text(c, step.Name).FontSize(12).SingleLine().Tooltip(step.Name)
						ui.Text(c, locales.In(c.Locale()).AttemptCount(step.Attempt)+" · "+text(c, operationLabel(step.Operation))).
							FontSize(11).TextColor(t.TextMuted)
					})
					statusDot(c, statusLabel(step.Status), statusColor(c, step.Status))
				})
			}).Height(float32(min(len(report.Steps)*48, 192))).Padding(0, 16, 0, 0)
			ui.Text(c, text(c, "累计传输 ")+formatBytes(report.WireBytes)+" · "+locales.In(c.Locale()).AttemptCount(len(report.Attempts))).
				FontSize(11).TextColor(t.TextMuted)
		})
		if s.isError && s.notice != "" {
			ui.Text(c, s.notice).Shrink(0).FontSize(12).TextColor(t.Danger).MaxLines(3).Tooltip(s.notice)
		}
		ui.Row(c).Shrink(0).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("dismiss-run-report"), text(c, "关闭")).Height(30).OnClick(func() { open = false })
			if report.Status == "failed" || report.Status == "paused" || report.Status == "interrupted" || report.Status == "cancelled" {
				ui.PrimaryButton(c.Key("resume-reported-run"), text(c, "恢复未完成步骤")).Height(30).Disabled(s.busy).OnClick(func() {
					id := report.Run.ID
					s.runAfter(text(c, "恢复执行"), func() error { return s.app.ActOnRun(id, "resume") }, func() {
						s.workflowUI.report = nil
						s.page = "queue"
					})
				})
			}
		})
	})
	if !open {
		s.workflowUI.report = nil
	}
}
