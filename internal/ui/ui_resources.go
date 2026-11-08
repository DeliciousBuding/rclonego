package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) resourcesPage(c *ui.Context) {
	t := c.Theme()
	contentScroll(c).Grow(1).Gap(18).Children(func() {
		pageHeading(c, "运行与资源", "资源计数每 15 秒采样一次，与 API 和 CLI 共用。", nil)
		ui.Row(c).Gap(12).Children(func() {
			brandIcon(c, rcloneLogo, 32).Label("rclone")
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, core.ValueOr(s.version, text(c, "引擎未就绪"))).FontSize(15).FontWeight(600)
				path := s.app.EngineClient().Path
				ui.Text(c, path).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(path)
			})
			actionButton(c.Key("manage-engine"), iconSettings, "管理引擎", false).OnClick(func() {
				s.page, s.showEngineSettings = "settings", true
			})
		})
		if s.health.Code != "" && !s.health.Ready {
			ui.Column(c).Padding(12).Gap(8).Radius(8).Background(t.Warning.Alpha(0.08)).Children(func() {
				ui.Text(c, text(c, "任务调度需要处理")).FontSize(13).FontWeight(600)
				ui.Text(c, s.health.Message).FontSize(12).MaxLines(3).TextColor(t.Warning)
				if s.health.Code == "storage_unavailable" {
					ui.Button(c.Key("recover-storage"), text(c, "检查存储并恢复队列")).Disabled(s.busy).
						OnClick(func() { s.run(text(c, "恢复队列"), s.app.Recover) })
				}
			})
		}
		ui.Row(c).Gap(24).Wrap().Children(func() {
			var cpu float64
			var memory int64
			threads := 0
			cpuAvailable, memoryAvailable := len(s.resources.Processes) > 0, len(s.resources.Processes) > 0
			for _, process := range s.resources.Processes {
				if process.External {
					continue
				}
				cpu += process.CPUPercent
				memory += process.MemoryBytes
				threads += process.Threads
				cpuAvailable = cpuAvailable && process.CPUAvailable
				memoryAvailable = memoryAvailable && process.MemoryAvailable
			}
			cpuText, memoryText := "—", "—"
			if cpuAvailable {
				cpuText = fmt.Sprintf("%.1f%%", cpu)
			}
			if memoryAvailable {
				memoryText = formatBytes(memory)
			}
			ui.Column(c).Basis(0).Grow(1).MinWidth(140).Children(func() { s.metric(c, "CPU 总占用", cpuText) })
			ui.Column(c).Basis(0).Grow(1).MinWidth(140).Children(func() { s.metric(c, "进程内存", memoryText) })
			ui.Column(c).Basis(0).Grow(1).MinWidth(140).Children(func() { s.metric(c, "系统线程", fmt.Sprint(threads)) })
		})
		ui.Text(c, fmt.Sprintf(text(c, "%d 核 · %d 个 Go 协程"), s.resources.LogicalCPUs, s.resources.Goroutines)).
			FontSize(11).TextColor(t.TextMuted).Label(text(c, "运行计数"))
		ui.Divider(c)
		sectionHeading(c, "本客户端")
		ui.Row(c).Gap(16).Children(func() {
			ui.Text(c, text(c, "名称")).Grow(1).MinWidth(0).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "CPU").Width(66).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, text(c, "进程内存")).Width(96).FontSize(11).TextColor(t.TextMuted)
		})
		if s.resources.Error != "" {
			ui.Text(c, s.resources.Error).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Column(c).Gap(0).Children(func() {
			for _, process := range s.resources.Processes {
				if !process.External {
					s.processResourceRow(c, process)
				}
			}
		})
		ui.Text(c, text(c, "CPU 按整机 100% 计算；Windows 内存为私有提交，Linux 为私有驻留估算。不可读时显示未知。")).FontSize(11).TextColor(t.TextMuted)
		externalCount := 0
		for _, process := range s.resources.Processes {
			if process.External {
				externalCount++
			}
		}
		if externalCount > 0 {
			ui.Divider(c)
			ui.Collapsible(c.Key("external-processes"), fmt.Sprintf(text(c, "其他 rclone · %d"), externalCount), &s.externalOpen, func() {
				ui.Text(c, text(c, "由其他程序启动，仅观察资源，不计入本客户端总占用。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
				for _, process := range s.resources.Processes {
					if process.External {
						s.processResourceRow(c, process)
					}
				}
			})
		}
		if s.activity.Available && s.activity.Stats != nil {
			ui.Divider(c)
			sectionHeading(c, "传输诊断")
			st := s.activity.Stats
			ui.Row(c).Gap(24).Wrap().Children(func() {
				s.metric(c, "已列举", fmt.Sprint(st.Listed))
				s.metric(c, "已校验", fmt.Sprintf("%d / %d", st.Checks, st.TotalChecks))
				s.metric(c, "错误", fmt.Sprint(st.Errors))
			})
			if st.FatalError || st.RetryError {
				ui.Text(c, text(c, map[bool]string{true: "引擎报告不可恢复的错误", false: "引擎将重试可恢复的错误"}[st.FatalError])).
					FontSize(12).TextColor(t.Warning)
			}
			if st.LastError != "" {
				ui.Text(c, st.LastError).FontSize(12).TextColor(t.Warning).MaxLines(3).Selectable()
			}
			if st.ElapsedTime > 0 {
				ui.Text(c, text(c, "已运行 ")+durationText(c, st.ElapsedTime)).FontSize(11).TextColor(t.TextMuted)
			}
		}
		ui.Divider(c)
		sectionHeading(c, "运行参数")
		for _, job := range s.jobs {
			if job.Status != "running" {
				continue
			}
			if job.Operation != "check" && core.DestinationCapabilities(job.Destination, s.remotes).DriveChunkSize {
				ui.Text(c, fmt.Sprintf(text(c, "%s：%d 路传输 / %d 路校验 / %s Google Drive 分块 / %s 缓冲"), job.Name, job.Transfers, job.Checkers, job.ChunkSize, job.BufferSize)).
					FontSize(12)
			} else {
				ui.Text(c, fmt.Sprintf(text(c, "%s：%d 路传输 / %d 路校验 / %s 缓冲"), job.Name, job.Transfers, job.Checkers, job.BufferSize)).
					FontSize(12)
			}
		}
		for _, e := range s.external {
			if e.Available {
				ui.Text(c, fmt.Sprintf(text(c, "%s：%d 路传输 / %d 路校验 / %s 缓冲"), e.Name, e.Transfers, e.Checkers, e.BufferSize)).FontSize(12)
			}
		}
		targets := []string{}
		ids := map[string]string{}
		for _, job := range s.jobs {
			if job.Status == "running" {
				label := job.Name + " · " + job.ID[:8]
				targets = append(targets, label)
				ids[label] = job.ID
			}
		}
		for _, external := range s.external {
			if external.Available {
				label := external.Name + " · " + external.ID
				targets = append(targets, label)
				ids[label] = external.ID
			}
		}
		if len(targets) == 0 {
			ui.Text(c, text(c, "当前没有活动传输")).FontSize(12).TextColor(t.TextMuted)
		}
		if len(targets) > 0 && ids[s.liveTarget] == "" {
			s.liveTarget = targets[0]
		}
		if len(targets) > 0 {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Select(c.Key("bandwidth-target"), &s.liveTarget, targets).Width(230).Label(text(c, "限速对象")).Disabled(len(targets) == 0)
				ui.TextInput(c.Key("live-bandwidth"), &s.liveBandwidth).Width(160).Placeholder(text(c, "4M，off 表示不限速"))
				ui.Button(c.Key("apply-bandwidth"), text(c, "应用限速")).Disabled(s.busy || len(targets) == 0).OnClick(func() {
					rate := strings.TrimSpace(s.liveBandwidth)
					id := ids[s.liveTarget]
					s.run(text(c, "调整限速"), func() error { return s.app.SetBandwidth(context.Background(), id, rate) })
				})
			})
			ui.Text(c, text(c, "限速即时生效；并发、缓冲和 CPU 并行度在任务启动时生效。")).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Divider(c)
		ui.Collapsible(c.Key("engine-service-details"), text(c, "引擎与服务详情"), &s.showResourceDetails, func() {
			ui.Text(c, core.ValueOr(s.version, text(c, "正在连接…"))).FontSize(13)
			ui.Text(c, "Agent API · "+s.api).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, text(c, "引擎负责传输、重试、校验和云端访问；客户端负责顺序、计划时间、资源观察和状态呈现。")).
				FontSize(12).TextColor(t.TextMuted).MaxLines(2)
		})
	})
}

func (s *nativeUI) processResourceRow(c *ui.Context, process core.ProcessResource) {
	t := c.Theme()
	ui.Row(c).Padding(11, 0).Gap(16).BorderWidth(0, 0, 0.5, 0).BorderColor(t.Border).Children(func() {
		if process.PID == os.Getpid() {
			ui.Icon(c, appMark).FontSize(20).TextColor(t.Accent)
		} else {
			brandIcon(c, rcloneLogo, 20).Label("rclone")
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			label := processLabel(process, s.jobs, s.mounts, os.Getpid())
			ui.Text(c, label).FontWeight(500).SingleLine().Tooltip(label)
			ui.Text(c, fmt.Sprintf(text(c, "PID %d · %d 个系统线程"), process.PID, process.Threads)).FontSize(11).TextColor(t.TextMuted)
			if process.Error != "" {
				ui.Text(c, process.Error).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
			}
		})
		cpu, memory := "—", "—"
		if process.CPUAvailable {
			cpu = fmt.Sprintf("%.1f%%", process.CPUPercent)
		}
		if process.MemoryAvailable {
			memory = formatBytes(process.MemoryBytes)
		}
		ui.Text(c, cpu).FontSize(13).Width(66)
		ui.Text(c, memory).FontSize(13).Width(96)
	})
}

func (s *nativeUI) editedDefaults() (core.TransferSettings, error) {
	transfers, e1 := strconv.Atoi(s.defaultTransfers)
	checkers, e2 := strconv.Atoi(s.defaultCheckers)
	cpus, e3 := strconv.Atoi(s.defaultCPU)
	if e1 != nil || e2 != nil || e3 != nil {
		return core.TransferSettings{}, fmt.Errorf("%s", locales.Text("并发和 CPU 并行度必须是整数"))
	}
	settings := core.TransferSettings{
		Transfers: transfers, Checkers: checkers, CPUParallelism: cpus,
		ChunkSize: strings.TrimSpace(s.defaultChunk), BufferSize: strings.TrimSpace(s.defaultBuffer), Bandwidth: strings.TrimSpace(s.defaultBandwidth),
	}
	return settings, settings.Validate()
}

func cpuParallelismLabel(c *ui.Context, cpus int) string {
	if cpus == 0 {
		return text(c, "自动 CPU 并行度")
	}
	return fmt.Sprintf(text(c, "%d 核 CPU 并行度"), cpus)
}
