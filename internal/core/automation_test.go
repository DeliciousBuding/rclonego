package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	observation "github.com/DeliciousBuding/rclonego/internal/automation"
)

func automationFixture(t *testing.T) (*App, Automation, *time.Time) {
	t.Helper()
	app := testApp(t)
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	definition, err := app.SaveAutomation(Automation{Name: "automatic copy", Kind: "watch",
		Source: source, Destination: t.TempDir(), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	app.automations.now = func() time.Time { return clock }
	// Periodic reconciliation must work even if an OS watcher cannot be created.
	app.automations.newMonitor = func() (*observation.Monitor, error) { return nil, errors.New("synthetic watcher unavailable") }
	return app, definition, &clock
}

func TestAutomationWaitsForStableFilesAndNeverOverlaps(t *testing.T) {
	app, definition, clock := automationFixture(t)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Jobs) != 0 {
		t.Fatal("copy dispatched before stable observation")
	}
	*clock = clock.Add(3 * time.Second)
	app.automations.tick(context.Background())
	first := app.Snapshot().Jobs
	if len(first) != 1 || first[0].Operation != "copy" || first[0].AllowDelete {
		t.Fatalf("unexpected automatic copy: %+v", first)
	}
	if err := os.WriteFile(filepath.Join(definition.Source, "sample.txt"), []byte("changed while queued"), 0o600); err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(10 * time.Minute)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Jobs) != 1 {
		t.Fatal("change during existing task dispatched overlapping copy")
	}
	first[0].Status = "completed"
	if err := app.store.SaveJob(first[0]); err != nil {
		t.Fatal(err)
	}
	app.automations.tick(context.Background())
	*clock = clock.Add(3 * time.Second)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Jobs) != 2 {
		t.Fatal("change during previous task was lost after its completion")
	}
	if app.ListAutomations()[0].WatchWarning == "" {
		t.Fatal("unavailable watcher was not surfaced")
	}
}

func TestDisabledAutomationRequiresExplicitRequestAndFailureDoesNotLoop(t *testing.T) {
	app, definition, clock := automationFixture(t)
	definition.Enabled = false
	if _, err := app.SaveAutomation(definition); err != nil {
		t.Fatal(err)
	}
	app.automations.tick(context.Background())
	*clock = clock.Add(time.Hour)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Jobs) != 0 {
		t.Fatal("disabled automation created a task")
	}
	if err := app.TriggerAutomation(definition.ID); err != nil {
		t.Fatal(err)
	}
	app.automations.tick(context.Background())
	*clock = clock.Add(3 * time.Second)
	app.automations.tick(context.Background())
	job := app.Snapshot().Jobs[0]
	job.Status, job.Error = "failed", "synthetic connection failure"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	app.automations.tick(context.Background())
	for i := 0; i < 4; i++ {
		*clock = clock.Add(time.Hour)
		app.automations.tick(context.Background())
	}
	if len(app.Snapshot().Jobs) != 1 || app.ListAutomations()[0].Status != "attention" {
		t.Fatal("failed automation retried without an explicit action")
	}
	if err := app.TriggerAutomation(definition.ID); err != nil {
		t.Fatal(err)
	}
	app.automations.tick(context.Background())
	*clock = clock.Add(3 * time.Second)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Jobs) != 2 {
		t.Fatal("explicit run did not recover failed automation")
	}
}

func TestAutomationReservationSurvivesRestartWithoutDuplicateDispatch(t *testing.T) {
	data := t.TempDir()
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, "rclone", data)
	definition, err := app.SaveAutomation(Automation{Name: "reserved copy", Kind: "watch",
		Source: t.TempDir(), Destination: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	key := "auto-" + definition.ID + "-reserved"
	job, err := app.AddJob(JobCreateRequest{Name: definition.Name, Operation: "copy",
		Source: definition.Source, Destination: definition.Destination, IdempotencyKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.editAutomation(definition.ID, func(current *Automation) {
		current.PendingKey, current.PendingFingerprint = key, "reserved-baseline"
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	app = NewApp(reopened, "rclone", data)
	app.automations.tick(context.Background())
	if jobs := app.Snapshot().Jobs; len(jobs) != 1 || jobs[0].ID != job.ID {
		t.Fatalf("restart duplicated reserved automatic task: %+v", jobs)
	}
	current := app.ListAutomations()[0]
	if current.PendingKey != "" || current.LastJobID != job.ID || current.LastSubmissionKey != key {
		t.Fatalf("reservation did not finish reconciliation: %+v", current)
	}
}

func TestIntervalCoalescesRunRequestWhileBusy(t *testing.T) {
	app := testApp(t)
	workflow, err := app.SaveWorkflow(WorkflowDefinition{Name: "scheduled workflow", Source: t.TempDir(), Destination: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	definition, err := app.SaveAutomation(Automation{Name: "every minute", Kind: "interval",
		WorkflowID: workflow.ID, IntervalSeconds: 60, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	app.automations.now = func() time.Time { return clock }
	app.automations.tick(context.Background())
	if len(app.Snapshot().Runs) != 0 {
		t.Fatal("new interval ran before its due time")
	}
	clock = clock.Add(time.Minute)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Runs) != 1 {
		t.Fatal("interval did not create saved workflow run")
	}
	for i := 0; i < 3; i++ {
		if err := app.TriggerAutomation(definition.ID); err != nil {
			t.Fatal(err)
		}
	}
	clock = clock.Add(time.Hour)
	app.automations.tick(context.Background())
	if len(app.Snapshot().Runs) != 1 {
		t.Fatal("busy interval overlapped workflow run")
	}
	for _, job := range app.Snapshot().Jobs {
		job.Status = "completed"
		if err := app.store.SaveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	app.automations.tick(context.Background())
	if len(app.Snapshot().Runs) != 2 {
		t.Fatal("manual requests while busy were not coalesced into one later run")
	}
}

func TestAutomationRejectsOverlappingPathsAndUntrustedRuntimeFields(t *testing.T) {
	app := testApp(t)
	source := t.TempDir()
	_, err := app.SaveAutomation(Automation{Name: "recursive", Kind: "watch", Source: source, Destination: filepath.Join(source, "destination")})
	if err == nil {
		t.Fatal("recursive automatic copy was accepted")
	}
	saved, err := app.SaveAutomation(Automation{Name: "safe", Kind: "watch", Source: source, Destination: t.TempDir(),
		Status: "running", LastJobID: "caller-supplied", PendingKey: "caller-supplied", Requested: true})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "disabled" || saved.LastJobID != "" || saved.PendingKey != "" || saved.Requested {
		t.Fatalf("caller injected runtime automation state: %+v", saved)
	}
	if _, err := app.SaveAutomation(Automation{Name: "state loop", Kind: "watch", Source: app.dataDir, Destination: t.TempDir()}); err == nil {
		t.Fatal("automation accepted its own private state as a source")
	}
}

func TestAutomaticCopyTransfersChangesAndPreservesDestinationExtras(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("integration test requires rclone")
	}
	root := t.TempDir()
	source, destination, data := filepath.Join(root, "source"), filepath.Join(root, "destination"), filepath.Join(root, "state")
	for _, path := range []string{source, destination} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sourceFile, destinationFile := filepath.Join(source, "sample.txt"), filepath.Join(destination, "sample.txt")
	if err := os.WriteFile(sourceFile, []byte("initial fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(destination, "destination-only.txt")
	if err := os.WriteFile(extra, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	if _, err := app.SaveAutomation(Automation{Name: "synthetic watched copy", Kind: "watch",
		Source: source, Destination: destination, Enabled: true, DebounceSeconds: 1, StableSeconds: 1, RescanSeconds: 10}); err != nil {
		t.Fatal(err)
	}
	app.Start()
	defer app.Close()
	waitAutomaticCopy := func(expected string, count int) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			actual, readErr := os.ReadFile(destinationFile)
			jobs := app.Snapshot().Jobs
			completed := 0
			for _, job := range jobs {
				if job.Status == "failed" {
					t.Fatalf("automatic engine task failed: %s", job.Error)
				}
				if job.Status == "completed" {
					completed++
				}
			}
			if readErr == nil && string(actual) == expected && completed >= count {
				return
			}
			if current := app.ListAutomations()[0]; current.Error != "" {
				t.Fatalf("automation failed: %s", current.Error)
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("automatic copy did not transfer revision %q", expected)
	}
	waitAutomaticCopy("initial fixture", 1)
	if err := os.WriteFile(sourceFile, []byte("changed synthetic fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitAutomaticCopy("changed synthetic fixture", 2)
	if contents, err := os.ReadFile(extra); err != nil || string(contents) != "preserve" {
		t.Fatal("one-way automatic copy removed destination-only file")
	}
}
