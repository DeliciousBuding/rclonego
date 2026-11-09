package desktop

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestBrowserSearchSortKeepsFoldersFirstAndUnknownSizesLast(t *testing.T) {
	page := []core.DirectoryEntry{
		{Name: "unknown.txt", Path: "storage:unknown.txt", Size: -1},
		{Name: "beta.txt", Path: "storage:beta.txt", Size: 25},
		{Name: "Folder", Path: "storage:Folder", IsDir: true, Size: -1},
		{Name: "Alpha.txt", Path: "storage:Alpha.txt", Size: 2},
	}
	names := func(entries []core.DirectoryEntry) []string {
		result := make([]string, len(entries))
		for i, entry := range entries {
			result[i] = entry.Name
		}
		return result
	}
	if got := names(browserEntries(page, "", "大小由大到小")); !slices.Equal(got, []string{"Folder", "beta.txt", "Alpha.txt", "unknown.txt"}) {
		t.Fatalf("size order: %v", got)
	}
	if got := names(browserEntries(page, " TXT ", "名称 Z–A")); !slices.Equal(got, []string{"unknown.txt", "beta.txt", "Alpha.txt"}) {
		t.Fatalf("filtered name order: %v", got)
	}
	if page[0].Name != "unknown.txt" {
		t.Fatal("sorting modified the service's page")
	}
}

func TestDirectoryPickerKeyboardSelectionCopiesPathWithoutStartingATask(t *testing.T) {
	model := New(testApp(t), "")
	target := "storage:"
	model.browserUI = browserUIState{open: true, target: &target, directoryUIState: directoryUIState{path: "storage:", selected: -1,
		result: core.BrowseResult{Path: "storage:", Entries: []core.DirectoryEntry{
			{Name: "Alpha", Path: "storage:Alpha", IsDir: true},
			{Name: "Beta", Path: "storage:Beta", IsDir: true},
		}}}}
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	if err := tester.Click("Alpha"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, ui.KeyDown)
	if model.browserUI.selected != 1 {
		t.Fatal("native Down selection did not select the next directory")
	}
	tester.Key(ui.Cmd, ui.KeyC)
	if tester.Clipboard() != "storage:Beta" {
		t.Fatalf("keyboard path copy: %q", tester.Clipboard())
	}
	if err := tester.Click("选择此目录"); err != nil {
		t.Fatal(err)
	}
	if target != "storage:" || model.browserUI.open || len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("choosing the current directory used its highlighted child or submitted a task")
	}
}

func TestDirectoryPickerControlsFitMinimumWindowWithLongNames(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := New(testApp(t), "")
		long := "a-" + strings.Repeat("long-name-", 12)
		target := "storage:"
		model.browserUI = browserUIState{open: true, target: &target, directoryUIState: directoryUIState{path: "storage:", selected: 0,
			result: core.BrowseResult{Path: "storage:", Entries: []core.DirectoryEntry{
				{Name: long, Path: "storage:" + long, IsDir: true},
			}, HasMore: true},
		}}
		for i := range 20 {
			name := fmt.Sprintf("folder-%02d", i)
			model.browserUI.result.Entries = append(model.browserUI.result.Entries, core.DirectoryEntry{Name: name, Path: "storage:" + name, IsDir: true})
		}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if language == "en" {
			assertNoChineseInterfaceText(t, "file browser", tester.Texts())
		}
		for _, source := range []string{"选择此目录", "关闭目录选择", "上一页", "下一页"} {
			label := locales.TextIn(language, source)
			bounds, ok := tester.Find(label)
			if !ok || bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.W > 900 || bounds.Y+bounds.H > 560 {
				t.Fatalf("%s %q is outside minimum viewport: %+v", language, label, bounds)
			}
		}
		footer, _ := tester.Find(locales.TextIn(language, "选择此目录"))
		visible := 0
		for i := range 20 {
			if bounds, ok := tester.Find(fmt.Sprintf("folder-%02d", i)); ok && bounds.H > 0 && bounds.Y+bounds.H <= footer.Y {
				visible++
			}
		}
		if visible < 7 {
			t.Fatalf("%s directory picker shows only %d complete folder rows", language, visible)
		}
	}
}

func TestStorageCapabilitiesUseDisclosureWithoutHidingHashes(t *testing.T) {
	model := New(testApp(t), "")
	model.page, model.storageDetail, model.showStorageDetails = "remotes", "storage", true
	model.remotes = []core.RemoteInfo{{Name: "storage", Type: "drive"}}
	model.storageUI["storage"] = &storageUIState{
		info:  core.StorageInfo{Name: "storage", Hashes: []string{"md5", "sha256"}, UpdatedAt: time.Now()},
		quota: core.RemoteQuota{Name: "storage", Status: "unsupported"},
	}
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	if !tester.HasText("校验算法 · md5, sha256") || tester.HasText("服务端复制") {
		t.Fatal("hashes or folded capability hierarchy is wrong")
	}
	if err := tester.Click("后端能力"); err != nil {
		t.Fatal(err)
	}
	if !model.storageUI["storage"].detailsOpen || !tester.HasText("服务端复制") {
		t.Fatal("capability disclosure did not open")
	}
}
