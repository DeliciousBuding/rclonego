package desktop

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/egoist/mygo/ui"
)

func TestWorkflowRunSummariesPreserveFailureAndExcludeStandaloneJobs(t *testing.T) {
	jobs := []core.Job{
		{ID: "one", WorkflowID: "run", Status: "completed"},
		{ID: "two", WorkflowID: "run", Status: "failed"},
		{ID: "three", WorkflowID: "run", Status: "pending"},
		{ID: "standalone", Status: "running"},
		{ID: "unknown", WorkflowID: "future-state", Status: "waiting_input"},
	}
	summaries := workflowRunSummaries(jobs)
	if got := summaries["run"]; got.status != "failed" || got.steps != 3 || got.completed != 1 {
		t.Fatalf("incorrect summary: %+v", got)
	}
	if _, exists := summaries[""]; exists {
		t.Fatal("standalone jobs appeared in workflow summaries")
	}
	if got := summaries["future-state"]; got.status != "waiting_input" {
		t.Fatalf("unknown nonterminal state was displayed as complete: %+v", got)
	}
	if workflowRunMatches(core.Run{Name: "Archived"}, summaries["archived"], "", "已完成") {
		t.Fatal("an evicted run was treated as completed without loading its report")
	}
}

func TestWorkflowEditorClonePreservesOrderAndDoesNotStartTransfers(t *testing.T) {
	app := testApp(t)
	model := New(app, "")
	original := core.WorkflowDefinition{
		ID: "original", Name: "Example", Source: t.TempDir(), Destination: "example:archive",
		Entries: []string{"notes.txt", "documents"}, VerificationMode: "content",
		Settings: core.TransferSettings{Transfers: 1, Checkers: 2, ChunkSize: "8M", BufferSize: "4M", CPUParallelism: 1},
	}
	model.beginWorkflowEditor(original, true)
	clone, err := editedWorkflow(model.workflowUI)
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID != "" || clone.Name == original.Name || !strings.HasPrefix(clone.Name, original.Name+" ") || !reflect.DeepEqual(clone.Entries, original.Entries) ||
		clone.Settings != original.Settings || clone.VerificationMode != original.VerificationMode {
		t.Fatalf("clone lost definition data: %+v", clone)
	}
	saved, err := app.SaveWorkflow(clone)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.ID == original.ID || len(app.Snapshot().Runs) != 0 || len(app.Snapshot().Jobs) != 0 {
		t.Fatal("saving a clone started a transfer or retained the original identity")
	}
	model.beginWorkflowEditor(saved, false)
	model.workflowUI.name = "Edited"
	edited, err := editedWorkflow(model.workflowUI)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.SaveWorkflow(edited); err != nil {
		t.Fatal(err)
	}
	if definitions := app.Snapshot().Workflows; len(definitions) != 1 || definitions[0].Name != "Edited" || definitions[0].ID != saved.ID {
		t.Fatalf("editing created another definition instead of updating: %+v", definitions)
	}
}

func TestWorkflowEditorRejectsInvalidOrRepeatedTopLevelSteps(t *testing.T) {
	model := New(testApp(t), "")
	model.beginWorkflowEditor(core.WorkflowDefinition{Name: "Example", Source: "/example", Destination: "example:archive"}, false)
	for _, entries := range []string{"../outside", "sub/folder", "sub\\folder", "same\nsame", ".", ".."} {
		model.workflowUI.entries = entries
		if _, err := editedWorkflow(model.workflowUI); err == nil {
			t.Fatalf("invalid steps accepted: %q", entries)
		}
	}
}

func TestPortableWorkflowRoundTripClearsIdentityAndBoundsInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.json")
	original := core.WorkflowDefinition{
		ID: "source-installation-id", CreatedAt: time.Now(), Name: "Example",
		Source: "/example/source", Destination: "example:archive", Entries: []string{"b", "a"},
		VerificationMode: "size",
		Settings:         core.TransferSettings{Transfers: 2, Checkers: 4, ChunkSize: "16M", BufferSize: "8M", CPUParallelism: 2},
	}
	if err := writePortableWorkflow(path, original); err != nil {
		t.Fatal(err)
	}
	imported, err := readPortableWorkflow(path)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID != "" || !imported.CreatedAt.IsZero() || !reflect.DeepEqual(imported.Entries, original.Entries) ||
		imported.Settings != original.Settings || imported.Destination != original.Destination {
		t.Fatalf("portable definition was changed: %+v", imported)
	}
	if err = writePortableWorkflow(path, imported); err != nil {
		t.Fatalf("native save overwrite failed: %v", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("export left temporary files: %v / %v", files, err)
	}
	if err = os.WriteFile(path, []byte(strings.Repeat(" ", maxWorkflowImportBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = readPortableWorkflow(path); err == nil {
		t.Fatal("oversized import accepted")
	}
}

func TestWorkflowHistoryPageIsNewestFirstAndBounded(t *testing.T) {
	var runs []core.Run
	for i := 0; i < 25; i++ {
		runs = append(runs, core.Run{ID: itoa(i), CreatedAt: time.Unix(int64(i), 0)})
	}
	rows, hasNext := recentWorkflowRuns(runs)
	if !hasNext || len(rows) != workflowHistoryPageSize || rows[0].ID != "24" || rows[19].ID != "5" || runs[0].ID != "0" {
		t.Fatalf("unexpected page or original order mutated: %+v", rows)
	}
}

func TestWorkflowReportVirtualizesLongStepHistory(t *testing.T) {
	model := New(testApp(t), "")
	report := &core.RunReport{Run: core.Run{ID: "example", Name: "Example"}, Status: "completed", Verification: "not_run"}
	for i := 0; i < 500; i++ {
		report.Steps = append(report.Steps, core.Job{
			ID: itoa(i), Name: "Example step " + itoa(i), WorkflowStep: i + 1, Status: "completed", Operation: "copy",
		})
	}
	model.workflowUI.report = report
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { model.workflowReport(c) })
	}, 680, 600)
	tester.SetLocale("zh-Hans")
	tester.Frame()
	visible := 0
	for _, text := range tester.Texts() {
		if strings.HasPrefix(text, "Example step ") {
			visible++
		}
	}
	// List may measure extra rows on its first layout to learn variable row
	// heights. That work must still stay far below the complete history.
	if visible == 0 || visible > 100 {
		t.Fatalf("report built %d rows for a short viewport instead of virtualizing 500 steps", visible)
	}
	if tester.HasText("已下载并比较文件内容") {
		t.Fatal("report claimed a content comparison without verification evidence")
	}
}

func TestWorkflowEditorActionsStayInsideNarrowAndDarkWindows(t *testing.T) {
	for _, width := range []int{900, 1120, 1440} {
		for _, appearance := range []string{"light", "dark"} {
			model := New(testApp(t), "")
			model.page, model.appearance = "workflows", appearance
			model.workflows = []core.WorkflowDefinition{{
				ID: "example", Name: strings.Repeat("Long example name ", 10),
				Source: "/example/" + strings.Repeat("long-directory/", 30), Destination: "example:archive",
			}}
			tester := ui.NewTester(model.view, width, 740)
			tester.SetLocale("zh-Hans")
			for _, label := range []string{"搜索工作流", "导入工作流", "新建工作流…", "编辑工作流", "更多工作流操作"} {
				rect, ok := tester.Find(label)
				if !ok || rect.X < 208 || rect.X+rect.W > float32(width-28) {
					t.Fatalf("%d/%s: workflow action outside canvas: %s %+v", width, appearance, label, rect)
				}
			}
			if err := tester.Click("编辑工作流"); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{"工作流源目录", "工作流目标目录", "工作流步骤顺序", "保存工作流"} {
				rect, ok := tester.Find(label)
				if !ok || rect.X < 0 || rect.X+rect.W > float32(width) || rect.Y < 0 || rect.Y+rect.H > 740 {
					t.Fatalf("%d/%s: workflow editor clipped: %s %+v", width, appearance, label, rect)
				}
			}
			tester.Key(0, ui.KeyEscape)
			if model.workflowUI.editorOpen {
				t.Fatal("official dialog did not close with Escape")
			}
			if err := tester.Click("更多工作流操作"); err != nil {
				t.Fatal(err)
			}
			if err := tester.ChooseMenuItem("复制工作流"); err != nil {
				t.Fatal(err)
			}
			if !model.workflowUI.editorOpen || model.workflowUI.editorID != "" || len(model.app.Snapshot().Jobs) != 0 {
				t.Fatal("Clone menu did not open a new definition without starting work")
			}
		}
	}
}

func TestWorkflowEditorExpandedOptionsKeepPrimaryActionVisibleAtMinimumHeight(t *testing.T) {
	model := New(testApp(t), "")
	model.page = "workflows"
	model.beginWorkflowEditor(core.WorkflowDefinition{Name: "Example", Source: "/example", Destination: "example:archive"}, false)
	model.workflowUI.advanced = true
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("zh-Hans")
	for _, label := range []string{"保存工作流", "取消"} {
		rect, ok := tester.Find(label)
		if !ok || rect.X < 0 || rect.X+rect.W > 900 || rect.Y < 0 || rect.Y+rect.H > 560 {
			t.Fatalf("expanded workflow editor placed action outside minimum window: %s %+v", label, rect)
		}
	}
}
