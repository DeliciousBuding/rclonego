package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) openState(key string, initial bool) *bool {
	if value, ok := s.treeOpen[key]; ok {
		return value
	}
	value := initial
	s.treeOpen[key] = &value
	return &value
}

func (s *nativeUI) pruneTreeState() {
	active := map[string]bool{}
	var visit func(string, []*core.TransferNode)
	visit = func(scope string, nodes []*core.TransferNode) {
		for _, node := range nodes {
			if node.Directory {
				active[scope+"/"+node.Path] = true
				visit(scope, node.Children)
			}
		}
	}
	for _, external := range s.external {
		active["external-"+external.ID] = true
		visit(external.ID, external.Tree)
	}
	for _, job := range s.jobs {
		if job.Status == "running" {
			active[job.ID] = true
			if len(job.CheckingFiles) > 0 {
				active[job.ID+"/checks"] = true
			}
			visit(job.ID, core.TransferTree(job.ActiveFiles))
		}
	}
	for key := range s.treeOpen {
		if !active[key] {
			delete(s.treeOpen, key)
		}
	}
}

func disclosure(c *ui.Context, icon *ui.SVG, name, summary string, open *bool, content func()) {
	t := c.Theme()
	ui.Column(c).Gap(2).Children(func() {
		parts := ui.CollapsibleBase(c, open)
		header := parts.Trigger.Padding(7, 4).Gap(8).Radius(7).Label(name)
		if header.Hovered() {
			header.Background(t.SurfaceHover)
		}
		header.Children(func() {
			arrow := iconChevron
			if *open {
				arrow = iconDown
			}
			ui.Icon(c, arrow).FontSize(12).TextColor(t.TextMuted)
			ui.Icon(c, icon).FontSize(17).TextColor(t.Accent)
			ui.Text(c, name).FontWeight(500).Grow(1).MinWidth(0).SingleLine().Tooltip(name)
			ui.Text(c, summary).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		panel := parts.Panel(content)
		panel.Padding(0, 0, 0, 20).Gap(3)
	})
}

func (s *nativeUI) transferFolders(c *ui.Context, scope string, nodes []*core.TransferNode) {
	for _, node := range nodes {
		if node.Directory {
			key := scope + "/" + node.Path
			open := s.openState(key, false)
			disclosure(c.Key(key), iconFolder, node.Name, fmt.Sprintf(text(c, "%d 个文件 · %.2f MiB/s"), node.FileCount, node.Speed/1048576), open, func() {
				s.transferFolders(c, scope, node.Children)
			})
		} else {
			ui.Row(c.Key(scope+"/file/"+node.Path)).Gap(9).Padding(7, 0).Children(func() {
				ui.Icon(c, iconFile).FontSize(16).TextColor(c.Theme().TextMuted)
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, node.Name).FontSize(12).SingleLine().Tooltip(node.Path)
					ui.Progress(c, progressValue(node.Bytes, node.TotalBytes)).Height(3).Label(node.Name + text(c, " 的进度"))
				})
				ui.Column(c).Width(105).Gap(2).Children(func() {
					ui.Text(c, formatBytes(node.Bytes)+" / "+formatBytes(node.TotalBytes)).FontSize(10).TextColor(c.Theme().TextMuted).SingleLine()
					ui.Text(c, fmt.Sprintf("%.2f MiB/s", node.Speed/1024/1024)).FontSize(10).TextColor(c.Theme().TextMuted)
				})
			})
		}
	}
}

func (s *nativeUI) queuedJobs(c *ui.Context) {
	pending := core.PendingJobs(s.jobs)
	if len(pending) == 0 {
		return
	}
	ui.Collapsible(c.Key("pending-jobs"), fmt.Sprintf(text(c, "等待执行 · %d"), len(pending)), &s.waitingOpen, func() {
		if len(pending) == 0 {
			ui.Text(c, text(c, "暂无等待任务")).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		for i, job := range pending {
			ui.Row(c.Key("queued-"+job.ID)).Padding(9, 0).Gap(10).
				BorderWidth(0, 0, 0.5, 0).BorderColor(c.Theme().Border).Children(func() {
				ui.Text(c, fmt.Sprintf("%02d", i+1)).FontSize(11).TextColor(c.Theme().TextMuted).Width(24)
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, job.Name).FontWeight(500).SingleLine()
					when := text(c, "接续执行")
					if job.NotBefore != nil {
						when = text(c, "计划于 ") + dateTime(c, *job.NotBefore)
					}
					if job.WorkflowID != "" {
						when = fmt.Sprintf(text(c, "步骤 %d / %d"), job.WorkflowStep, job.WorkflowSize)
						if core.WorkflowBlocked(s.jobs, job) {
							when += text(c, " · 等待前序")
						}
					}
					ui.Text(c, text(c, operationLabel(job.Operation))+" · "+when).FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
					pathSummary(c, job.Source, job.Destination)
				})
				iconButton(c.Key("up-"+job.ID), iconUp, "上移").Disabled(s.busy || i == 0 || (i > 0 && job.WorkflowID != "" && job.WorkflowID == pending[i-1].WorkflowID)).OnClick(func() {
					id := job.ID
					s.run(text(c, "调整顺序"), func() error { return s.app.Move(id, -1) })
				})
				iconButton(c.Key("down-"+job.ID), iconDown, "下移").Disabled(s.busy || i == len(pending)-1 || (i+1 < len(pending) && job.WorkflowID != "" && job.WorkflowID == pending[i+1].WorkflowID)).OnClick(func() {
					id := job.ID
					s.run(text(c, "调整顺序"), func() error { return s.app.Move(id, 1) })
				})
				iconButton(c.Key("log-"+job.ID), iconArrow, "任务详情").OnClick(func() { s.loadLog(job.ID) })
			})
		}
	})
}

func historyJobs(jobs []core.Job) []core.Job {
	var result []core.Job
	for _, job := range jobs {
		if job.Status != "pending" && job.Status != "running" {
			result = append(result, job)
		}
	}
	return result
}

func processLabel(process core.ProcessResource, jobs []core.Job, mounts []core.MountView, appPID int) string {
	if process.PID == appPID {
		return "RcloneGo"
	}
	for _, job := range jobs {
		if job.PID == process.PID && job.Status == "running" {
			return job.Name
		}
	}
	for _, mount := range mounts {
		if mount.PID == process.PID {
			return mount.Drive + " · " + mount.Name
		}
	}
	return strings.TrimSuffix(process.Name, ".exe")
}
