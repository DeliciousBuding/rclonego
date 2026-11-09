package desktop

import (
	"sort"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
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
