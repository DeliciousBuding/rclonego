package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPartialWorkflowChecksOnlySelectedLiteralEntries(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	for _, name := range []string{"selected [x]", "selected x", "omitted"} {
		if err := os.MkdirAll(filepath.Join(source, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name, "sample.txt"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"notes [x].txt", "notes x.txt", "omitted.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stateDir := filepath.Join(dir, "state")
	store, err := OpenStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, stateDir)
	jobs, err := app.AddWorkflow(WorkflowRequest{Name: "Partial", Source: source, Destination: destination, IdempotencyKey: "partial",
		Entries: []string{"selected [x]", "notes [x].txt"}, VerificationMode: "content"})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || len(jobs[2].Includes) != 2 {
		t.Fatalf("unexpected verification scope: %+v", jobs)
	}
	app.Start()
	defer app.Close()
	var report RunReport
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		report, err = app.ReportRun("partial")
		if err != nil {
			t.Fatal(err)
		}
		if report.Status == "completed" || report.Status == "failed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if report.Status != "completed" || report.Verification != "content_compared" || !strings.Contains(report.Scope, "selected") {
		t.Fatalf("partial workflow checked unselected files: %+v", report)
	}
	for _, name := range []string{"selected x", "omitted", "notes x.txt", "omitted.txt"} {
		if _, err := os.Stat(filepath.Join(destination, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected entry copied: %s %v", name, err)
		}
	}
	expected, err := os.ReadFile(filepath.Join(destination, "selected [x]", "sample.txt"))
	if err != nil || string(expected) != "selected [x]" {
		t.Fatalf("selected file missing: %q %v", expected, err)
	}
}
