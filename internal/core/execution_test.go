package core

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowRecoveryKeepsAttemptsAndVerifiesContent(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("integration test requires a local rclone executable")
	}
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "source"), filepath.Join(dir, "destination")
	if err := os.MkdirAll(filepath.Join(source, "part-a"), 0o700); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Repeat([]byte("rclonego-synthetic-fixture\n"), 320000)
	if err := os.WriteFile(filepath.Join(source, "part-a", "sample.bin"), expected, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "notes.txt"), []byte("synthetic root entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "test-rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, engine, filepath.Join(dir, "state"))
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	definition, err := app.SaveWorkflow(WorkflowDefinition{Name: "Synthetic backup", Source: source, Destination: destination,
		Entries:          []string{"part-a", "notes.txt"},
		VerificationMode: "content", Settings: TransferSettings{Transfers: 1, Checkers: 2, ChunkSize: "16M", BufferSize: "1M", CPUParallelism: 1, Bandwidth: "512K"}})
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	run, err := app.StartWorkflow(definition.ID, "recovery-example")
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	app.Start()
	deadline := time.Now().Add(15 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		jobs := app.Snapshot().Jobs
		for _, job := range jobs {
			if job.Status == "running" && job.PID > 0 {
				observed = true
				break
			}
		}
		if observed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	app.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("did not observe a real engine process before interruption")
	}
	store, err = OpenStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app = NewApp(store, engine, filepath.Join(dir, "state"))
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	snapshot := app.Snapshot()
	// Local copy/clone may finish a step before the next polling sample.
	// Assert the step actually interrupted, rather than assuming the first.
	interrupted := map[string]bool{}
	for _, attempt := range snapshot.Attempts {
		if attempt.Status == "interrupted" {
			interrupted[attempt.StepID] = true
		}
	}
	if len(interrupted) == 0 {
		t.Fatalf("interruption was not retained: %+v", snapshot.Attempts)
	}
	for _, job := range snapshot.Jobs {
		if job.Status == "interrupted" {
			if err := app.Retry(job.ID); err != nil {
				t.Fatal(err)
			}
		}
		if job.Status == "interrupted" || job.Status == "pending" {
			settings := definition.Settings
			settings.Bandwidth = ""
			if err := app.UpdateParameters(job.ID, JobUpdateRequest{Settings: settings}); err != nil {
				t.Fatal(err)
			}
		}
	}
	app.Start()
	defer app.Close()
	deadline = time.Now().Add(20 * time.Second)
	var report RunReport
	for time.Now().Before(deadline) {
		report, err = app.ReportRun(run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status == "completed" || report.Status == "failed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if report.Status != "completed" || report.Verification != "content_compared" {
		t.Fatalf("run did not complete verified: %+v", report)
	}
	retried := false
	for _, step := range report.Steps {
		if interrupted[step.ID] && step.Attempt >= 2 {
			retried = true
		}
	}
	if len(report.Attempts) < 4 || !retried {
		t.Fatalf("previous attempt was overwritten: %+v", report.Attempts)
	}
	actual, err := os.ReadFile(filepath.Join(destination, "part-a", "sample.bin"))
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("destination differs from source")
	}
	remotes, err := app.Remotes(context.Background())
	if err != nil || len(remotes) != 0 {
		t.Fatal("synthetic config unexpectedly saw external accounts")
	}
}
