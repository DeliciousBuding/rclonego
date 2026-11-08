package desktop

import (
	"context"
	"fmt"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

type cacheUIState struct {
	open, loading, offlineOpen bool
	id                         string
	view                       core.MountCacheView
	err                        string
	name, source, destination  string
	file                       bool
	list                       ui.ListState
}

func (s *nativeUI) inspectCache(id string) {
	w := &s.cacheUI
	w.open, w.loading, w.id, w.err = true, true, id, ""
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		view, err := s.app.MountCache(ctx, id)
		s.win.Update(func() {
			if w.id != id || !w.open {
				return
			}
			w.loading, w.view = false, view
			if err != nil {
				w.err = err.Error()
			}
		})
	}()
}

func (s *nativeUI) cacheOverlays(c *ui.Context) {
	w := &s.cacheUI
	ui.DialogBase(c.Key("mount-cache"), &w.open, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(540).MaxWidthPercent(90).MaxHeightPercent(88).Padding(20).Gap(14).Radius(16).Background(t.Background)
		ui.Row(c).Children(func() {
			sectionHeading(c, "缓存与写回")
			ui.Spacer(c)
			iconButton(c.Key("refresh"), iconRefresh, "刷新缓存状态").Disabled(w.loading).OnClick(func() { s.inspectCache(w.id) })
			iconButton(c.Key("close"), iconClose, "关闭缓存状态").OnClick(func() { w.open = false })
		})
		if w.loading {
			ui.Text(c, text(c, "正在读取缓存状态…")).FontSize(12).TextColor(t.TextMuted)
			return
		}
		if w.err != "" {
			ui.Text(c, w.err).FontSize(12).TextColor(t.Danger).MaxLines(3)
			return
		}
		if !w.view.Available {
			ui.Text(c, w.view.Error).FontSize(12).TextColor(t.TextMuted).MaxLines(3)
			return
		}
		contentScroll(c).Gap(12).Children(func() {
			if disk := w.view.DiskCache; disk != nil {
				ui.Row(c).Gap(20).Wrap().Children(func() {
					s.metric(c, "缓存大小", formatBytes(disk.BytesUsed))
					s.metric(c, "缓存文件", fmt.Sprint(disk.Files))
					s.metric(c, "等待写回", fmt.Sprint(disk.UploadsQueued))
					s.metric(c, "正在写回", fmt.Sprint(disk.UploadsInProgress))
				})
				if disk.OutOfSpace || disk.ErroredFiles > 0 {
					ui.Text(c, fmt.Sprintf(text(c, "缓存空间不足：%t · %d 个文件需要处理"), disk.OutOfSpace, disk.ErroredFiles)).FontSize(12).TextColor(t.Warning)
				}
			}
			ui.Text(c, text(c, "VFS 缓存可以被回收；需要长期离线使用时，请保存独立的本机副本。")).FontSize(11).TextColor(t.TextMuted).MaxLines(3)
			sectionHeading(c, "写回队列")
			ui.List(c.Key("cache-queue"), &w.list, len(w.view.Queue), func(i int) {
				entry := w.view.Queue[i]
				ui.Row(c).Gap(10).MinWidth(0).Children(func() {
					ui.Text(c, entry.Name).Grow(1).MinWidth(0).SingleLine().Tooltip(entry.Name).FontSize(12)
					ui.Text(c, formatBytes(entry.Size)).FontSize(11).TextColor(t.TextMuted)
				})
			}).Height(180).MinWidth(0).Label(text(c, "缓存写回队列"))
			if w.view.QueueError != "" {
				ui.Text(c, w.view.QueueError).FontSize(11).TextColor(t.Warning).MaxLines(3)
			}
			if w.view.QueueTruncated {
				ui.Text(c, text(c, "当前仅显示前 200 项")).FontSize(11).TextColor(t.TextMuted)
			}
		})
	})
	ui.DialogBase(c.Key("offline-copy"), &w.offlineOpen, func(backdrop, panel ui.Element) {
		t := c.Theme()
		backdrop.Background(t.Text.Alpha(0.12))
		panel.Width(520).MaxWidthPercent(90).MaxHeightPercent(88).Padding(22).Gap(14).Radius(16).Background(t.Background)
		sectionHeading(c, "保存离线副本")
		ui.Form(c, func() {
			ui.Field(c, text(c, "名称"), func() { ui.TextInput(c.Key("name"), &w.name).AutoFocus() })
			ui.Field(c, text(c, "云端源路径"), func() { s.storagePathInput(c.Key("source"), &w.source, "云端源路径", false) })
			ui.Field(c, text(c, "本机目标目录"), func() {
				s.pathInput(c.Key("destination"), &w.destination, "本机目标目录", "选择独立的 SSD 目录", true, false, nil)
			})
		})
		ui.Checkbox(c.Key("file"), &w.file, text(c, "源路径是单个文件"))
		ui.Text(c, text(c, "创建正常复制任务，完成后可直接打开本机副本。副本独立于挂载缓存；同名文件按复制规则更新。")).FontSize(11).TextColor(t.TextMuted).MaxLines(3)
		if s.isError && s.notice != "" {
			ui.Text(c, s.notice).FontSize(12).TextColor(t.Danger).MaxLines(3)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c.Key("cancel"), text(c, "取消")).OnClick(func() { w.offlineOpen = false })
			ui.PrimaryButton(c.Key("copy"), text(c, "加入队列")).Disabled(s.busy).OnClick(func() {
				request := core.OfflineCopyRequest{Name: w.name, Source: w.source, Destination: w.destination, File: w.file}
				s.runAfter(text(c, "保存离线副本"), func() error { _, err := s.app.AddOfflineCopy(request); return err }, func() { w.offlineOpen = false; s.page = "queue" })
			})
		})
	})
}
