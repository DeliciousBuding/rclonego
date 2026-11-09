package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestFileBatchKeepsSuccessfulItemsAndRetriesOnlyMissingOnes(t *testing.T) {
	app := testApp(t)
	source, destination := t.TempDir(), t.TempDir()
	paths := []string{filepath.Join(source, "first.txt"), filepath.Join(source, "missing.txt"), filepath.Join(source, "last.txt")}
	for _, path := range []string{paths[0], paths[2]} {
		if err := os.WriteFile(path, []byte("sample"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	batch := enqueueFileOperation(app, fileOperationState{items: uploadEntries(paths), destination: destination})
	if queued, failed := batch.counts(); queued != 2 || failed != 1 {
		t.Fatalf("partial result: queued=%d failed=%d", queued, failed)
	}
	if !errors.Is(batch.items[1].err, os.ErrNotExist) || len(app.Snapshot().Jobs) != 2 {
		t.Fatal("missing middle item hid success or stopped later items")
	}
	firstID, lastID := batch.items[0].jobID, batch.items[2].jobID
	if err := os.WriteFile(paths[1], []byte("sample"), 0600); err != nil {
		t.Fatal(err)
	}
	batch = enqueueFileOperation(app, batch)
	if queued, failed := batch.counts(); queued != 3 || failed != 0 {
		t.Fatalf("retry result: queued=%d failed=%d", queued, failed)
	}
	if batch.items[0].jobID != firstID || batch.items[2].jobID != lastID || len(app.Snapshot().Jobs) != 3 {
		t.Fatal("retry duplicated previously queued files")
	}
	// A repeated submission with the same item key is also harmless if the
	// caller loses its successful result.
	batch.items[1].jobID = ""
	batch = enqueueFileOperation(app, batch)
	if len(app.Snapshot().Jobs) != 3 || batch.items[1].jobID == "" {
		t.Fatal("lost result created another job")
	}
}

func TestFileCopyBatchReportsInvalidNamesAndKeepsOriginalSelection(t *testing.T) {
	app := testApp(t)
	entries := []core.DirectoryEntry{
		{Name: "first.txt", Path: "example:first.txt"},
		{Name: "../invalid", Path: "example:invalid"},
		{Name: "last.txt", Path: "example:last.txt"},
	}
	operation := fileOperationState{destination: t.TempDir()}
	for _, entry := range entries {
		operation.items = append(operation.items, fileOperationItem{entry: entry})
	}
	result := enqueueFileOperation(app, operation)
	if queued, failed := result.counts(); queued != 2 || failed != 1 {
		t.Fatalf("copy partial result: queued=%d failed=%d", queued, failed)
	}
	if operation.items[0].jobID != "" || operation.items[1].err != nil {
		t.Fatal("worker changed the active UI selection")
	}
	if len(app.Snapshot().Jobs) != 2 {
		t.Fatal("invalid child name reached the queue")
	}
}

func TestFileOperationLinksItsTasksAndPreservesFilePosition(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := fileModel(t)
		model.filesUI.query, model.filesUI.offset = "a", 100
		model.filesUI.operation = &fileOperationState{items: []fileOperationItem{
			{entry: core.DirectoryEntry{Name: "a.txt"}, jobID: "file-job"},
			{entry: core.DirectoryEntry{Name: "missing.txt"}, err: os.ErrNotExist},
		}}
		model.jobs = []core.Job{
			{ID: "file-job", Name: "a.txt", Status: "pending"},
			{ID: "unrelated", Name: "Unrelated task", Status: "pending"},
		}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if language == "en" {
			assertNoChineseInterfaceText(t, "file result", tester.Texts())
		}
		for _, label := range []string{"查看本次任务", "重试未加入项", "上传文件…", "下一页"} {
			bounds, ok := tester.Find(locales.TextIn(language, label))
			if !ok || bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.W > 900 || bounds.Y+bounds.H > 560 {
				t.Fatalf("%s result action clipped: %s %+v", language, label, bounds)
			}
		}
		if err := tester.Click(locales.TextIn(language, "未加入的项目")); err != nil {
			t.Fatal(err)
		}
		if !model.filesUI.operation.detailsOpen || !tester.HasText("missing.txt") {
			t.Fatal("failed items cannot be inspected")
		}
		tester.Key(0, ui.KeyEscape)
		if err := tester.Click(locales.TextIn(language, "查看本次任务")); err != nil {
			t.Fatal(err)
		}
		if model.page != "queue" || !tester.HasText("a.txt") || tester.HasText("Unrelated task") {
			t.Fatal("file result did not show only its own tasks")
		}
		model.loadLog("file-job")
		tester.Frame()
		if !tester.HasText(locales.TextIn(language, "任务详情")) {
			t.Fatal("a task detail could not be opened from the result")
		}
		if err := tester.Click(locales.TextIn(language, "返回文件")); err != nil {
			t.Fatal(err)
		}
		if model.page != "files" || model.filesUI.query != "a" || model.filesUI.offset != 100 || model.filesUI.result.Path != "fixture:" {
			t.Fatal("return to files lost the current location")
		}
	}
}

func TestFileReadErrorsGiveActionsWithoutExposingDetailsByDefault(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := fileModel(t)
		model.filesUI.err = context.DeadlineExceeded.Error()
		model.filesUI.readIssue = context.DeadlineExceeded
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		if !tester.HasText(locales.TextIn(language, "读取超时")) || tester.HasText(context.DeadlineExceeded.Error()) {
			t.Fatal("timeout did not separate explanation and technical details")
		}
		if err := tester.Click(locales.TextIn(language, "错误详情")); err != nil {
			t.Fatal(err)
		}
		if !tester.HasText(context.DeadlineExceeded.Error()) {
			t.Fatal("technical details are unavailable")
		}
		model.filesUI.readIssue = errors.New("directory listing failed; check the path and account authorization")
		tester.Frame()
		if err := tester.Click(locales.TextIn(language, "检查账户")); err != nil || model.page != "remotes" {
			t.Fatalf("account action: %v", err)
		}
	}
}

func TestFileWorkspaceDensityLeavesEightRowsVisibleWithAResult(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		model := fileModel(t)
		model.filesUI.result.Entries = nil
		for i := range 20 {
			name := fmt.Sprintf("sample-%02d.txt", i)
			model.filesUI.result.Entries = append(model.filesUI.result.Entries, core.DirectoryEntry{
				Name: name, Path: "fixture:" + name, Size: 1024,
			})
		}
		model.filesUI.operation = &fileOperationState{items: []fileOperationItem{
			{entry: core.DirectoryEntry{Name: "sample-00.txt"}, jobID: "first"},
			{entry: core.DirectoryEntry{Name: "missing.txt"}, err: os.ErrNotExist},
		}}
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale(language)
		visible := 0
		for i := range 20 {
			if bounds, ok := tester.Find(fmt.Sprintf("sample-%02d.txt", i)); ok && bounds.H > 0 && bounds.Y >= 0 && bounds.Y+bounds.H <= 510 {
				visible++
			}
		}
		if visible < 8 {
			t.Fatalf("%s shows only %d rows in the minimum viewport", language, visible)
		}
		tester.Key(ui.Cmd, ui.KeyL)
		if !model.filesUI.editPath {
			t.Fatal("compact navigation cannot edit the path with Ctrl/Cmd+L")
		}
		if _, found := tester.Find(locales.TextIn(language, "文件路径")); !found {
			t.Fatal("path editing field is unavailable")
		}
		model.filesUI.pathDraft = "different:folder"
		tester.Key(0, ui.KeyEscape)
		if model.filesUI.path != "fixture:" || model.filesUI.editPath {
			t.Fatal("cancelled path editing changed the current directory")
		}
	}
}

func TestUnavailablePreviewOffersDownloadWithoutCreatingATask(t *testing.T) {
	model := fileModel(t)
	model.filesUI.previewOpen = true
	model.filesUI.preview = core.FilePreview{Path: "fixture:archive.zip"}
	model.filesUI.previewEntry = core.DirectoryEntry{Name: "archive.zip", Path: "fixture:archive.zip", Size: 100}
	model.filesUI.previewIssue = errors.New("this file has no text or image preview; download it to open in another app")
	model.filesUI.previewError = model.filesUI.previewIssue.Error()
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	assertNoChineseInterfaceText(t, "file task table", tester.Texts())
	if !tester.HasText("This file cannot be previewed in the app") {
		t.Fatal("unsupported preview has no explanation")
	}
	if _, ok := tester.Find("Download…"); !ok {
		t.Fatal("unsupported preview has no download action")
	}
	tester.Key(0, ui.KeyEscape)
	if len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("preview or dismissal submitted a transfer")
	}
}

func TestFileTaskTableUsesCurrentQueueDataAndVirtualizesRows(t *testing.T) {
	model := fileModel(t)
	operation := &fileOperationState{}
	for i := range 500 {
		id := fmt.Sprintf("file-%03d", i)
		model.jobs = append(model.jobs, core.Job{
			ID: id, Name: id, Destination: "sample:archive/" + id,
			Status: "completed", Bytes: 12, TotalBytes: 12,
		})
		operation.items = append(operation.items, fileOperationItem{jobID: id})
	}
	model.jobs = append(model.jobs, core.Job{ID: "other", Name: "Unrelated task"})
	model.filesUI.operation = operation
	model.showFileOperationTasks()
	selected := fileOperationJobs(model.jobs, operation.items)
	if len(selected) != 500 || selected[0].ID != "file-000" || selected[499].ID != "file-499" {
		t.Fatal("file task filter lost queue data or included an unrelated task")
	}
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if !tester.HasText("file-000") || tester.HasText("file-499") || tester.HasText("Unrelated task") {
		t.Fatal("file tasks are not limited to visible table rows")
	}
	if !tester.HasText("sample:archive/file-000") || !tester.HasText("100%") {
		t.Fatal("file task table lost destination or progress")
	}
}

func BenchmarkFileOperationJobs(b *testing.B) {
	jobs := make([]core.Job, 500)
	items := make([]fileOperationItem, 100)
	for i := range jobs {
		jobs[i].ID = fmt.Sprintf("job-%d", i)
	}
	for i := range items {
		items[i].jobID = jobs[i*5].ID
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = fileOperationJobs(jobs, items)
	}
}
