package desktop

import (
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) overviewPage(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).MinHeight(0).Gap(24).Children(func() {
		s.activityCard(c)
		ui.Divider(c)
		ui.Row(c).Gap(32).AlignItems(ui.Stretch).Children(func() {
			s.activeMountCard(c).Basis(0).Grow(1)
			s.recentJobCard(c).Basis(0).Grow(1)
		})
		ui.Row(c).Gap(8).Padding(4, 0).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 个云端账户", len(s.remotes))).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "·").TextColor(t.TextMuted)
			ui.Text(c, strings.TrimPrefix(core.ValueOr(s.version, "正在连接 rclone…"), "rclone ")).
				FontSize(11).TextColor(t.TextMuted)
			ui.Spacer(c)
			if !s.lastUpdated.IsZero() {
				ui.Text(c, "更新于 "+s.lastUpdated.Format("15:04:05")).FontSize(11).TextColor(t.TextMuted)
			}
		})
	})
}

func (s *nativeUI) activityCard(c *ui.Context) {
	t := c.Theme()
	active := s.activeJob()
	ui.Column(c).Gap(18).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Box(c).Size(40, 40).Radius(11).Background(t.Accent.Alpha(0.08)).
				Center().Children(func() {
				ui.Icon(c, appMark).FontSize(26).TextColor(t.Accent)
			})
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				title := "传输总览"
				for _, job := range s.jobs {
					if job.Status == "running" {
						title = job.Name
						break
					}
				}
				for _, external := range s.external {
					if external.Available {
						title = external.Name
						break
					}
				}
				ui.Text(c, title).FontSize(22).FontWeight(600)
				phase := s.activityPhase()
				ui.Text(c, phase).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			label, color := "正在连接", t.TextMuted
			if s.activity.Available && s.activity.Stats != nil {
				switch {
				case s.activity.Stats.FatalError:
					label, color = "上传失败", t.Danger
				case active != nil && len(active.CheckingFiles) > 0:
					label, color = "正在比对", t.Warning
				case len(s.activity.Stats.Transferring) > 0:
					label, color = "正在上传", t.Success
				default:
					label = "等待传输"
				}
			} else if !s.lastUpdated.IsZero() {
				label = "空闲"
				if s.paused {
					label = "队列已暂停"
					color = t.Warning
				}
				if s.version == "" {
					label = "引擎未就绪"
				}
			}
			statusDot(c, label, color)
		})
		if !s.activity.Available || s.activity.Stats == nil {
			emptyState(c, iconQueue, "暂无活动传输", "任务开始后，这里会显示真实进度和速度。")
			return
		}
		st := s.activity.Stats
		doneBytes, totalBytes, workflow := workflowProgress(s.jobs, active)
		progress := progressValue(st.Bytes, st.TotalBytes)
		if workflow {
			progress = progressValue(doneBytes, totalBytes)
		}
		ui.Row(c).Gap(32).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Gap(12).Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Gap(6).Children(func() {
					pct := "—"
					if progress >= 0 {
						pct = fmt.Sprintf("%.1f", progress*100)
					}
					ui.Text(c, pct).FontSize(38).FontWeight(400)
					ui.Text(c, "%").FontSize(17).TextColor(t.TextMuted)
					ui.Spacer(c)
					ui.Text(c, "当前阶段").FontSize(11).TextColor(t.TextMuted)
				})
				ui.Progress(c, progress).Height(4).Radius(2).Label("当前阶段上传进度")
				progressText := formatBytes(st.Bytes) + " / " + formatBytes(st.TotalBytes)
				if workflow {
					progressText = formatBytes(doneBytes) + " / " + formatBytes(totalBytes)
				}
				ui.Text(c, progressText).
					FontSize(12).TextColor(t.TextMuted)
				if workflow {
					ui.Text(c, fmt.Sprintf("当前阶段 %s / %s · 第 %d / %d 步", formatBytes(st.Bytes), formatBytes(st.TotalBytes), active.WorkflowStep, active.WorkflowSize)).
						FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
			})
			ui.Column(c).Grow(1).Basis(0).Gap(10).Children(func() {
				ui.Row(c).AlignItems(ui.Baseline).Children(func() {
					ui.Text(c, fmt.Sprintf("%.2f", st.Speed/1024/1024)).FontSize(26).FontWeight(400)
					ui.Text(c, " MiB/s").FontSize(12).TextColor(t.TextMuted)
					ui.Spacer(c)
					ui.Text(c, "传输速度").FontSize(11).TextColor(t.TextMuted)
				})
				s.speedChart(c, s.activitySpeeds, 42)
				caption := "正在采集速度"
				if len(s.activitySpeeds) > 1 {
					caption = fmt.Sprintf("最近 %d 秒", (len(s.activitySpeeds)-1)*3)
					if len(s.activitySpeeds) == 60 {
						caption = "最近 3 分钟"
					}
				}
				ui.Text(c, caption).FontSize(10).TextColor(t.TextMuted)
			})
		})
		ui.Row(c).Gap(24).Wrap().Children(func() {
			ui.Text(c, fmt.Sprintf("已传 %d / %d 个文件", st.Transfers, st.TotalTransfers)).
				FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%d 个文件正在上传", len(st.Transferring))).
				FontSize(12).TextColor(t.TextMuted)
			if st.ETA > 0 {
				ui.Text(c, "约剩 "+formatDuration(st.ETA)).FontSize(12).TextColor(t.TextMuted)
			} else if workflow && st.Speed > 0 && totalBytes > doneBytes {
				ui.Text(c, "约剩 "+formatDuration(float64(totalBytes-doneBytes)/st.Speed)).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Text(c, fmt.Sprintf("%d 个错误", st.Errors)).FontSize(12).TextColor(t.TextMuted)
		})
	})
}

func (s *nativeUI) activityPhase() string {
	for _, job := range s.jobs {
		if job.Status != "running" {
			continue
		}
		if len(job.CheckingFiles) > 0 {
			return "校验 " + job.Name + " · " + job.CheckingFiles[0]
		}
		if job.Source != "" {
			return job.Source + "  →  " + job.Destination
		}
	}
	if s.activity.Stats != nil && len(s.activity.Stats.Transferring) > 0 {
		first := s.activity.Stats.Transferring[0]
		src := strings.TrimPrefix(strings.ReplaceAll(first.SrcFs, "/", `\`), `\\?\`)
		dst := first.DstFs
		if at := strings.Index(dst, "{"); at >= 0 {
			if end := strings.Index(dst[at:], "}"); end >= 0 {
				dst = dst[:at] + dst[at+end+1:]
			}
		}
		return src + "  →  " + dst
	}
	return "实时传输状态"
}

func (s *nativeUI) activeJob() *core.Job {
	for i := range s.jobs {
		if s.jobs[i].Status == "running" {
			return &s.jobs[i]
		}
	}
	return nil
}

func workflowProgress(jobs []core.Job, active *core.Job) (done, total int64, ok bool) {
	if active == nil || active.WorkflowID == "" || active.WorkflowTotalBytes <= 0 {
		return 0, 0, false
	}
	total = active.WorkflowTotalBytes
	done = active.WorkflowInitialBytes + active.Bytes
	for _, job := range jobs {
		if job.WorkflowID == active.WorkflowID && job.Status == "completed" && job.ID != active.ID {
			done += job.Bytes
		}
	}
	if done > total {
		done = total
	}
	return done, total, true
}

func (s *nativeUI) activeMountCard(c *ui.Context) ui.Element {
	return ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Children(func() {
			sectionHeading(c, "盘符映射")
			ui.Spacer(c)
			iconButton(c.Key("overview-mounts"), iconArrow, "查看全部映射").OnClick(func() { s.page = "mounts" })
		})
		if len(s.mounts) == 0 {
			emptyState(c, iconMounts, "暂无映射", "添加云端目录到本机盘符")
			return
		}
		for _, mount := range s.mounts {
			ui.Row(c).Padding(10, 0).Gap(10).BorderWidth(0, 0, 0.5, 0).
				BorderColor(c.Theme().Border).Children(func() {
				ui.Icon(c, iconMounts).FontSize(18).TextColor(c.Theme().TextMuted)
				ui.Text(c, mount.Drive).Width(28).FontWeight(500)
				ui.Text(c, mount.Name).Grow(1).SingleLine()
				label, color := "未挂载", c.Theme().TextMuted
				if mount.Running {
					label, color = "已挂载", c.Theme().Success
				}
				statusDot(c, label, color)
			})
		}
	})
}

func (s *nativeUI) recentJobCard(c *ui.Context) ui.Element {
	return ui.Column(c).Gap(10).Children(func() {
		ui.Row(c).Children(func() {
			sectionHeading(c, "最近任务")
			ui.Spacer(c)
			iconButton(c.Key("overview-queue"), iconArrow, "查看任务队列").OnClick(func() { s.page = "queue" })
		})
		if len(s.jobs) == 0 {
			emptyState(c, iconQueue, "暂无任务", "新建任务，开始备份或校验文件")
			return
		}
		for i, job := range s.jobs {
			if i >= 4 {
				break
			}
			ui.Row(c).Padding(10, 0).Gap(10).BorderWidth(0, 0, 0.5, 0).
				BorderColor(c.Theme().Border).Children(func() {
				ui.Icon(c, iconFile).FontSize(18).TextColor(c.Theme().TextMuted)
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, job.Name).SingleLine()
					ui.Text(c, operationLabel(job.Operation)).FontSize(11).TextColor(c.Theme().TextMuted)
				})
				statusDot(c, statusLabel(job.Status), statusColor(c, job.Status))
			})
		}
	})
}

func (s *nativeUI) metric(c *ui.Context, label, value string) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, value).FontSize(17).FontWeight(500)
	})
}

func statusPill(c *ui.Context, label string, color ui.Color) {
	statusDot(c, label, color)
}

func (s *nativeUI) speedChart(c *ui.Context, samples []float64, height float32) {
	accent, border := c.Theme().Accent, c.Theme().Border
	values := append([]float64(nil), samples...)
	ui.Box(c).FillWidth().Height(height).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Line(r.X, r.Y+r.H, r.X+r.W, r.Y+r.H, 0.5, border)
		if len(values) < 2 {
			return
		}
		maxValue := 1.0
		for _, value := range values {
			if value > maxValue {
				maxValue = value
			}
		}
		points := make([][2]float32, 0, len(values))
		area := &ui.Path{}
		area.MoveTo(r.X, r.Y+r.H)
		for i, value := range values {
			x := r.X + r.W*float32(i)/float32(len(values)-1)
			y := r.Y + r.H - 4 - (r.H-8)*float32(value/maxValue)
			points = append(points, [2]float32{x, y})
			area.LineTo(x, y)
		}
		area.LineTo(r.X+r.W, r.Y+r.H).Close()
		p.FillPathGradient(area, ui.LinearGradient{From: accent.Alpha(0.14), To: accent.Alpha(0.01), Angle: 180})
		p.StrokePolyline(points, 1.5, accent)
	}).Role(ui.RoleImage).Label("实时传输速度趋势")
}
