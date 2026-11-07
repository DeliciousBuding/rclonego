package desktop

import (
	"context"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

func (s *nativeUI) resourcesPage(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Gap(22).Children(func() {
		ui.Row(c).Gap(10).Children(func() {
			ui.Icon(c, iconResources).FontSize(25).TextColor(t.Accent)
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "运行与资源").FontSize(22).FontWeight(600)
				ui.Text(c, "资源计数每 15 秒采样一次，与 API 和 CLI 共用。").FontSize(12).TextColor(t.TextMuted)
			})
		})
		ui.Row(c).Gap(32).Children(func() {
			var cpu float64
			var memory int64
			threads := 0
			for _, process := range s.resources.Processes {
				cpu += process.CPUPercent
				memory += process.MemoryBytes
				threads += process.Threads
			}
			s.metric(c, "CPU 总占用", fmt.Sprintf("%.1f%%", cpu))
			s.metric(c, "私有提交内存", formatBytes(memory))
			s.metric(c, "系统线程", fmt.Sprint(threads))
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 核 · %d 个 Go 协程", s.resources.LogicalCPUs, s.resources.Goroutines)).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		sectionHeading(c, "进程")
		if s.resources.Error != "" {
			ui.Text(c, s.resources.Error).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Column(c).Gap(0).Children(func() {
			for _, process := range s.resources.Processes {
				ui.Row(c).Padding(11, 0).Gap(16).BorderWidth(0, 0, 0.5, 0).BorderColor(t.Border).Children(func() {
					ui.Column(c).Grow(1).Gap(3).Children(func() {
						ui.Text(c, processLabel(process, s.jobs, s.mounts, os.Getpid())).FontWeight(500).SingleLine()
						ui.Text(c, fmt.Sprintf("PID %d · %d 个系统线程", process.PID, process.Threads)).FontSize(11).TextColor(t.TextMuted)
						if process.Error != "" {
							ui.Text(c, process.Error).FontSize(11).TextColor(t.TextMuted)
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
		})
		ui.Text(c, "CPU 按整机 100% 计算；内存为私有提交量。系统线程由引擎调度。").FontSize(11).TextColor(t.TextMuted)
		ui.Divider(c)
		sectionHeading(c, "运行参数")
		for _, job := range s.jobs {
			if job.Status != "running" {
				continue
			}
			ui.Text(c, fmt.Sprintf("%s：%d 路传输 / %d 路校验 / %s 分块 / %s 缓冲", job.Name, job.Transfers, job.Checkers, job.ChunkSize, job.BufferSize)).
				FontSize(12)
		}
		for _, e := range s.external {
			if e.Available {
				ui.Text(c, fmt.Sprintf("%s：%d 路传输 / %d 路校验 / %s 分块 / %s 缓冲", e.Name, e.Transfers, e.Checkers, e.ChunkSize, e.BufferSize)).FontSize(12)
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
			ui.Text(c, "当前没有活动传输").FontSize(12).TextColor(t.TextMuted)
		}
		if len(targets) > 0 && ids[s.liveTarget] == "" {
			s.liveTarget = targets[0]
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Select(c.Key("bandwidth-target"), &s.liveTarget, targets).Width(230).Label("限速对象").Disabled(len(targets) == 0)
			ui.TextInput(c.Key("live-bandwidth"), &s.liveBandwidth).Width(160).Placeholder("4M，off 表示不限速")
			ui.Button(c.Key("apply-bandwidth"), "应用限速").Disabled(s.busy || len(targets) == 0).OnClick(func() {
				rate := strings.TrimSpace(s.liveBandwidth)
				id := ids[s.liveTarget]
				s.run("调整限速", func() error { return s.app.SetBandwidth(context.Background(), id, rate) })
			})
		})
		ui.Text(c, "限速即时生效；并发、缓冲和 CPU 并行度在任务启动时生效。").FontSize(11).TextColor(t.TextMuted)
		ui.Divider(c)
		ui.Collapsible(c.Key("default-settings"), "新任务默认参数", &s.showDefaults, func() {
			ui.Text(c, fmt.Sprintf("%d 路传输 · %d 路校验 · %s 分块 · %s 缓冲", s.defaults.Transfers, s.defaults.Checkers, s.defaults.ChunkSize, s.defaults.BufferSize)).FontSize(12)
			ui.Form(c, func() {
				ui.Field(c, "传输并发", func() { ui.TextInput(c.Key("default-transfers"), &s.defaultTransfers) })
				ui.Field(c, "校验并发", func() { ui.TextInput(c.Key("default-checkers"), &s.defaultCheckers) })
				ui.Field(c, "分块大小", func() { ui.TextInput(c.Key("default-chunk"), &s.defaultChunk) })
				ui.Field(c, "每路缓冲", func() { ui.TextInput(c.Key("default-buffer"), &s.defaultBuffer) })
				ui.Field(c, "CPU 并行度", func() { ui.TextInput(c.Key("default-cpu"), &s.defaultCPU) })
				ui.Field(c, "默认限速", func() {
					ui.TextInput(c.Key("default-bandwidth"), &s.defaultBandwidth).Placeholder("留空表示不限速")
				})
			})
			edited, err := s.editedDefaults()
			if err == nil {
				ui.Text(c, "传输缓冲估算 "+formatBytes(edited.BufferBudget())+"，另有校验、引擎和界面开销。").FontSize(11).TextColor(t.TextMuted)
			}
			ui.Text(c, "CPU 并行度限制同时执行 Go 代码的核心数；0 为自动，系统线程数仍由 rclone 管理。").FontSize(11).TextColor(t.TextMuted)
			ui.Button(c.Key("save-defaults"), "保存默认参数").Disabled(s.busy).OnClick(func() {
				settings, err := s.editedDefaults()
				if err != nil {
					s.setError(err)
					return
				}
				s.run("保存参数", func() error { return s.app.SaveSettings(core.SettingsRequest{Defaults: &settings}) })
			})
		})
		ui.Divider(c)
		sectionHeading(c, "rclone")
		ui.Text(c, core.ValueOr(s.version, "正在连接…")).FontSize(13)
		ui.Text(c, "Agent API · "+s.api).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, "引擎负责传输、重试、校验和云端访问；客户端负责顺序、计划时间、资源观察和状态呈现。").
			FontSize(12).TextColor(t.TextMuted)
	})
}

func (s *nativeUI) editedDefaults() (core.TransferSettings, error) {
	transfers, e1 := strconv.Atoi(s.defaultTransfers)
	checkers, e2 := strconv.Atoi(s.defaultCheckers)
	cpus, e3 := strconv.Atoi(s.defaultCPU)
	if e1 != nil || e2 != nil || e3 != nil {
		return core.TransferSettings{}, fmt.Errorf("并发和 CPU 并行度必须是整数")
	}
	settings := core.TransferSettings{
		Transfers: transfers, Checkers: checkers, CPUParallelism: cpus,
		ChunkSize: strings.TrimSpace(s.defaultChunk), BufferSize: strings.TrimSpace(s.defaultBuffer), Bandwidth: strings.TrimSpace(s.defaultBandwidth),
	}
	return settings, settings.Validate()
}

func cpuParallelismLabel(cpus int) string {
	if cpus == 0 {
		return "自动 CPU 并行度"
	}
	return fmt.Sprintf("%d 核 CPU 并行度", cpus)
}
