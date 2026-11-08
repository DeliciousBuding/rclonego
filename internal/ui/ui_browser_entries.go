package desktop

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

func browserEntries(page []core.DirectoryEntry, query, order string) []core.DirectoryEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	entries := make([]core.DirectoryEntry, 0, len(page))
	for _, entry := range page {
		if query == "" || strings.Contains(strings.ToLower(entry.Name), query) {
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.IsDir != right.IsDir {
			return left.IsDir
		}
		if order == "大小由大到小" && !left.IsDir && left.Size != right.Size {
			return left.Size > right.Size
		}
		if order == "修改时间由新到旧" {
			if left.ModTime == nil && right.ModTime != nil {
				return false
			}
			if left.ModTime != nil && right.ModTime == nil {
				return true
			}
			if left.ModTime != nil && right.ModTime != nil && !left.ModTime.Equal(*right.ModTime) {
				return left.ModTime.After(*right.ModTime)
			}
		}
		a, b := strings.ToLower(left.Name), strings.ToLower(right.Name)
		if a == b {
			a, b = left.Name, right.Name
		}
		if order == "名称 Z–A" {
			return a > b
		}
		return a < b
	})
	return entries
}

func selectedBrowserEntry(entries []core.DirectoryEntry, selected int) *core.DirectoryEntry {
	if selected < 0 || selected >= len(entries) {
		return nil
	}
	return &entries[selected]
}

var browserCopyIcon = lineIcon(`<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V4a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h4"/>`)

func (s *nativeUI) browserSelection(c *ui.Context, entries []core.DirectoryEntry) {
	b, t := &s.browserUI, c.Theme()
	selected := selectedBrowserEntry(entries, b.selected)
	ui.Row(c).Shrink(0).MinHeight(36).Gap(8).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			if selected == nil {
				ui.Text(c, fmt.Sprintf(text(c, "当前页显示 %d / %d 项"), len(entries), len(b.result.Entries))).
					FontSize(11).TextColor(t.TextMuted)
				return
			}
			ui.Text(c, selected.Path).FontSize(11).SingleLine().TextColor(t.TextMuted).
				Tooltip(selected.Path).Selectable()
			detail := text(c, "目录")
			if !selected.IsDir {
				detail = text(c, "文件") + " · " + formatBytes(selected.Size)
			}
			if selected.ModTime != nil {
				detail += " · " + dateTime(c, *selected.ModTime)
			}
			ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		copyPath := b.result.Path
		if selected != nil {
			copyPath = selected.Path
		}
		iconButton(c.Key("copy-path"), browserCopyIcon, "复制路径").Disabled(b.loading || b.err != "" || copyPath == "").
			OnClick(func() { c.WriteClipboard(copyPath) })
	})
}
