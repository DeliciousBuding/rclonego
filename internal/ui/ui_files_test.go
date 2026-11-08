package desktop

import (
	"slices"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func fileModel(t *testing.T) *nativeUI {
	model := New(testApp(t), "")
	model.page = "files"
	model.filesUI = fileWorkspaceState{browserUIState: browserUIState{open: true, path: "fixture:", selected: -1,
		result: core.BrowseResult{Path: "fixture:", Entries: []core.DirectoryEntry{
			{Name: "Documents", Path: "fixture:Documents", IsDir: true},
			{Name: "a.txt", Path: "fixture:a.txt", Size: 12},
			{Name: "b.txt", Path: "fixture:b.txt", Size: 24},
		}}}}
	return model
}

func TestFileWorkspaceSelectionAndActionsFitMinimumWindow(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := fileModel(t)
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if language == "en" {
			assertNoChineseInterfaceText(t, "files", tester.Texts())
		}
		if err := tester.Click("a.txt"); err != nil {
			t.Fatal(err)
		}
		tester.Key(ui.Cmd, ui.KeyC)
		if tester.Clipboard() != "fixture:a.txt" {
			t.Fatal("file keyboard copy failed")
		}
		tester.Key(0, ui.KeyF2)
		if !model.filesUI.actionOpen || model.filesUI.action != "rename" {
			t.Fatal("F2 did not open selected file rename")
		}
		tester.Key(0, ui.KeyEscape)
		if model.filesUI.actionOpen {
			t.Fatal("Escape did not cancel rename")
		}
		if len(model.app.Snapshot().Jobs) != 0 {
			t.Fatal("cancelled file edit created a job")
		}
		for _, label := range []string{"上传文件…", "新建文件夹", "下载…", "上一页", "下一页"} {
			rect, ok := tester.Find(locales.TextIn(language, label))
			if !ok || rect.X < 208 || rect.Y < 0 || rect.X+rect.W > 900 || rect.Y+rect.H > 560 {
				t.Fatalf("%s file action outside minimum window: %s %+v", language, label, rect)
			}
		}
	}
}

func TestFilePreviewAndDeleteConfirmationStaySeparate(t *testing.T) {
	model := fileModel(t)
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	tester.Click("a.txt")
	tester.Key(0, ui.KeyDelete)
	if !model.filesUI.actionOpen || model.filesUI.action != "delete" || !tester.HasText("将永久删除此文件或空文件夹，不会移入回收站。") {
		t.Fatal("delete did not expose explicit confirmation")
	}
	tester.Key(0, ui.KeyEscape)
	if len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("delete cancellation mutated storage")
	}
	model.filesUI.previewOpen = true
	model.filesUI.preview = core.FilePreview{Path: "fixture:a.txt", Kind: "text", Text: "sample content"}
	tester.Frame()
	if !tester.HasText("sample content") {
		t.Fatal("text preview content missing")
	}
	tester.Key(0, ui.KeyEscape)
	if model.filesUI.previewOpen {
		t.Fatal("preview did not dismiss")
	}
}

func TestMountEditorFooterStaysVisibleWithAdvancedFields(t *testing.T) {
	model := New(testApp(t), "")
	model.beginMountEdit(core.MountConfig{})
	model.showMountAdvanced = true
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	rect, ok := tester.Find("保存")
	if !ok || rect.Y+rect.H > 560 {
		t.Fatalf("mount save outside minimum window: %+v", rect)
	}
	if model.mountCache == "" || model.mountDrive == "" {
		t.Fatal("new mount does not have useful defaults")
	}
}

func TestFileBreadcrumbsPreserveRemoteRootAndSelection(t *testing.T) {
	labels, paths := fileBreadcrumbs("fixture:/Documents/Reports")
	if !slices.Equal(labels, []string{"fixture:", "Documents", "Reports"}) ||
		!slices.Equal(paths, []string{"fixture:/", "fixture:/Documents", "fixture:/Documents/Reports"}) {
		t.Fatalf("remote breadcrumbs changed its absolute root: %v / %v", labels, paths)
	}
	model := fileModel(t)
	model.filesUI.result.Path = "fixture:Documents/Reports"
	model.filesUI.path = model.filesUI.result.Path
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if err := tester.Click("Documents"); err != nil {
		t.Fatal(err)
	}
	if model.filesUI.path != "fixture:Documents" {
		t.Fatal("breadcrumb did not navigate to parent")
	}
}
