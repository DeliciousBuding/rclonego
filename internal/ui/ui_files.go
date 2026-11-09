package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type fileWorkspaceState struct {
	directoryUIState
	open               bool
	history            []string
	historyIndex       int
	breadcrumbIndex    int
	selection          ui.Selection[string]
	actionOpen         bool
	action             string
	actionEntry        core.DirectoryEntry
	actionName         string
	actionError        string
	transferEntries    []core.DirectoryEntry
	previewOpen        bool
	previewBusy        bool
	preview            core.FilePreview
	previewImage       *ui.Bitmap
	previewCancel      context.CancelFunc
	previewGeneration  int
	refreshedAt        time.Time
	operation          *fileOperationState
	editPath           bool
	pathDraft          string
	readDetailsOpen    bool
	previewIssue       error
	previewDetailsOpen bool
	previewEntry       core.DirectoryEntry
	usage              core.DirectoryUsage
	usageBusy          bool
	usageErr           string
	usageCancel        context.CancelFunc
}

func (s *nativeUI) openFiles(value string) {
	s.page = "files"
	f := &s.filesUI
	f.open = true
	if value == "" {
		if f.path != "" {
			return
		}
		if len(s.remotes) > 0 {
			value = s.remotes[0].Name + ":"
		} else {
			value, _ = os.UserHomeDir()
		}
	}
	s.navigateFiles(value, true)
}

func (s *nativeUI) navigateFiles(value string, history bool) {
	f := &s.filesUI
	if history && (len(f.history) == 0 || f.history[f.historyIndex] != value) {
		f.history = append(f.history[:min(len(f.history), f.historyIndex+1)], value)
		if len(f.history) > 32 {
			f.history = f.history[1:]
		}
		f.historyIndex = len(f.history) - 1
	}
	f.query, f.selected, f.editPath = "", -1, false
	f.selection.Clear()
	s.loadFiles(value, 0)
}

func (s *nativeUI) loadFiles(value string, offset int) {
	f := &s.filesUI
	if f.cancel != nil {
		f.cancel()
	}
	if f.usageCancel != nil {
		f.usageCancel()
	}
	f.usage, f.usageBusy, f.usageErr = core.DirectoryUsage{}, false, ""
	f.selection.Clear()
	f.selected = -1
	f.generation++
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	f.cancel, f.path, f.offset, f.loading, f.err = cancel, value, offset, true, nil
	f.refreshedAt = time.Now()
	f.readDetailsOpen = false
	generation := f.generation
	if s.win == nil {
		cancel()
		f.loading = false
		return
	}
	go func() {
		defer cancel()
		result, err := s.app.Browse(ctx, core.BrowseRequest{Path: value, Offset: offset, Limit: 100})
		s.win.Update(func() {
			if generation != f.generation {
				return
			}
			f.loading = false
			if err != nil {
				f.err = err
				return
			}
			f.result, f.path = result, result.Path
		})
	}()
}

func (s *nativeUI) filesPage(c *ui.Context) {
	f, t := &s.filesUI, c.Theme()
	ui.Column(c).Grow(1).MinHeight(0).Gap(8).Children(func() {
		if !f.open {
			s.openFiles("")
		}
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			ui.Text(c, text(c, "文件")).FontSize(18).FontWeight(600)
			ui.Spacer(c)
			actionButton(c.Key("upload"), iconUp, "上传文件…", true).Disabled(f.loading || f.err != nil || f.result.Path == "").
				OnClick(s.chooseUploads)
		})
		ui.Row(c).Shrink(0).Gap(4).Children(func() {
			iconButton(c.Key("back"), lineIconBack, "返回").Disabled(f.loading || f.historyIndex <= 0).OnClick(func() {
				f.historyIndex--
				s.navigateFiles(f.history[f.historyIndex], false)
			})
			iconButton(c.Key("forward"), iconChevron, "前进").Disabled(f.loading || f.historyIndex+1 >= len(f.history)).OnClick(func() {
				f.historyIndex++
				s.navigateFiles(f.history[f.historyIndex], false)
			})
			iconButton(c.Key("parent"), iconUp, "上一级目录").Disabled(f.loading || f.result.Parent == "").
				OnClick(func() { s.navigateFiles(f.result.Parent, true) })
			if c.Shortcut(ui.Cmd, ui.KeyL) {
				f.editPath, f.pathDraft = true, f.path
			}
			if f.editPath {
				path := ui.TextInput(c.Key("path"), &f.pathDraft).Grow(1).MinWidth(0).Label(text(c, "文件路径")).AutoFocus()
				if path.Submitted() {
					s.navigateFiles(f.pathDraft, true)
				} else if c.Shortcut(0, ui.KeyEscape) {
					f.editPath = false
				}
			} else {
				labels, paths := fileBreadcrumbs(f.path)
				if len(labels) > 4 {
					// Keep the actual location readable; earlier ancestors remain
					// reachable without displaying every level in the toolbar.
					iconButton(c.Key("ancestors"), iconMore, "上级目录").Menu(func(m *ui.Menu) {
						for i := 0; i < len(labels)-3; i++ {
							if m.Item(paths[i]).Chosen() {
								s.navigateFiles(paths[i], true)
							}
						}
					})
					labels, paths = labels[len(labels)-3:], paths[len(paths)-3:]
				}
				ui.Row(c).Grow(1).MinWidth(0).Clip().Gap(5).Children(func() {
					s.storageIcon(c, f.path, iconFolder, 16)
					if ui.Breadcrumbs(c.Key("breadcrumbs"), labels, &f.breadcrumbIndex).MinWidth(0).Label(text(c, "目录导航")).Changed() {
						s.navigateFiles(paths[f.breadcrumbIndex], true)
					}
				})
			}
			iconButton(c.Key("edit-path"), iconRename, "编辑路径").OnClick(func() {
				f.editPath, f.pathDraft = !f.editPath, f.path
			})
			iconButton(c.Key("locations"), iconMore, "选择位置").Menu(func(m *ui.Menu) {
				if m.Item(text(c, "本机目录…")).Chosen() {
					s.chooseFileLocation()
				}
				for _, remote := range s.remotes {
					if m.Item(remote.Name + ":").Chosen() {
						s.navigateFiles(remote.Name+":", true)
					}
				}
				for _, mount := range s.mounts {
					if mount.DriveAvailable && m.Item(mount.Name+" · "+mount.Drive).Chosen() {
						s.navigateFiles(mountedRoot(mount.Drive), true)
					}
				}
			})
		})
		ui.Row(c).Shrink(0).Gap(8).Children(func() {
			ui.SearchField(c.Key("filter"), &f.query).Grow(1).MinWidth(0).Placeholder(text(c, "搜索当前页…")).
				Label(text(c, "搜索当前页")).OnChange(func() { f.selected = -1; f.selection.Clear() })
			if f.order == "" {
				f.order = "名称 A–Z"
			}
			translatedSelect(c.Key("sort"), &f.order, []string{"名称 A–Z", "名称 Z–A", "大小由大到小", "修改时间由新到旧"}).
				Width(150).Label(text(c, "排序")).OnChange(func() { f.selected = -1; f.selection.Clear() })
			iconButton(c.Key("refresh-directory"), iconRefresh, "刷新目录").Disabled(f.loading).OnClick(func() { s.loadFiles(f.path, f.offset) })
			iconButton(c.Key("new-folder"), iconPlus, "新建文件夹").Disabled(f.loading || f.err != nil || f.result.Path == "").
				OnClick(func() { s.beginFileAction("mkdir", core.DirectoryEntry{Path: f.result.Path, IsDir: true}) })
			iconButton(c.Key("folder-info"), iconInfo, "统计目录大小").Disabled(f.loading || f.usageBusy || f.result.Path == "").OnClick(s.fileDirectoryUsage)
		})
		if f.loading {
			ui.Row(c).Shrink(0).Gap(8).Children(func() {
				ui.Spinner(c).Size(14, 14)
				ui.Text(c, text(c, "正在读取目录…")).FontSize(12).TextColor(t.TextMuted)
			})
		}
		if f.err != nil {
			s.fileErrorFeedback(c.Key("directory-issue"), f.err, &f.readDetailsOpen,
				func() { s.loadFiles(f.path, f.offset) }, nil)
		}
		s.fileOperationFeedback(c)
		entries := browserEntries(f.result.Entries, f.query, f.order)
		ui.Row(c).Shrink(0).Padding(0, 10).Gap(10).Children(func() {
			ui.Text(c, text(c, "名称")).Grow(1).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, text(c, "大小")).Width(75).FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, text(c, "修改时间")).Width(145).FontSize(11).TextColor(t.TextMuted)
		})
		f.list.Selected, f.list.Selection = &f.selected, &f.selection
		f.list.Key = func(i int) any { return entries[i].Path }
		f.list.Label = func(i int) string { return entries[i].Name }
		list := ui.List(c.Key("files-list"), &f.list, len(entries), func(i int) {
			entry := entries[i]
			color := t.TextMuted
			if f.selection.Has(entry.Path) {
				color = t.AccentText
			}
			ui.Row(c).MinWidth(0).Height(28).Padding(0, 6).Gap(10).
				ContextMenu(func(m *ui.Menu) { s.fileContext(c, m, entry) }).Children(func() {
				icon := iconFile
				if entry.IsDir {
					icon = iconFolder
				}
				ui.Icon(c, icon).FontSize(17).TextColor(color)
				ui.Text(c, entry.Name).Grow(1).MinWidth(0).SingleLine().Tooltip(entry.Name)
				size := formatBytes(entry.Size)
				if entry.IsDir {
					size = "—"
				}
				ui.Text(c, size).Width(75).FontSize(11).TextColor(color)
				modified := "—"
				if entry.ModTime != nil {
					modified = dateTime(c, *entry.ModTime)
				}
				ui.Text(c, modified).Width(145).FontSize(11).SingleLine().TextColor(color).Tooltip(modified)
			})
		}).Grow(1).MinHeight(32).MinWidth(0).Disabled(f.loading || f.err != nil).Label(text(c, "文件列表"))
		if list.FileDragOver() {
			list.Border(1, t.Accent)
		}
		if paths := list.DroppedFiles(); len(paths) > 0 && !f.loading && f.err == nil {
			s.queueUploads(paths, f.result.Path)
		}
		if entry := selectedBrowserEntry(entries, f.selected); entry != nil {
			if list.Submitted() {
				if entry.IsDir {
					s.navigateFiles(entry.Path, true)
				} else {
					s.previewFile(*entry)
				}
			}
			if list.Shortcut(ui.Cmd, ui.KeyC) {
				c.WriteClipboard(entry.Path)
			}
			if list.Shortcut(0, ui.KeyF2) {
				s.beginFileAction("rename", *entry)
			}
			if list.Shortcut(0, ui.KeyDelete) {
				s.beginFileAction("delete", *entry)
			}
			if list.Shortcut(0, ui.KeySpace) && !entry.IsDir {
				s.previewFile(*entry)
			}
		}
		if list.Shortcut(0, ui.KeyBackspace) && f.result.Parent != "" {
			s.navigateFiles(f.result.Parent, true)
		}
		if len(entries) == 0 && !f.loading && f.err == nil {
			ui.Text(c, text(c, "此目录为空或当前页没有匹配项")).Shrink(0).FontSize(12).TextColor(t.TextMuted)
		}
		s.fileSelectionActions(c, entries)
		if f.usageBusy {
			ui.Row(c).Shrink(0).Gap(8).Children(func() {
				ui.Text(c, text(c, "正在统计文件…")).FontSize(11).TextColor(t.TextMuted)
				iconButton(c, iconClose, "取消统计").OnClick(func() {
					if f.usageCancel != nil {
						f.usageCancel()
					}
				})
			})
		} else if !f.usage.UpdatedAt.IsZero() {
			ui.Text(c, fmt.Sprintf(text(c, "%d 个文件 · %s"), f.usage.Count, formatBytes(f.usage.Bytes))).Shrink(0).FontSize(11).TextColor(t.TextMuted)
		} else if f.usageErr != "" {
			ui.Text(c, f.usageErr).Shrink(0).FontSize(11).TextColor(t.Warning).MaxLines(2)
		}
	})
}

func (s *nativeUI) fileDirectoryUsage() {
	f := &s.filesUI
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	f.usageCancel, f.usageBusy, f.usageErr = cancel, true, ""
	generation, value := f.generation, f.result.Path
	go func() {
		defer cancel()
		result, err := s.app.DirectoryUsage(ctx, value)
		s.win.Update(func() {
			if generation != f.generation {
				return
			}
			f.usageBusy = false
			f.usageCancel = nil
			if err != nil {
				f.usageErr = err.Error()
				return
			}
			f.usage = result
		})
	}()
}

var lineIconBack = lineIcon(`<path d="m14 6-6 6 6 6"/>`)

func (s *nativeUI) fileContext(c *ui.Context, m *ui.Menu, entry core.DirectoryEntry) {
	if m.Item(text(c, "打开")).Chosen() {
		if entry.IsDir {
			s.navigateFiles(entry.Path, true)
		} else {
			s.previewFile(entry)
		}
	}
	if m.Item(text(c, "下载…")).Chosen() {
		s.downloadEntries([]core.DirectoryEntry{entry})
	}
	if m.Item(text(c, "复制到…")).Chosen() {
		s.beginFileCopy([]core.DirectoryEntry{entry})
	}
	if m.Item(text(c, "复制路径")).Chosen() {
		c.WriteClipboard(entry.Path)
	}
	if m.Item(text(c, "在文件管理器中打开")).Disabled(!filepath.IsAbs(entry.Path)).Chosen() {
		if entry.IsDir {
			if err := mygo.Shell.OpenPath(entry.Path); err != nil {
				s.setError(err)
			}
		} else {
			mygo.Shell.ShowItemInFolder(entry.Path)
		}
	}
	m.Separator()
	if m.Item(text(c, "重命名…")).Chosen() {
		s.beginFileAction("rename", entry)
	}
	if m.Item(text(c, "删除…")).Chosen() {
		s.beginFileAction("delete", entry)
	}
}

func (s *nativeUI) fileSelectionActions(c *ui.Context, entries []core.DirectoryEntry) {
	f := &s.filesUI
	selected := workspaceSelection(entries, &f.selection, f.selected)
	ui.Row(c).Shrink(0).MinHeight(30).Gap(5).Children(func() {
		label := fmt.Sprintf(text(c, "当前页显示 %d / %d 项"), len(entries), len(f.result.Entries))
		if len(selected) > 0 {
			label = fmt.Sprintf(text(c, "已选择 %d 项"), len(selected))
		}
		ui.Text(c, label).Grow(1).MinWidth(0).SingleLine().FontSize(11).Tooltip(label).TextColor(c.Theme().TextMuted)
		if f.result.Truncated {
			ui.Icon(c, iconWarning).FontSize(14).TextColor(c.Theme().Warning).Tooltip(text(c, "目录过大，请输入更具体的路径"))
		}
		actionButton(c.Key("download"), iconDownload, "下载…", false).Disabled(f.loading || len(selected) == 0).OnClick(func() { s.downloadEntries(selected) })
		iconButton(c.Key("copy-to"), browserCopyIcon, "复制到…").Disabled(f.loading || len(selected) == 0).OnClick(func() { s.beginFileCopy(selected) })
		iconButton(c.Key("rename"), iconRename, "重命名…").Disabled(f.loading || len(selected) != 1).OnClick(func() { s.beginFileAction("rename", selected[0]) })
		iconButton(c.Key("delete"), iconTrash, "删除…").Disabled(f.loading || len(selected) != 1).OnClick(func() { s.beginFileAction("delete", selected[0]) })
		if f.offset > 0 || f.result.HasMore {
			iconButton(c.Key("previous"), lineIconBack, "上一页").Disabled(f.loading || f.offset == 0).OnClick(func() { s.loadFiles(f.path, max(0, f.offset-100)) })
			iconButton(c.Key("next"), iconChevron, "下一页").Disabled(f.loading || !f.result.HasMore).OnClick(func() { s.loadFiles(f.path, f.result.NextOffset) })
		}
	})
}

func workspaceSelection(entries []core.DirectoryEntry, selection *ui.Selection[string], index int) []core.DirectoryEntry {
	var chosen []core.DirectoryEntry
	for _, entry := range entries {
		if selection.Has(entry.Path) {
			chosen = append(chosen, entry)
		}
	}
	if len(chosen) == 0 {
		if entry := selectedBrowserEntry(entries, index); entry != nil {
			chosen = append(chosen, *entry)
		}
	}
	return chosen
}

func fileBreadcrumbs(value string) (labels, paths []string) {
	if value == "" {
		return nil, nil
	}
	if filepath.IsAbs(value) {
		root := filepath.VolumeName(value) + string(filepath.Separator)
		labels, paths = []string{root}, []string{root}
		parts := strings.Split(strings.TrimPrefix(filepath.Clean(value), root), string(filepath.Separator))
		current := root
		for _, part := range parts {
			if part != "" {
				current = filepath.Join(current, part)
				labels = append(labels, part)
				paths = append(paths, current)
			}
		}
		return labels, paths
	}
	name, suffix, ok := strings.Cut(value, ":")
	if !ok {
		return nil, nil
	}
	current := name + ":"
	if strings.HasPrefix(suffix, "/") {
		current += "/"
	}
	labels, paths = []string{name + ":"}, []string{current}
	for _, part := range strings.Split(strings.Trim(suffix, "/"), "/") {
		if part != "" {
			current, _ = core.ChildPath(current, part)
			labels = append(labels, part)
			paths = append(paths, current)
		}
	}
	return labels, paths
}

func (s *nativeUI) refreshCompletedFiles(jobs []core.Job) {
	f := &s.filesUI
	if s.page != "files" || f.loading || f.err != nil || f.result.Path == "" || f.path != f.result.Path {
		return
	}
	for _, job := range jobs {
		if job.FinishedAt != nil && job.FinishedAt.After(f.refreshedAt) &&
			(job.Status == "completed" || job.Status == "failed" || job.Status == "cancelled") &&
			core.JobChangesPath(job, f.result.Path, s.mounts) {
			s.loadFiles(f.result.Path, f.offset)
			return
		}
	}
}
