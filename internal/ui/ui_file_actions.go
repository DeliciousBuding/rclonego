package desktop

import (
	"context"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"time"
)

func (s *nativeUI) chooseFileLocation() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: s.win, Directory: true, Title: localesText("选择本机目录")})
		s.win.Update(func() {
			if err != nil {
				s.setError(err)
			} else if len(paths) > 0 {
				s.navigateFiles(paths[0], true)
			}
		})
	}()
}

func (s *nativeUI) chooseUploads() {
	destination := s.filesUI.result.Path
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: s.win, Multiple: true, Title: localesText("选择上传文件")})
		s.win.Update(func() {
			if err != nil {
				s.setError(err)
				return
			}
			if len(paths) == 0 {
				return
			}
			s.queueUploads(paths, destination)
		})
	}()
}

func (s *nativeUI) queueUploads(paths []string, destination string) {
	if len(paths) > 0 {
		s.queueFileOperation(fileOperationState{destination: destination, items: uploadEntries(paths)})
	}
}

func (s *nativeUI) downloadEntries(entries []core.DirectoryEntry) {
	if len(entries) == 0 {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: s.win, Directory: true, Title: localesText("选择下载目录")})
		s.win.Update(func() {
			if err != nil {
				s.setError(err)
				return
			}
			if len(paths) == 0 {
				return
			}
			s.queueFileEntries(entries, paths[0])
		})
	}()
}

func (s *nativeUI) beginFileAction(action string, entry core.DirectoryEntry) {
	f := &s.filesUI
	f.actionOpen, f.action, f.actionEntry, f.actionName, f.actionError = true, action, entry, entry.Name, ""
	if action == "mkdir" {
		f.actionName = ""
	}
}

func (s *nativeUI) beginFileCopy(entries []core.DirectoryEntry) {
	if len(entries) == 0 {
		return
	}
	f := &s.filesUI
	f.actionOpen, f.action, f.actionName, f.actionError = true, "copy", "", ""
	f.transferEntries = append([]core.DirectoryEntry(nil), entries...)
	f.actionEntry = entries[0]
}

func (s *nativeUI) submitFileAction() {
	f := &s.filesUI
	if f.action == "copy" {
		destination, entries := f.actionName, append([]core.DirectoryEntry(nil), f.transferEntries...)
		if destination == "" {
			f.actionError = localesText("请选择目标目录")
			return
		}
		s.queueFileEntries(entries, destination)
		return
	}
	request := core.FileActionRequest{Operation: f.action, Path: f.actionEntry.Path, Name: f.actionName, IsDir: f.actionEntry.IsDir, ConfirmDelete: f.action == "delete"}
	if request.Operation != "delete" {
		if _, err := core.ChildPath(f.result.Path, request.Name); err != nil {
			f.actionError = localesText(explainFileError(err).message)
			return
		}
	}
	var job core.Job
	entry := f.actionEntry
	s.runAfter(localesText("加入队列"), func() error {
		var err error
		job, err = s.app.AddFileAction(request)
		return err
	}, func() {
		f.actionOpen, s.notice = false, ""
		f.operation = &fileOperationState{items: []fileOperationItem{{entry: entry, jobID: job.ID}}}
		s.jobs = s.app.Snapshot().Jobs
	})
}

func (s *nativeUI) previewFile(entry core.DirectoryEntry) {
	f := &s.filesUI
	if f.previewCancel != nil {
		f.previewCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.previewCancel, f.previewOpen, f.previewBusy = cancel, true, true
	f.previewIssue, f.previewDetailsOpen = nil, false
	f.previewEntry = entry
	f.previewGeneration++
	generation := f.previewGeneration
	f.preview, f.previewImage = core.FilePreview{Path: entry.Path}, nil
	if s.win == nil {
		return
	}
	go func() {
		defer cancel()
		result, err := s.app.PreviewFile(ctx, entry.Path)
		var bitmap *ui.Bitmap
		if err == nil && result.Kind == "image" {
			bitmap, err = ui.DecodeBitmap(result.Data)
		}
		s.win.Update(func() {
			if generation != f.previewGeneration || !f.previewOpen {
				return
			}
			f.previewBusy = false
			if err != nil {
				f.previewIssue = err
				return
			}
			result.Data = nil
			f.preview, f.previewImage = result, bitmap
		})
	}()
}

func (s *nativeUI) fileOverlays(c *ui.Context) {
	f, t := &s.filesUI, c.Theme()
	s.fileOperationDetails(c)
	ui.DialogBase(c.Key("file-action"), &f.actionOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(420).MaxWidthPercent(90).Padding(20).Gap(12).Radius(14).Background(t.Background)
		panel.Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, ExitDuration: 80 * time.Millisecond, Enter: &ui.Motion{Y: 4, Opacity: 0}})
		title := "新建文件夹"
		if f.action == "rename" {
			title = "重命名"
		}
		if f.action == "delete" {
			title = "删除文件"
		}
		if f.action == "copy" {
			title = "复制到…"
		}
		sectionHeading(c, title)
		ui.Text(c, f.actionEntry.Path).FontSize(11).TextColor(t.TextMuted).MaxLines(2).Selectable()
		if f.action == "copy" {
			s.storagePathInput(c.Key("copy-destination"), &f.actionName, "目标目录", s.busy)
			ui.Text(c, text(c, "匹配文件将按 rclone 比较规则更新，目标独有文件会保留。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		} else if f.action == "delete" {
			ui.Text(c, text(c, "将永久删除此文件或空文件夹，不会移入回收站。")).FontSize(12).MaxLines(2)
		} else {
			input := ui.TextInput(c.Key("name"), &f.actionName).Label(text(c, "名称")).AutoFocus()
			if input.Submitted() {
				s.submitFileAction()
			}
		}
		ui.Text(c, text(c, "操作进入任务队列，完成后刷新目录查看结果。")).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		if f.actionError != "" {
			ui.Text(c, f.actionError).FontSize(12).TextColor(t.Danger)
		}
		if s.isError && s.notice != "" {
			ui.Text(c, s.notice).FontSize(12).TextColor(t.Danger).MaxLines(3)
		}
		ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
			ui.Button(c.Key("cancel"), text(c, "取消")).OnClick(func() { f.actionOpen = false })
			label := "加入队列"
			if f.action == "delete" {
				label = "确认删除"
			}
			ui.PrimaryButton(c.Key("confirm"), text(c, label)).Disabled(s.busy).OnClick(s.submitFileAction)
		})
	})
	ui.DialogBase(c.Key("file-preview"), &f.previewOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(700).MaxWidthPercent(92).MaxHeightPercent(88).Padding(20).Gap(12).Radius(14).Background(t.Background)
		panel.Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, ExitDuration: 80 * time.Millisecond, Enter: &ui.Motion{Y: 4, Opacity: 0}})
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			ui.Text(c, core.FileBaseName(f.preview.Path)).Grow(1).MinWidth(0).SingleLine().FontSize(15).FontWeight(600)
			iconButton(c.Key("close"), iconClose, "关闭预览").OnClick(func() { f.previewOpen = false })
		})
		if f.previewBusy {
			ui.Text(c, text(c, "正在读取文件…")).TextColor(t.TextMuted)
		}
		if f.previewIssue != nil {
			s.fileErrorFeedback(c.Key("preview-issue"), f.previewIssue, &f.previewDetailsOpen,
				func() { s.previewFile(f.previewEntry) }, func() { s.downloadEntries([]core.DirectoryEntry{f.previewEntry}); f.previewOpen = false })
		}
		if f.previewImage != nil {
			ui.Image(c, f.previewImage).WidthPercent(100).Height(280).Shrink(1).MinHeight(80).Fit(ui.Contain)
		} else if f.preview.Kind == "text" {
			contentScroll(c).Height(280).MinHeight(80).Shrink(1).Children(func() {
				ui.Text(c, f.preview.Text).FontSize(12).Selectable()
			})
		}
		if f.preview.Truncated {
			ui.Text(c, text(c, "仅预览文件开头，下载可查看完整内容。")).FontSize(11).TextColor(t.Warning)
		}
		ui.Text(c, f.preview.Path+" · "+formatBytes(f.preview.Size)).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(f.preview.Path)
	})
	if !f.previewOpen && f.previewCancel != nil {
		f.previewCancel()
		f.previewCancel = nil
		f.previewImage = nil
	}
}

func localesText(source string) string { return locales.Text(source) }
