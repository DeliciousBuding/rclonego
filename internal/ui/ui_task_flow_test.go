package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/locales"
	"github.com/egoist/mygo/ui"
)

func TestTaskFooterStaysVisibleWithAdvancedOptionsAndPreview(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, operation := range []string{"copy", "sync"} {
			model := New(testApp(t), "")
			model.beginJobForm(nil)
			model.jobName, model.jobSource, model.jobDestination = "Example copy", "/synthetic/source", "example:archive"
			model.showJobAdvanced, model.jobOperation, model.jobAllowDelete = true, operation, true
			model.jobOperationChoice = operationLabel(operation)
			model.jobPreview = core.Preview{ID: "synthetic", ExpiresAt: time.Now().Add(time.Hour)}
			model.jobPreviewKey = model.currentPreviewKey()
			for i := range 200 {
				model.jobPreview.Entries = append(model.jobPreview.Entries, core.PreviewEntry{Action: "added", Path: "Example file " + itoa(i)})
			}
			tester := ui.NewTester(model.view, 900, 560)
			tester.SetLocale(language)
			action := "加入队列"
			if operation == "sync" {
				action = "确认同步"
			}
			for _, label := range []string{action, "取消", "预览更改"} {
				label = locales.TextIn(language, label)
				before, found := tester.Find(label)
				if !found || before.X < 0 || before.X+before.W > 900 || before.Y < 0 || before.Y+before.H > 560 {
					t.Fatalf("%s/%s footer action clipped: %s %+v", language, operation, label, before)
				}
				tester.Scroll(460, 280, 0, -4)
				after, found := tester.Find(label)
				if !found || before.Y != after.Y {
					t.Fatalf("%s/%s action scrolled with the form: %s %+v / %+v", language, operation, label, before, after)
				}
			}
		}
	}
}

func TestTaskRequiredIssuesAppearAtTheirFieldsAndClearAfterEditing(t *testing.T) {
	model := New(testApp(t), "")
	model.beginJobForm(nil)
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if tester.HasText("Enter a task name") {
		t.Fatal("Untouched fields show an error immediately")
	}
	if err := tester.Click("Add to queue"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("Enter a task name") || model.isError {
		t.Fatal("Missing name was not explained within the form")
	}
	model.jobName = "Example"
	tester.Frame()
	if tester.HasText("Enter a task name") || !tester.HasText("Choose a source folder") {
		t.Fatal("Required-field issue did not follow the current input")
	}
	model.jobSource, model.jobDestination = "/synthetic/source", "/synthetic/source"
	tester.Frame()
	if !tester.HasText("Source and destination must be different") {
		t.Fatal("Same-location issue was not shown at the destination")
	}
	model.jobDestination = "example:archive"
	tester.Frame()
	if tester.HasText("Source and destination must be different") {
		t.Fatal("Correcting the destination retained the old error")
	}
	model.jobBandwidth = "off"
	if _, issue := model.editedJob(); issue != nil {
		t.Fatalf("Explicit unlimited bandwidth rejected: %+v", issue)
	}
	if len(model.app.Snapshot().Jobs) != 0 {
		t.Fatal("Input validation created a task")
	}
}

func TestSyncFooterRequiresPreviewAndExplicitDeletionPermission(t *testing.T) {
	model := New(testApp(t), "")
	model.beginJobForm(nil)
	model.jobOperation, model.jobOperationChoice = "sync", "同步"
	tester := ui.NewTester(model.view, 900, 560)
	tester.SetLocale("en")
	if _, found := tester.Find("Preview changes"); !found {
		t.Fatal("Sync did not start with a preview action")
	}
	model.jobPreview = core.Preview{ID: "synthetic", ExpiresAt: time.Now().Add(time.Hour)}
	model.jobPreviewKey = model.currentPreviewKey()
	tester.Frame()
	if control, found := tester.Access("Confirm sync"); !found || !control.Disabled {
		t.Fatal("Sync confirmation was enabled without deletion permission")
	}
	model.jobAllowDelete, model.jobPreview.ReportIncomplete = true, true
	tester.Frame()
	if control, found := tester.Access("Confirm sync"); !found || !control.Disabled {
		t.Fatal("Incomplete preview evidence enabled sync confirmation")
	}
	model.beginJobForm(&core.Job{
		ID: "synthetic", Name: "Example", Operation: "sync", Source: "/synthetic/source",
		Destination: "example:archive", Transfers: 2, Checkers: 4, ChunkSize: "16M", BufferSize: "8M",
	})
	tester.Frame()
	if control, found := tester.Access("Save parameters"); !found || control.Disabled {
		t.Fatal("Editing queued sync parameters unexpectedly required another preview")
	}
}

func TestReportKeepsVerificationEvidenceAndRecoveryVisible(t *testing.T) {
	for _, verification := range []string{"content_compared", "not_run", "content"} {
		model := New(testApp(t), "")
		report := &core.RunReport{
			Run:    core.Run{ID: "synthetic", Name: "Example workflow"},
			Status: "failed", Verification: verification,
			Scope: "one-way: selected top-level workflow entries and their descendants",
			Steps: []core.Job{
				{ID: "copy", Name: "Documents", Operation: "copy", Status: "completed", WorkflowStep: 1},
				{ID: "check", Name: "Final check", Operation: "check", Status: "failed", WorkflowStep: 2,
					Source: "/synthetic/source", Destination: "example:archive", Error: "Example destination is unavailable"},
			},
		}
		model.workflowUI.report = report
		tester := ui.NewTester(model.view, 900, 560)
		tester.SetLocale("en")
		assertNoChineseInterfaceText(t, "report", tester.Texts())
		if !tester.HasText("1 / 2 steps completed") || !tester.HasText("Example destination is unavailable") {
			t.Fatal("The result did not explain progress and the failed step")
		}
		if tester.HasText("Downloaded and compared file contents") != (verification == "content_compared") {
			t.Fatal("Requested verification was treated as evidence of a completed comparison")
		}
		if !strings.Contains(strings.Join(tester.Texts(), "\n"), "selected top-level") {
			t.Fatal("The report expanded its selected scope")
		}
		bounds, found := tester.Find("Resume incomplete steps")
		if !found || bounds.Y < 0 || bounds.Y+bounds.H > 560 {
			t.Fatalf("Recovery action is outside the minimum viewport: %+v", bounds)
		}
		tester.Key(0, ui.KeyEscape)
		if model.workflowUI.report != nil {
			t.Fatal("Escape did not close the result dialog")
		}
	}
}
