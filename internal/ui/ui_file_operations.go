package desktop

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

// Only the latest file operation is retained here. Jobs and attempts remain in
// the application's existing durable queue.
type fileOperationItem struct {
	entry  core.DirectoryEntry
	upload bool
	key    string
	jobID  string
	err    error
}

type fileOperationState struct {
	items       []fileOperationItem
	destination string
	detailsOpen bool
}

func (operation fileOperationState) counts() (queued, failed int) {
	for _, item := range operation.items {
		if item.jobID != "" {
			queued++
		} else if item.err != nil {
			failed++
		}
	}
	return
}

func enqueueFileOperation(app *core.App, operation fileOperationState) fileOperationState {
	operation.items = append([]fileOperationItem(nil), operation.items...)
	for i := range operation.items {
		item := &operation.items[i]
		if item.jobID != "" {
			continue
		}
		item.err = nil
		if item.key == "" {
			item.key, item.err = core.RandomID()
			if item.err != nil {
				continue
			}
		}
		entry := item.entry
		if item.upload {
			info, err := os.Stat(entry.Path)
			if err != nil {
				item.err = err
				continue
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				item.err = fmt.Errorf("unsupported upload file: %s", entry.Name)
				continue
			}
			entry.IsDir = info.IsDir()
		}
		target, err := core.ChildPath(operation.destination, entry.Name)
		if err != nil {
			item.err = err
			continue
		}
		kind := "copyto"
		if entry.IsDir {
			kind = "copy"
		}
		job, err := app.AddJob(core.JobCreateRequest{
			Name: entry.Name, Operation: kind, Source: entry.Path, Destination: target,
			CreateDirs: entry.IsDir, IdempotencyKey: item.key,
		})
		item.err = err
		if err == nil {
			item.jobID = job.ID
		}
	}
	return operation
}

func (s *nativeUI) queueFileOperation(operation fileOperationState) {
	s.runAfter(localesText("加入队列"), func() error {
		operation = enqueueFileOperation(s.app, operation)
		return nil
	}, func() {
		s.filesUI.operation = &operation
		s.filesUI.actionOpen = false
		s.notice = ""
		s.jobs = s.app.Snapshot().Jobs
	})
}

func (s *nativeUI) queueFileEntries(entries []core.DirectoryEntry, destination string) {
	if len(entries) == 0 {
		return
	}
	operation := fileOperationState{destination: destination}
	for _, entry := range entries {
		operation.items = append(operation.items, fileOperationItem{entry: entry})
	}
	s.queueFileOperation(operation)
}

func (s *nativeUI) showFileOperationTasks() {
	s.page, s.fileOperationFocus = "queue", true
	s.logJobID, s.jobSelected = "", -1
	s.jobTable = ui.ListState{}
}

func (s *nativeUI) fileOperationFeedback(c *ui.Context) {
	operation := s.filesUI.operation
	if operation == nil {
		return
	}
	t := c.Theme()
	queued, failed := operation.counts()
	ui.Row(c).Shrink(0).MinHeight(30).Gap(6).BorderWidth(0, 0, 0.5, 0).
		BorderColor(t.Border).Role(ui.RoleStatus).Children(func() {
		label := fmt.Sprintf(text(c, "已加入 %d 项任务"), queued)
		color := t.TextMuted
		if failed > 0 {
			label = fmt.Sprintf(text(c, "已加入 %d 项，%d 项未加入"), queued, failed)
			color = t.Warning
		}
		ui.Text(c, label).Grow(1).MinWidth(0).SingleLine().Tooltip(label).FontSize(12).TextColor(color)
		if failed > 0 {
			iconButton(c.Key("file-failures"), iconInfo, "未加入的项目").OnClick(func() { operation.detailsOpen = true })
			iconButton(c.Key("retry-file-items"), iconRefresh, "重试未加入项").Disabled(s.busy).
				OnClick(func() { s.queueFileOperation(*operation) })
		}
		if queued > 0 {
			ui.Button(c.Key("view-file-tasks"), text(c, "查看本次任务")).Height(28).OnClick(s.showFileOperationTasks)
		}
		iconButton(c.Key("dismiss-file-result"), iconClose, "关闭操作结果").Disabled(s.busy).
			OnClick(func() { s.filesUI.operation = nil })
	})
}

func (s *nativeUI) fileOperationTasks(c *ui.Context) {
	operation := s.filesUI.operation
	if operation == nil {
		s.fileOperationFocus = false
		return
	}
	ui.Row(c).Shrink(0).Gap(8).Wrap().Children(func() {
		ui.Text(c, text(c, "本次文件操作")).Grow(1).FontSize(13).FontWeight(500)
		ui.Button(c.Key("return-to-files"), text(c, "返回文件")).OnClick(func() {
			s.page, s.fileOperationFocus = "files", false
		})
		ui.Button(c.Key("show-all-jobs"), text(c, "显示全部任务")).OnClick(func() {
			s.fileOperationFocus = false
			s.jobSelected, s.jobTable = -1, ui.ListState{}
		})
	})
	s.taskTable(c, fileOperationJobs(s.jobs, operation.items), true).Grow(1)
}

func uploadEntries(paths []string) []fileOperationItem {
	items := make([]fileOperationItem, 0, len(paths))
	for _, path := range paths {
		items = append(items, fileOperationItem{
			entry: core.DirectoryEntry{Name: filepath.Base(path), Path: path}, upload: true,
		})
	}
	return items
}

func (s *nativeUI) fileOperationDetails(c *ui.Context) {
	operation := s.filesUI.operation
	if operation == nil {
		return
	}
	ui.DialogBase(c.Key("file-operation-details"), &operation.detailsOpen, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(560).MaxWidthPercent(90).MaxHeightPercent(80).Padding(18).Gap(10).Radius(12).Background(t.Background)
		ui.Row(c).Gap(8).Children(func() {
			sectionHeading(c, "未加入的项目")
			ui.Spacer(c)
			iconButton(c.Key("close-file-failures"), iconClose, "关闭").OnClick(func() { operation.detailsOpen = false })
		})
		contentScroll(c).MaxHeight(260).Gap(10).Children(func() {
			for _, item := range operation.items {
				if item.err == nil {
					continue
				}
				issue := explainFileError(item.err)
				ui.Column(c.Key(item.key)).Gap(3).Children(func() {
					ui.Text(c, item.entry.Name).FontSize(12).FontWeight(500).SingleLine().Tooltip(item.entry.Name)
					ui.Text(c, text(c, issue.message)+" · "+text(c, issue.hint)).FontSize(11).MaxLines(2).TextColor(t.Warning)
					ui.Text(c, item.err.Error()).FontSize(11).TextColor(t.TextMuted).MaxLines(3).Selectable()
				})
			}
		})
		ui.Text(c, text(c, "检查失败项后重试；已加入的任务不会重复提交。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		ui.Row(c).Justify(ui.End).Children(func() {
			ui.Button(c.Key("retry-file-failures"), text(c, "重试未加入项")).Disabled(s.busy).OnClick(func() {
				operation.detailsOpen = false
				s.queueFileOperation(*operation)
			})
		})
	})
}

// Filtering preserves queue order and never mutates job data.
func fileOperationJobs(jobs []core.Job, items []fileOperationItem) []*core.Job {
	ids := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.jobID != "" {
			ids[item.jobID] = struct{}{}
		}
	}
	result := make([]*core.Job, 0, min(len(items), len(jobs)))
	for i := range jobs {
		if _, ok := ids[jobs[i].ID]; ok {
			result = append(result, &jobs[i])
		}
	}
	return result
}
