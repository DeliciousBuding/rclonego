package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"time"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) queuePage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(16).Children(func() {
		pageHeading(c, "任务队列", locales.In(c.Locale()).TaskCountLabel(len(s.jobs)), func() {
			label, color := "队列就绪", t.TextMuted
			if s.paused {
				label, color = "队列已暂停", t.Warning
			} else if s.queueBlockReason != "" {
				label = "等待接续"
			}
			statusDot(c, label, color)
			pauseLabel, pauseIcon := "暂停队列", iconPause
			if s.paused {
				pauseLabel, pauseIcon = "继续队列", iconPlay
			}
			iconButton(c.Key("toggle-queue"), pauseIcon, pauseLabel).Disabled(s.busy).OnClick(func() {
				paused := !s.paused
				s.run(text(c, "更新队列"), func() error { return s.app.SetPaused(paused) })
			})
			actionButton(c.Key("show-job-form"), iconPlus, "新建任务…", true).
				Disabled(s.busy).OnClick(func() { s.beginJobForm(nil) })
		})
		if s.queueBlockReason != "" && s.handoff == nil {
			ui.Text(c, text(c, s.queueBlockReason)).FontSize(11).TextColor(t.TextMuted)
		}
		s.handoffStatus(c)
		width, _ := c.Size()
		narrow := s.logJobID != "" && width < 1040
		var layout ui.Element
		if narrow {
			layout = ui.Column(c).Grow(1).MinHeight(0).Gap(12)
		} else {
			layout = ui.Row(c).Grow(1).MinHeight(0).Gap(20).AlignItems(ui.Stretch)
		}
		layout.Children(func() {
			if !narrow {
				ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Gap(8).Children(func() {
					contentScroll(c).Grow(1).Gap(16).Children(func() {
						if s.fileOperationFocus && s.filesUI.operation != nil {
							s.fileOperationTasks(c)
							return
						}
						if len(s.jobs) == 0 && len(s.external) == 0 {
							emptyState(c, iconQueue, "队列里还没有任务", "新建任务，复制或校验本机与云端文件")
						}
						for _, external := range s.external {
							if !external.Available {
								continue
							}
							disclosure(c.Key("external-"+external.ID), iconQueue, external.Name, fmt.Sprintf("%.2f MiB/s", external.Stats.Speed/1048576), s.openState("external-"+external.ID, true), func() {
								ui.Text(c, text(c, "外部观察 · 操作与最终结果由原客户端负责")).FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, text(c, "当前阶段 · ")+formatBytes(external.Stats.Bytes)+" / "+formatBytes(external.Stats.TotalBytes)).
									FontSize(12).TextColor(t.TextMuted)
								ui.Text(c, text(c, "当前活动文件；其他文件由 rclone 继续扫描和调度。")).FontSize(11).TextColor(t.TextMuted)
								s.transferFolders(c, external.ID, external.Tree)
							})
						}
						s.activeJobCard(c)
						s.queuedJobs(c)
						if len(historyJobs(s.jobs)) > 0 {
							ui.Collapsible(c.Key("job-history"), fmt.Sprintf(text(c, "历史记录 · %d"), len(historyJobs(s.jobs))), &s.historyOpen, func() {
								s.jobsTable(c).Height(240)
							})
						}
					})
				})
			}
			if s.logJobID != "" {
				s.jobInspector(c)
			}
		})
	})
}

func jobComparing(job core.Job) bool {
	return job.Operation == "check" || len(job.CheckingFiles) > 0 && len(job.ActiveFiles) == 0 && job.Speed <= 0
}

func jobPhase(job core.Job) string {
	if job.Operation == "check" {
		return "正在校验"
	}
	if jobComparing(job) {
		return "正在比对"
	}
	if len(job.ActiveFiles) > 0 || job.Speed > 0 {
		return "正在传输"
	}
	return "正在准备"
}

func jobProgress(job core.Job) float64 {
	if job.Status == "completed" {
		return 1
	}
	if jobComparing(job) {
		return progressValue(job.ChecksDone, job.ChecksTotal)
	}
	return progressValue(job.Bytes, job.TotalBytes)
}

func (s *nativeUI) activeJobCard(c *ui.Context) {
	var active *core.Job
	for i := range s.jobs {
		if s.jobs[i].Status == "running" || s.jobs[i].Status == "handoff" {
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
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, active.Name).FontSize(17).FontWeight(600).SingleLine().Tooltip(active.Name)
				statusDot(c, jobPhase(*active), t.Accent)
			})
			if active.Status != "handoff" {
				ui.Button(c.Key("pause-active"), text(c, "暂停")).Height(28).Padding(4, 10).
					Disabled(s.busy).
					OnClick(func() {
						id := active.ID
						s.run(text(c, "暂停任务"), func() error { return s.app.Pause(id) })
					})
				iconButton(c.Key("active-details"), iconArrow, "查看活动任务详情").OnClick(func() { s.loadLog(active.ID) })
				iconButton(c.Key("active-more"), iconMore, "更多活动任务操作").Disabled(s.busy).Menu(func(m *ui.Menu) {
					if m.Item(text(c, "取消")).Chosen() {
						id := active.ID
						s.run(text(c, "停止任务"), func() error { return s.app.Cancel(id) })
					}
				})
			} else {
				statusDot(c, "旧客户端执行中", t.TextMuted)
			}
		})
		pathSummary(c, active.Source, active.Destination)
		ui.Progress(c, jobProgress(*active)).Height(4).Label(text(c, "活动任务进度"))
		if active.Operation == "check" {
			if active.ChecksTotal > 0 {
				ui.Text(c, fmt.Sprintf(text(c, "已校验 %d / %d 个文件"), active.ChecksDone, active.ChecksTotal)).FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Text(c, text(c, "正在统计待校验文件")).FontSize(12).TextColor(t.TextMuted)
			}
		} else if jobComparing(*active) {
			message := fmt.Sprintf(text(c, "已比对 %d 个文件"), active.ChecksDone)
			if active.ChecksTotal > 0 {
				message = fmt.Sprintf(text(c, "正在比对 %d / %d 个文件"), active.ChecksDone, active.ChecksTotal)
			}
			ui.Text(c, message).FontSize(12).TextColor(t.TextMuted)
		} else if jobPhase(*active) == "正在准备" {
			ui.Text(c, text(c, "正在统计文件与传输大小")).FontSize(12).TextColor(t.TextMuted)
		} else {
			message := fmt.Sprintf(text(c, "已传 %d 个文件"), active.FilesDone)
			if active.FilesTotal > 0 {
				message = fmt.Sprintf(text(c, "已传 %d / %d 个文件"), active.FilesDone, active.FilesTotal)
			}
			ui.Text(c, message).FontSize(12).TextColor(t.TextMuted)
		}
		if jobPhase(*active) == "正在传输" {
			ui.Row(c).Gap(20).Wrap().Children(func() {
				ui.Column(c).Basis(0).Grow(1).MinWidth(100).Children(func() { s.metric(c, "已传", formatBytes(active.Bytes)) })
				ui.Column(c).Basis(0).Grow(1).MinWidth(100).Children(func() { s.metric(c, "速度", fmt.Sprintf("%.2f MiB/s", active.Speed/1024/1024)) })
				eta := "正在计算"
				if active.Speed > 0 && active.TotalBytes > active.Bytes {
					eta = durationText(c, float64(active.TotalBytes-active.Bytes)/active.Speed)
				}
				ui.Column(c).Basis(0).Grow(1).MinWidth(100).Children(func() { s.metric(c, "预计剩余", eta) })
			})
			if len(s.jobSpeeds[active.ID]) > 1 {
				s.speedChart(c, s.jobSpeeds[active.ID], 28)
			}
		}
		if len(active.ActiveFiles) > 0 {
			ui.Collapsible(c.Key("active-files"), text(c, "正在传输的文件"), s.openState(active.ID, true), func() {
				s.transferFolders(c, active.ID, core.TransferTree(active.ActiveFiles))
			})
		}
		if len(active.CheckingFiles) > 0 {
			ui.Collapsible(c.Key("active-checks"), fmt.Sprintf(text(c, "正在比对 · %d"), active.ChecksDone), s.openState(active.ID+"/checks", true), func() {
				ui.Column(c).Gap(3).Children(func() {
					for _, name := range active.CheckingFiles {
						ui.Row(c).Gap(8).Padding(6, 0).Children(func() {
							ui.Icon(c, iconFile).FontSize(15).TextColor(t.TextMuted)
							ui.Text(c, name).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(name)
						})
					}
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
				ui.Text(c, text(c, operationLabel(job.Operation))).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		case 1:
			ui.Text(c, text(c, statusLabel(job.Status))).FontSize(12).TextColor(statusColor(c, job.Status)).SingleLine()
		case 2:
			value := jobProgress(job)
			if value >= 0 {
				ui.Text(c, fmt.Sprintf("%.0f%%", value*100)).FontSize(12)
			} else {
				ui.Text(c, "—").TextColor(c.Theme().TextMuted)
			}
		case 3:
			iconButton(c.Key("log-"+job.ID), iconArrow, "查看任务详情").OnClick(func() { s.loadLog(job.ID) })
		}
	}).Grow(1).Label(text(c, "任务记录"))
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
	panel := ui.Column(c).Width(280).Shrink(0).MinWidth(0).MinHeight(0).Padding(0, 0, 0, 18).
		BorderWidth(0, 0, 0, 0.5).BorderColor(t.Border).Gap(14)
	if width, _ := c.Size(); width < 1040 {
		panel.WidthPercent(100).Grow(1).Shrink(1).Padding(0).BorderWidth(0)
	}
	panel.Children(func() {
		ui.Row(c).Children(func() {
			sectionHeading(c, "任务详情")
			ui.Spacer(c)
			iconButton(c.Key("close-log"), iconClose, "关闭任务详情").OnClick(func() { s.logJobID, s.logText = "", "" })
		})
		contentScroll(c).Grow(1).Gap(14).Children(func() {
			if s.fileOperationFocus {
				ui.Button(c.Key("return-to-file-position"), text(c, "返回文件")).OnClick(func() {
					s.page, s.fileOperationFocus = "files", false
				})
			}
			ui.Text(c, selected.Name).FontSize(15).FontWeight(600).MaxLines(2).Tooltip(selected.Name)
			statusDot(c, statusLabel(selected.Status), statusColor(c, selected.Status))
			if selected.Status == "completed" {
				summary := "复制已完成；未执行独立内容校验。"
				if selected.Operation == "sync" {
					summary = "同步已完成；未执行独立内容校验。"
				} else if selected.Operation == "mkdir" || selected.Operation == "moveto" || selected.Operation == "deletefile" || selected.Operation == "rmdir" {
					summary = "文件操作已完成，返回目录查看结果。"
				} else if selected.Operation == "check" {
					mode := "backend_check_completed: size and available hashes; hash availability not attested"
					if selected.VerificationMode == "content" {
						mode = "content_compared"
					} else if selected.VerificationMode == "size" {
						mode = "size_compared"
					}
					summary = workflowVerificationLabel(c, mode)
				}
				ui.Text(c, text(c, summary)).FontSize(12).TextColor(t.TextMuted).MaxLines(3)
			}
			detail := func(label, value string) {
				ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
					ui.Text(c, text(c, label)).Width(76).Shrink(0).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
					ui.Text(c, value).Grow(1).MinWidth(0).FontSize(12).MaxLines(2).Tooltip(value).Selectable()
				})
			}
			detail(text(c, "源目录"), selected.Source)
			detail(text(c, "目标目录"), selected.Destination)
			if selected.NotBefore != nil {
				detail(text(c, "计划开始"), dateTime(c, *selected.NotBefore))
			}
			if selected.WorkflowID != "" {
				detail(text(c, "工作流"), fmt.Sprintf(text(c, "步骤 %d / %d"), selected.WorkflowStep, selected.WorkflowSize))
			}
			ui.Collapsible(c.Key("job-execution-details"), text(c, "执行详情"), &s.showJobDetails, func() {
				detail(text(c, "创建时间"), dateTime(c, selected.CreatedAt))
				detail(text(c, "执行尝试"), fmt.Sprintf(text(c, "第 %d 次"), selected.Attempt))
				if selected.StartedAt != nil {
					detail(text(c, "开始时间"), dateTime(c, *selected.StartedAt))
					end := time.Now()
					if selected.FinishedAt != nil {
						end = *selected.FinishedAt
					}
					detail(text(c, "运行时长"), durationText(c, end.Sub(*selected.StartedAt).Seconds()))
				}
				detail(text(c, "并发 / 缓冲"), fmt.Sprintf(text(c, "%d 传输 · %d 校验 · %s 缓冲"), selected.Transfers, selected.Checkers, core.ValueOr(selected.BufferSize, "8M")))
				if selected.Operation != "check" && core.DestinationCapabilities(selected.Destination, s.remotes).DriveChunkSize {
					detail(text(c, "Google Drive 分块 / CPU"), selected.ChunkSize+" · "+cpuParallelismLabel(c, selected.CPUParallelism))
				} else {
					detail(text(c, "CPU 并行度"), cpuParallelismLabel(c, selected.CPUParallelism))
				}
				if selected.PID > 0 {
					detail(text(c, "进程"), fmt.Sprintf("PID %d · RC %d", selected.PID, selected.RCPort))
				}
			})
			if selected.Status == "pending" {
				ui.Button(c.Key("edit-job-parameters"), text(c, "调整参数…")).OnClick(func() { s.beginJobForm(selected) })
			}
			if selected.Error != "" {
				ui.Text(c, selected.Error).FontSize(12).TextColor(t.Danger).MaxLines(3).Tooltip(selected.Error).Selectable()
			}
			if selected.Status == "failed" || selected.Status == "interrupted" || selected.Status == "cancelled" || selected.Status == "paused" {
				label := "重试"
				if selected.Status == "paused" {
					label = "恢复"
				}
				ui.Button(c.Key("retry-"+selected.ID), text(c, label)).Height(28).Disabled(s.busy).OnClick(func() {
					id := selected.ID
					s.run(text(c, "重试任务"), func() error { return s.app.Retry(id) })
				})
			}
			ui.Divider(c)
			sectionHeading(c, "日志")
			ui.Scroll(c).Height(180).Shrink(0).Padding(8).Radius(6).Background(t.Text.Alpha(0.03)).
				Children(func() { ui.Text(c, s.logText).FontSize(11).TextColor(t.TextMuted).Selectable() })
		})
	})
}

func (s *nativeUI) editOverlays(c *ui.Context) {
	dialog := func(key string, open *bool, title string, width float32, content, footer func()) {
		ui.DialogBase(c.Key(key), open, func(backdrop, panel ui.Element) {
			backdrop.Background(ui.RGBA(25, 30, 40, 0.16))
			panel.Width(width).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(16).
				Radius(16).Background(c.Theme().Background).
				Shadow(0, 12, 36, 0, ui.RGBA(0, 0, 0, 0.12))
			ui.Row(c).Shrink(0).Children(func() {
				ui.Text(c, text(c, title)).FontSize(17).FontWeight(600)
				ui.Spacer(c)
				iconButton(c.Key("close-"+key), iconClose, fmt.Sprintf(text(c, "关闭%s"), text(c, title))).OnClick(func() {
					if key == "remote-editor" && s.account.ID != "" {
						id := s.account.ID
						s.runAfter(text(c, "关闭配置"), func() error { return s.app.CancelAccount(id) }, func() { s.account = core.AccountView{}; s.showRemoteForm = false })
						return
					}
					*open = false
				})
			})
			contentScroll(c).MinHeight(0).Shrink(1).Gap(14).Children(content)
			if s.isError && s.notice != "" {
				ui.Text(c, s.notice).Shrink(0).FontSize(12).TextColor(c.Theme().Danger).MaxLines(3)
			}
			if footer != nil {
				ui.Divider(c).Shrink(0)
				footer()
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
	dialog("job-editor", &s.showJobForm, title, 520, func() { s.jobForm(c) }, func() { s.jobFooter(c) })
	dialog("mount-editor", &s.editingMount, "映射配置", 500, func() { s.mountEditor(c) }, func() { s.mountFooter(c) })
	dialog("remote-editor", &s.showRemoteForm, "添加云端账户", 460, func() { s.remoteEditor(c) }, nil)
	dialog("legacy-handoff", &s.handoffUI.open, "旧客户端接管", 580, func() { s.handoffForm(c) }, func() { s.handoffFooter(c) })
	dialog("storage-details", &s.showStorageDetails, "云端账户详情", 520, func() { s.storageDetails(c) }, func() {
		actionButton(c.Key("browse-storage"), iconFolder, "浏览文件", false).OnClick(func() {
			s.showStorageDetails = false
			s.beginStorageBrowse(s.storageDetail)
		})
	})
	dialog("about", &s.showAbout, "关于", 440, func() { s.aboutContent(c) }, nil)
	dialog("licenses", &s.showLicenses, "许可文本", 560, func() { s.licensesContent(c) }, nil)
}

func (s *nativeUI) jobForm(c *ui.Context) {
	if s.jobPreview.ID != "" && (s.jobPreviewKey != s.currentPreviewKey() || time.Now().After(s.jobPreview.ExpiresAt)) {
		s.jobPreview = core.Preview{}
	}
	t := c.Theme()
	editing := s.editingJobID != ""
	_, issue := s.editedJob()
	showIssue := func(field string) {
		if s.jobAttempted && issue != nil && issue.field == field {
			ui.Text(c, text(c, issue.message)).FontSize(11).TextColor(t.Danger).MaxLines(2)
		}
	}
	ui.Form(c, func() {
		ui.Field(c, text(c, "名称"), func() {
			ui.TextInput(c.Key("job-name"), &s.jobName).Label(text(c, "任务名称")).
				Placeholder(text(c, "文档归档")).AutoFocus().Disabled(editing).
				Invalid(s.jobAttempted && issue != nil && issue.field == "name")
			showIssue("name")
		})
		ui.Field(c, text(c, "操作"), func() {
			if translatedSelect(c.Key("job-operation"), &s.jobOperationChoice, []string{"复制", "同步", "校验"}).Disabled(editing || s.saveDefinition).Changed() {
				s.jobOperation = map[string]string{"复制": "copy", "同步": "sync", "校验": "check"}[s.jobOperationChoice]
			}
		})
		ui.Field(c, text(c, "源目录"), func() {
			s.storagePathInput(c.Key("job-source"), &s.jobSource, "源目录", editing)
			showIssue("source")
		})
		ui.Field(c, text(c, "目标目录"), func() {
			s.storagePathInput(c.Key("job-destination"), &s.jobDestination, "目标目录", editing)
			showIssue("destination")
		})
	})
	if !editing && s.jobOperation == "copy" {
		if s.saveDefinition {
			ui.Text(c, text(c, "保存可重复执行的目录工作流，不会立即启动传输")).FontSize(12).TextColor(c.Theme().TextMuted)
		} else {
			ui.Checkbox(c.Key("job-workflow"), &s.jobWorkflow, text(c, "按子目录分步复制，完成后校验"))
		}
		if s.jobWorkflow {
			ui.Text(c, text(c, "用于本机目录；步骤按目录名排序，失败时等待重试，全部复制完成后执行最终校验。")).FontSize(11).TextColor(t.TextMuted)
			if !s.saveDefinition {
				ui.TextInput(c.Key("job-proof"), &s.jobProof).Placeholder(text(c, "可选：已验证的重复目录排除证明"))
			}
		}
	}
	ui.Text(c, text(c, "复制保留目标已有文件；同步会使目标与源目录一致。")).FontSize(11).TextColor(t.TextMuted)
	ui.Collapsible(c.Key("job-advanced"), text(c, "更多选项"), &s.showJobAdvanced, func() {
		ui.Form(c, func() {
			ui.Field(c, text(c, "传输并发"), func() { ui.TextInput(c.Key("job-transfers"), &s.jobTransfers) })
			ui.Field(c, text(c, "校验并发"), func() { ui.TextInput(c.Key("job-checkers"), &s.jobCheckers) })
			ui.Field(c, text(c, "限速"), func() {
				ui.TextInput(c.Key("job-bandwidth"), &s.jobBandwidth).Placeholder(text(c, "留空表示不限速"))
			})
			if s.jobOperation != "check" && core.DestinationCapabilities(s.jobDestination, s.remotes).DriveChunkSize {
				ui.Field(c, text(c, "Google Drive 上传分块"), func() { ui.TextInput(c.Key("job-chunk"), &s.jobChunk).Placeholder("16M") })
			}
			ui.Field(c, text(c, "每路缓冲"), func() { ui.TextInput(c.Key("job-buffer"), &s.jobBuffer).Placeholder("8M") })
			ui.Field(c, text(c, "CPU 并行度"), func() { ui.TextInput(c.Key("job-cpu"), &s.jobCPU).Placeholder(text(c, "0 表示自动")) })
			ui.Field(c, text(c, "计划开始"), func() {
				ui.TextInput(c.Key("job-schedule"), &s.jobSchedule).Placeholder(text(c, "YYYY-MM-DD HH:MM，留空表示接续执行")).Disabled(s.saveDefinition)
				showIssue("schedule")
			})
			ui.Field(c, text(c, "包含规则"), func() {
				ui.TextArea(c.Key("job-includes"), &s.jobIncludes).Height(60).Placeholder(text(c, "每行一条规则")).Disabled(editing || s.jobWorkflow)
			})
		})
		ui.Checkbox(c.Key("job-checksum"), &s.jobChecksum, text(c, "按校验和比较")).Disabled(editing || s.jobWorkflow)
		ui.Checkbox(c.Key("job-create-dirs"), &s.jobCreateDirs, text(c, "创建空目录")).Disabled(editing || s.jobWorkflow)
		showIssue("parameters")
	})
	if s.jobOperation == "sync" {
		ui.Checkbox(c.Key("job-allow-delete"), &s.jobAllowDelete, text(c, "允许删除目标多余文件"))
		ui.Field(c, text(c, "单次删除上限"), func() {
			ui.TextInput(c.Key("max-delete"), &s.jobMaxDelete).Placeholder("10")
			showIssue("max-delete")
		})
		ui.Text(c, text(c, "先预览，再确认加入队列。执行时继续检查删除上限。")).FontSize(11).TextColor(t.Warning)
	}
	if !editing && !s.jobWorkflow && !s.saveDefinition && s.jobOperation != "check" {
		if s.jobPreview.ID != "" {
			s.previewPanel(c)
		}
	}
}

func (s *nativeUI) jobFooter(c *ui.Context) {
	editing := s.editingJobID != ""
	previewFirst := !editing && s.jobOperation == "sync" && s.jobPreview.ID == ""
	ui.Row(c).Shrink(0).Gap(8).Children(func() {
		if !editing && !s.jobWorkflow && !s.saveDefinition && s.jobOperation != "check" && !previewFirst {
			ui.Button(c.Key("preview-sync"), text(c, "预览更改")).Height(30).Disabled(s.busy).
				OnClick(func() { s.previewOnly = true; s.addJob() })
		}
		ui.Spacer(c)
		ui.Button(c.Key("hide-job-form"), text(c, "取消")).Height(30).OnClick(func() { s.showJobForm = false })
		label := "加入队列"
		if editing {
			label = "保存参数"
		}
		if s.saveDefinition {
			label = "保存工作流"
		}
		if previewFirst {
			label = "预览更改"
		} else if !editing && s.jobOperation == "sync" {
			label = "确认同步"
		}
		blocked := !editing && s.jobOperation == "sync" && !previewFirst &&
			(!s.jobAllowDelete || s.jobPreview.ReportIncomplete || s.jobPreview.Summary.Errors > 0)
		ui.PrimaryButton(c.Key("add-job"), text(c, label)).Height(30).Disabled(s.busy || blocked).OnClick(s.addJob)
	})
}
