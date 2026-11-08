package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

type handoffUIState struct {
	open      bool
	request   core.HandoffRequest
	preview   core.HandoffPreview
	returning bool
}

func (s *nativeUI) beginHandoffForm() {
	h := &s.handoffUI
	h.open = true
	if h.request.StatePath == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			h.request.StatePath = filepath.Join(cache, "DataOpsConsole", "state.json")
		}
		if config, err := os.UserConfigDir(); err == nil {
			h.request.ConfigPath = filepath.Join(config, "rclone", "rclone.conf")
		}
		h.request.Endpoint = "http://127.0.0.1:8787"
	}
}

func (s *nativeUI) handoffPreferences(c *ui.Context) {
	ui.Column(c).Gap(9).Children(func() {
		sectionHeading(c, "旧客户端接管")
		ui.Text(c, text(c, "预览并导入旧任务；当前上传完成后，由本客户端继续后续步骤。")).
			FontSize(12).TextColor(c.Theme().TextMuted).MaxLines(2)
		if s.handoff != nil {
			s.handoffStatus(c)
			return
		}
		actionButton(c.Key("import-legacy"), iconDownload, "导入旧客户端…", false).
			Disabled(s.busy).OnClick(s.beginHandoffForm)
	})
}

func (s *nativeUI) handoffStatus(c *ui.Context) {
	h := s.handoff
	if h == nil {
		return
	}
	label := map[string]string{
		"waiting": "旧客户端执行中 · 完成当前步骤后接管",
		"blocked": "交接需要处理", "complete": "已接管 · 后续步骤由本客户端管理",
		"returned": "已交还旧客户端",
	}[h.Status]
	if label == "" {
		label = "交接需要处理"
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Icon(c, iconWorkflow).FontSize(16).TextColor(c.Theme().Accent)
		ui.Text(c, text(c, label)).FontSize(12).Grow(1).MinWidth(0).MaxLines(2)
		iconButton(c.Key("handoff-details"), iconInfo, "查看交接详情").OnClick(func() { s.handoffUI.open = true })
	})
	if h.Message != "" {
		message := h.Message
		keys := map[string]string{
			"Cannot read the previous client's final task result":                                  "handoffRuntime1",
			"The previous task definitions changed; return ownership to the previous client":       "handoffRuntime2",
			"The previous queue was resumed; pause it before continuing the handoff":               "handoffRuntime3",
			"The previous client no longer contains the active task":                               "handoffRuntime4",
			"The previous client has not recorded the exited transfer's result":                    "handoffRuntime5",
			"The previous client's final result is not available":                                  "handoffRuntime6",
			"Waiting for the previous transfer process to exit":                                    "handoffRuntime7",
			"The previous queue advanced to another step; return ownership to the previous client": "handoffRuntimeAdvanced",
		}
		if key := keys[message]; key != "" {
			message = locales.Catalog.Format(c.Locale(), key, nil)
		}
		ui.Text(c, message).FontSize(12).TextColor(c.Theme().Warning).MaxLines(3)
	}
}

func (s *nativeUI) handoffForm(c *ui.Context) {
	h := &s.handoffUI
	t := c.Theme()
	if s.handoff != nil {
		s.handoffStatus(c)
		ui.Text(c, fmt.Sprintf(text(c, "已导入 %d 个任务，保留原步骤顺序与过滤条件。"), len(s.handoff.JobIDs))).
			FontSize(12).TextColor(t.TextMuted).MaxLines(2)
		ui.Text(c, text(c, "旧客户端保持打开，当前上传不会重启。停止它会中断尚未交接的任务。")).
			FontSize(12).TextColor(t.TextMuted).MaxLines(3)
		if s.handoff.Status != "returned" {
			ui.Checkbox(c.Key("confirm-return"), &h.returning, text(c, "交还旧客户端继续执行"))
			ui.Text(c, text(c, "仅在本客户端尚未启动导入任务时可交还。")).
				FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		}
		return
	}
	before := h.request
	ui.Field(c, text(c, "旧任务文件"), func() {
		s.pathInput(c.Key("legacy-state"), &h.request.StatePath, "旧任务文件", "state.json", false, false, nil)
	})
	ui.Field(c, text(c, "原 rclone 配置"), func() {
		s.pathInput(c.Key("legacy-config"), &h.request.ConfigPath, "原 rclone 配置", "rclone.conf", false, false, nil)
	})
	ui.Field(c, text(c, "旧客户端 API"), func() {
		ui.TextInput(c.Key("legacy-endpoint"), &h.request.Endpoint).FillWidth().Placeholder("http://127.0.0.1:8787")
	})
	if h.request != before {
		h.preview = core.HandoffPreview{}
	}
	ui.Text(c, text(c, "先只读预览。开始交接时暂停旧队列派发，将账户配置复制到本应用的私人目录。")).
		FontSize(12).TextColor(t.TextMuted).MaxLines(3)
	ui.Text(c, text(c, "现有挂载继续由原程序管理，导入不会取得挂载的停止权限。")).
		FontSize(11).TextColor(t.TextMuted).MaxLines(2)
	if h.preview.PreviewKey == "" {
		return
	}
	ui.Divider(c)
	ui.Text(c, fmt.Sprintf(text(c, "%d 个任务 · %d 个账户"), len(h.preview.Jobs), len(h.preview.Remotes))).FontSize(13).FontWeight(600)
	for _, job := range h.preview.Jobs {
		ui.Column(c).Gap(3).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, job.Name).Grow(1).MinWidth(0).SingleLine().Tooltip(job.Name).FontSize(12)
				statusDot(c, statusLabel(job.Status), statusColor(c, job.Status))
			})
			pathSummary(c, job.Source, job.Destination)
		})
	}
}

func (s *nativeUI) handoffFooter(c *ui.Context) {
	h := &s.handoffUI
	if s.handoff != nil {
		if h.returning && s.handoff.Status != "returned" {
			actionButton(c.Key("return-handoff"), iconArrow, "交还旧客户端", false).Disabled(s.busy).OnClick(func() {
				s.run(text(c, "交还旧客户端"), func() error {
					_, err := s.app.ReturnHandoff(context.Background())
					return err
				})
			})
		}
		return
	}
	ui.Row(c).Gap(8).Children(func() {
		actionButton(c.Key("preview-handoff"), iconInfo, "预览导入", false).
			Disabled(s.busy || h.request.StatePath == "" || h.request.ConfigPath == "").OnClick(func() {
			request := h.request
			var preview core.HandoffPreview
			s.runAfter(text(c, "预览导入"), func() error {
				var err error
				preview, err = s.app.PreviewHandoff(context.Background(), request)
				return err
			}, func() { h.preview = preview })
		})
		ui.Spacer(c)
		actionButton(c.Key("begin-handoff"), iconPlay, "开始交接", true).
			Disabled(s.busy || h.preview.PreviewKey == "").OnClick(func() {
			request := h.request
			request.PreviewKey = h.preview.PreviewKey
			s.runAfter(text(c, "开始交接"), func() error {
				_, err := s.app.BeginHandoff(context.Background(), request)
				return err
			}, func() { s.page = "queue"; h.open = false })
		})
	})
}
