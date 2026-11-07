package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"time"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) queuePage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			label, color := "队列就绪", t.TextMuted
			if s.paused {
				label, color = "队列已暂停", t.Warning
			} else if s.queueBlockReason != "" {
				label = "等待接续"
			}
			statusDot(c, label, color)
			ui.Text(c, fmt.Sprintf("· %d 个任务", len(s.jobs))).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			pauseLabel, pauseIcon := "暂停队列", iconPause
			if s.paused {
				pauseLabel, pauseIcon = "继续队列", iconPlay
			}
			iconButton(c.Key("toggle-queue"), pauseIcon, pauseLabel).Disabled(s.busy).OnClick(func() {
				paused := !s.paused
				s.run("更新队列", func() error { return s.app.SetPaused(paused) })
			})
			ui.PrimaryButton(c.Key("show-job-form"), "新建任务…").Height(30).Padding(4, 12).
				Disabled(s.busy).OnClick(func() { s.beginJobForm(nil) })
		})
		if s.queueBlockReason != "" {
			ui.Text(c, s.queueBlockReason).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Row(c).Grow(1).MinHeight(0).Gap(20).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Gap(8).Children(func() {
				ui.Scroll(c).Grow(1).Gap(16).Children(func() {
					for _, external := range s.external {
						if !external.Available {
							continue
						}
						disclosure(c.Key("external-"+external.ID), iconQueue, external.Name, fmt.Sprintf("%.2f MiB/s", external.Stats.Speed/1048576), s.openState("external-"+external.ID, true), func() {
							ui.Text(c, "当前阶段 · "+formatBytes(external.Stats.Bytes)+" / "+formatBytes(external.Stats.TotalBytes)).
								FontSize(12).TextColor(t.TextMuted)
							ui.Text(c, "当前活动文件；其他文件由 rclone 继续扫描和调度。").FontSize(11).TextColor(t.TextMuted)
							s.transferFolders(c, external.ID, external.Tree)
						})
					}
					s.activeJobCard(c)
					s.queuedJobs(c)
					ui.Collapsible(c.Key("job-history"), fmt.Sprintf("历史记录 · %d", len(historyJobs(s.jobs))), &s.historyOpen, func() {
						if len(historyJobs(s.jobs)) == 0 {
							ui.Text(c, "暂无已结束的任务").FontSize(12).TextColor(t.TextMuted)
						} else {
							s.jobsTable(c).Height(240)
						}
					})
				})
			})
			if s.logJobID != "" {
				s.jobInspector(c)
			}
		})
	})
}

func (s *nativeUI) activeJobCard(c *ui.Context) {
	var active *core.Job
	for i := range s.jobs {
		if s.jobs[i].Status == "running" {
			active = &s.jobs[i]
			break
		}
	}
	if active == nil {
		return
	}
	t := c.Theme()
	ui.Column(c).Padding(16, 18).Gap(14).Radius(10).Background(t.Accent.Alpha(0.04)).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Icon(c, iconQueue).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, active.Name).FontSize(17).FontWeight(600).SingleLine()
				ui.Text(c, operationLabel(active.Operation)+" · "+active.Source).FontSize(11).
					TextColor(t.TextMuted).SingleLine()
			})
			ui.Button(c.Key("cancel-active"), "停止").Height(28).Padding(4, 10).
				OnClick(func() {
					id := active.ID
					s.run("停止任务", func() error { return s.app.Cancel(id) })
				})
		})
		progress := progressValue(active.Bytes, active.TotalBytes)
		if len(active.CheckingFiles) > 0 && active.ChecksTotal > 0 {
			progress = progressValue(active.ChecksDone, active.ChecksTotal)
		}
		ui.Progress(c, progress).Height(4).Label("活动任务进度")
		if len(active.CheckingFiles) > 0 && active.ChecksTotal > 0 {
			ui.Text(c, fmt.Sprintf("正在比对 %d / %d 个文件", active.ChecksDone, active.ChecksTotal)).FontSize(11).TextColor(t.TextMuted)
		} else if active.Operation == "check" {
			ui.Text(c, fmt.Sprintf("已校验 %d / %d 个文件", active.ChecksDone, active.ChecksTotal)).FontSize(11).TextColor(t.TextMuted)
		} else {
			ui.Text(c, fmt.Sprintf("已传 %d / %d 个文件", active.FilesDone, active.FilesTotal)).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Row(c).Gap(24).Wrap().Children(func() {
			s.metric(c, "已传", formatBytes(active.Bytes))
			s.metric(c, "速度", fmt.Sprintf("%.2f MiB/s", active.Speed/1024/1024))
			eta := "正在计算"
			if active.Speed > 0 && active.TotalBytes > active.Bytes {
				eta = formatDuration(float64(active.TotalBytes-active.Bytes) / active.Speed)
			}
			s.metric(c, "预计剩余", eta)
			ui.Column(c).Grow(1).MinWidth(140).Gap(4).Children(func() {
				ui.Text(c, "速度趋势").FontSize(11).TextColor(t.TextMuted)
				s.speedChart(c, s.jobSpeeds[active.ID], 32)
			})
		})
		if len(active.ActiveFiles) > 0 {
			ui.Collapsible(c.Key("active-files"), "正在传输的文件", s.openState(active.ID, true), func() {
				s.transferFolders(c, active.ID, core.TransferTree(active.ActiveFiles))
			})
		}
		if len(active.CheckingFiles) > 0 {
			ui.Collapsible(c.Key("active-checks"), fmt.Sprintf("正在比对 · %d", active.ChecksDone), s.openState(active.ID+"/checks", true), func() {
				ui.Column(c).Gap(3).Children(func() {
					for _, name := range active.CheckingFiles {
						ui.Row(c).Gap(8).Padding(6, 0).Children(func() {
							ui.Icon(c, iconFile).FontSize(15).TextColor(t.TextMuted)
							ui.Text(c, name).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(name)
						})
					}
					ui.Text(c, fmt.Sprintf("checkers %d · 仅核对已存在文件，不修改云端", active.Checkers)).FontSize(10).TextColor(t.TextMuted)
				})
			})
		}
	})
}

func (s *nativeUI) jobsTable(c *ui.Context) ui.Element {
	jobs := historyJobs(s.jobs)
	columns := []ui.TableColumn{
		{Title: "任务", MinWidth: 140},
		{Title: "状态", Width: 82},
		{Title: "进度", Width: 96},
		{Title: "操作", Width: 60},
	}
	s.jobTable.Selected = &s.jobSelected
	s.jobTable.Key = func(row int) any { return jobs[row].ID }
	s.jobTable.Label = func(row int) string { return jobs[row].Name }
	table := ui.Table(c.Key("jobs-table"), &s.jobTable, columns, len(jobs), func(row, column int) {
		if row < 0 || row >= len(jobs) {
			return
		}
		job := jobs[row]
		switch column {
		case 0:
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, job.Name).FontSize(13).FontWeight(500).SingleLine()
				ui.Text(c, operationLabel(job.Operation)).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		case 1:
			ui.Text(c, statusLabel(job.Status)).FontSize(12).TextColor(statusColor(c, job.Status)).SingleLine()
		case 2:
			value := progressValue(job.Bytes, job.TotalBytes)
			if job.Status == "completed" {
				value = 1
			}
			if value >= 0 {
				ui.Text(c, fmt.Sprintf("%.0f%%", value*100)).FontSize(12)
			} else {
				ui.Text(c, "—").TextColor(c.Theme().TextMuted)
			}
		case 3:
			iconButton(c.Key("log-"+job.ID), iconArrow, "查看任务详情").OnClick(func() { s.loadLog(job.ID) })
		}
	}).Grow(1).Label("任务记录")
	if table.Changed() && s.jobSelected >= 0 && s.jobSelected < len(jobs) {
		s.loadLog(jobs[s.jobSelected].ID)
	}
	return table
}

func (s *nativeUI) jobInspector(c *ui.Context) {
	t := c.Theme()
	var selected *core.Job
	for i := range s.jobs {
		if s.jobs[i].ID == s.logJobID {
			selected = &s.jobs[i]
			break
		}
	}
	if selected == nil {
		s.logJobID = ""
		return
	}
	ui.Column(c).Width(280).Shrink(0).MinHeight(0).Padding(0, 0, 0, 18).
		BorderWidth(0, 0, 0, 0.5).BorderColor(t.Border).Gap(14).Children(func() {
		ui.Row(c).Children(func() {
			sectionHeading(c, "任务详情")
			ui.Spacer(c)
			iconButton(c.Key("close-log"), iconClose, "关闭任务详情").OnClick(func() { s.logJobID, s.logText = "", "" })
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Gap(14).Children(func() {
			ui.Text(c, selected.Name).FontSize(15).FontWeight(600)
			statusDot(c, statusLabel(selected.Status), statusColor(c, selected.Status))
			detail := func(label, value string) {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
					ui.Text(c, value).FontSize(12).MaxLines(3)
				})
			}
			detail("源目录", selected.Source)
			detail("目标目录", selected.Destination)
			detail("创建时间", selected.CreatedAt.Local().Format("01月02日 15:04"))
			detail("执行尝试", fmt.Sprintf("第 %d 次", selected.Attempt))
			if selected.NotBefore != nil {
				detail("计划开始", selected.NotBefore.Local().Format("2006年01月02日 15:04"))
			}
			if selected.WorkflowID != "" {
				detail("工作流", fmt.Sprintf("步骤 %d / %d", selected.WorkflowStep, selected.WorkflowSize))
			}
			if selected.StartedAt != nil {
				detail("开始时间", selected.StartedAt.Local().Format("01月02日 15:04:05"))
				end := time.Now()
				if selected.FinishedAt != nil {
					end = *selected.FinishedAt
				}
				detail("运行时长", formatDuration(end.Sub(*selected.StartedAt).Seconds()))
			}
			detail("并发 / 缓冲", fmt.Sprintf("%d 传输 · %d 校验 · %s 缓冲", selected.Transfers, selected.Checkers, core.ValueOr(selected.BufferSize, "8M")))
			detail("分块 / CPU", selected.ChunkSize+" · "+cpuParallelismLabel(selected.CPUParallelism))
			detail("进程", fmt.Sprintf("PID %d · RC %d", selected.PID, selected.RCPort))
			if selected.Status == "pending" {
				ui.Button(c.Key("edit-job-parameters"), "调整参数…").OnClick(func() { s.beginJobForm(selected) })
			}
		})
		if selected.Error != "" {
			ui.Text(c, selected.Error).FontSize(12).TextColor(t.Danger)
		}
		if selected.Status == "failed" || selected.Status == "interrupted" || selected.Status == "cancelled" {
			ui.Button(c.Key("retry-"+selected.ID), "重试").Height(28).OnClick(func() {
				id := selected.ID
				s.run("重试任务", func() error { return s.app.Retry(id) })
			})
		}
		ui.Divider(c)
		sectionHeading(c, "日志")
		ui.Scroll(c).Grow(1).MinHeight(90).Padding(8).Radius(6).Background(t.Text.Alpha(0.03)).
			Children(func() { ui.Text(c, s.logText).FontSize(11).TextColor(t.TextMuted) })
	})
}

func (s *nativeUI) editOverlays(c *ui.Context) {
	dialog := func(key string, open *bool, title string, width float32, content func()) {
		ui.DialogBase(c.Key(key), open, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(25, 30, 40, 0.16))
			panel.Width(width).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(16).
				Radius(16).Background(c.Theme().Background).
				Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, 0.12))
			ui.Row(c).Children(func() {
				ui.Text(c, title).FontSize(17).FontWeight(600)
				ui.Spacer(c)
				iconButton(c.Key("close-"+key), iconClose, "关闭").OnClick(func() { *open = false })
			})
			ui.Scroll(c).MinHeight(0).Gap(14).Children(content)
			if s.isError && s.notice != "" {
				ui.Text(c, s.notice).FontSize(12).TextColor(c.Theme().Danger).MaxLines(3)
			}
		})
	}
	title := "新建任务"
	if s.editingJobID != "" {
		title = "调整任务参数"
	}
	if s.saveDefinition {
		title = "新建工作流"
	}
	dialog("job-editor", &s.showJobForm, title, 520, func() { s.jobForm(c) })
	dialog("mount-editor", &s.editingMount, "映射配置", 500, func() { s.mountEditor(c) })
	dialog("remote-editor", &s.showRemoteForm, "添加云端账户", 460, func() { s.remoteEditor(c) })
}

func (s *nativeUI) jobForm(c *ui.Context) {
	t := c.Theme()
	editing := s.editingJobID != ""
	ui.Form(c, func() {
		ui.Field(c, "名称", func() {
			ui.TextInput(c.Key("job-name"), &s.jobName).Placeholder("文档归档").AutoFocus().Disabled(editing)
		})
		ui.Field(c, "操作", func() {
			if ui.Select(c.Key("job-operation"), &s.jobOperationChoice, []string{"复制", "同步", "校验"}).Disabled(editing || s.saveDefinition).Changed() {
				s.jobOperation = map[string]string{"复制": "copy", "同步": "sync", "校验": "check"}[s.jobOperationChoice]
			}
		})
		ui.Field(c, "源目录", func() {
			ui.TextInput(c.Key("job-source"), &s.jobSource).Placeholder("本机目录或 remote:目录").Disabled(editing)
		})
		ui.Field(c, "目标目录", func() {
			ui.TextInput(c.Key("job-destination"), &s.jobDestination).Placeholder("remote:备份目录").Disabled(editing)
		})
	})
	if !editing && s.jobOperation == "copy" {
		if s.saveDefinition {
			ui.Text(c, "保存可重复执行的目录工作流，不会立即启动传输").FontSize(12).TextColor(c.Theme().TextMuted)
		} else {
			ui.Checkbox(c.Key("job-workflow"), &s.jobWorkflow, "按子目录分步复制，完成后校验")
		}
		if s.jobWorkflow {
			ui.Text(c, "用于本机目录；步骤按目录名排序，失败时等待重试，全部复制完成后执行最终校验。").FontSize(11).TextColor(t.TextMuted)
			if !s.saveDefinition {
				ui.TextInput(c.Key("job-proof"), &s.jobProof).Placeholder("可选：已验证的重复目录排除证明")
			}
		}
	}
	ui.Text(c, "复制保留目标已有文件；同步会使目标与源目录一致。").FontSize(11).TextColor(t.TextMuted)
	ui.Collapsible(c.Key("job-advanced"), "更多选项", &s.showJobAdvanced, func() {
		ui.Form(c, func() {
			ui.Field(c, "传输并发", func() { ui.TextInput(c.Key("job-transfers"), &s.jobTransfers) })
			ui.Field(c, "校验并发", func() { ui.TextInput(c.Key("job-checkers"), &s.jobCheckers) })
			ui.Field(c, "限速", func() { ui.TextInput(c.Key("job-bandwidth"), &s.jobBandwidth).Placeholder("留空表示不限速") })
			ui.Field(c, "分块大小", func() { ui.TextInput(c.Key("job-chunk"), &s.jobChunk).Placeholder("16M") })
			ui.Field(c, "每路缓冲", func() { ui.TextInput(c.Key("job-buffer"), &s.jobBuffer).Placeholder("8M") })
			ui.Field(c, "CPU 并行度", func() { ui.TextInput(c.Key("job-cpu"), &s.jobCPU).Placeholder("0 表示自动") })
			ui.Field(c, "计划开始", func() {
				ui.TextInput(c.Key("job-schedule"), &s.jobSchedule).Placeholder("YYYY-MM-DD HH:MM，留空表示接续执行").Disabled(s.saveDefinition)
			})
			ui.Field(c, "包含规则", func() {
				ui.TextArea(c.Key("job-includes"), &s.jobIncludes).Height(60).Placeholder("每行一条规则").Disabled(editing || s.jobWorkflow)
			})
		})
		ui.Checkbox(c.Key("job-checksum"), &s.jobChecksum, "按校验和比较").Disabled(editing || s.jobWorkflow)
		ui.Checkbox(c.Key("job-create-dirs"), &s.jobCreateDirs, "创建空目录").Disabled(editing || s.jobWorkflow)
	})
	if s.jobOperation == "sync" {
		ui.Checkbox(c.Key("job-allow-delete"), &s.jobAllowDelete, "允许删除目标多余文件")
		ui.Text(c, "单次最多删除 10 项。").FontSize(11).TextColor(t.Warning)
	}
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		ui.Button(c.Key("hide-job-form"), "取消").Height(30).OnClick(func() { s.showJobForm = false })
		label := "加入队列"
		if editing {
			label = "保存参数"
		}
		if s.saveDefinition {
			label = "保存工作流"
		}
		ui.PrimaryButton(c.Key("add-job"), label).Height(30).Disabled(s.busy).OnClick(s.addJob)
	})
}
