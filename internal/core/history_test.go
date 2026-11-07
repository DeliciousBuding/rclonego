package core

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestOldRunReportAndIdempotencySurviveBoundedStartupHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, "", dir)
	definition, err := app.SaveWorkflow(WorkflowDefinition{ID: "synthetic-definition", Name: "Synthetic history",
		Source: filepath.Join(dir, "source"), Destination: filepath.Join(dir, "destination")})
	if err != nil {
		t.Fatal(err)
	}
	err = store.Update(func(state *State) error {
		for i := 0; i < 205; i++ {
			id := fmt.Sprintf("synthetic-run-%03d", i)
			state.Runs = append(state.Runs, Run{ID: id, WorkflowID: definition.ID, Name: "Synthetic run", CreatedAt: time.Now().UTC()})
			state.Jobs = append(state.Jobs, Job{ID: id + "-step", WorkflowID: id, Status: "completed", Operation: "check", VerificationMode: "content"})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app = NewApp(store, "", dir)
	if len(app.Snapshot().Runs) != 200 {
		t.Fatalf("startup history not bounded: %d", len(app.Snapshot().Runs))
	}
	report, err := app.ReportRun("synthetic-run-000")
	if err != nil || len(report.Steps) != 1 {
		t.Fatalf("older report unavailable: %+v %v", report, err)
	}
	prior, err := app.StartWorkflow(definition.ID, "synthetic-run-000")
	if err != nil || prior.ID != "synthetic-run-000" {
		t.Fatalf("old idempotent key started another run: %+v %v", prior, err)
	}
	page, err := app.ListRuns(0, 1)
	if err != nil || len(page) != 1 || page[0].ID != "synthetic-run-204" {
		t.Fatalf("pagination order: %+v %v", page, err)
	}
}
