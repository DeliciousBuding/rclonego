package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func workflowFixture(t *testing.T, app *App) []Job {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := app.queue.AddWorkflow(WorkflowRequest{Name: "backup", Source: root, Destination: t.TempDir(), IdempotencyKey: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || jobs[2].Operation != "check" {
		t.Fatalf("unexpected workflow: %+v", jobs)
	}
	return jobs
}

func TestWorkflowFailureRetryAndReorderPreserveDependencies(t *testing.T) {
	app := testApp(t)
	jobs := workflowFixture(t, app)
	jobs[0].Status = "failed"
	if err := app.store.SaveJob(jobs[0]); err != nil {
		t.Fatal(err)
	}
	if _, ok := nextRunnable(app.store.Snapshot().Jobs); ok {
		t.Fatal("failed copy was bypassed")
	}
	if err := app.queue.Move(jobs[2].ID, -1); err == nil {
		t.Fatal("final check was allowed to cross its copy step")
	}
	if err := app.queue.Delete(jobs[0].ID); err == nil {
		t.Fatal("required workflow step was deleted")
	}
	if err := app.queue.Retry(jobs[0].ID); err != nil {
		t.Fatal(err)
	}
	next, ok := nextRunnable(app.store.Snapshot().Jobs)
	if !ok || next.ID != jobs[0].ID {
		t.Fatal("retry did not restore the first step")
	}
	// Simulate corrupt display positions: dependencies must still hold.
	jobs[2].Position = -1
	if err := app.store.SaveJob(jobs[2]); err != nil {
		t.Fatal(err)
	}
	next, ok = nextRunnable(app.store.Snapshot().Jobs)
	if !ok || next.ID != jobs[0].ID {
		t.Fatal("display order bypassed fixed workflow dependencies")
	}
	if !WorkflowBlocked([]Job{jobs[1], jobs[2]}, jobs[2]) {
		t.Fatal("missing predecessor was treated as complete")
	}
}

func TestWorkflowCanSelectTopLevelEntries(t *testing.T) {
	app := testApp(t)
	root := t.TempDir()
	for _, dir := range []string{"2024", "2025", "2026"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := app.queue.AddWorkflow(WorkflowRequest{
		Name: "resume", Source: root, Destination: "example-remote:archive",
		IdempotencyKey: "selected", Entries: []string{"2025", "2024"},
		TotalBytes: 100, InitialBytes: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || !strings.Contains(jobs[0].Source, "2025") || !strings.Contains(jobs[1].Source, "2024") {
		t.Fatalf("unexpected selected workflow: %+v", jobs)
	}
	if jobs[0].Checksum || jobs[0].WorkflowTotalBytes != 100 || jobs[0].WorkflowInitialBytes != 25 {
		t.Fatalf("workflow metadata lost: %+v", jobs[0])
	}
	if _, err := app.queue.AddWorkflow(WorkflowRequest{
		Name: "bad", Source: root, Destination: "example-remote:archive",
		IdempotencyKey: "bad-entry", Entries: []string{"../outside"},
	}); err == nil {
		t.Fatal("path traversal entry accepted")
	}
}

func TestWorkflowCreationIsAtomicAndIdempotent(t *testing.T) {
	app := testApp(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "one"), 0o700); err != nil {
		t.Fatal(err)
	}
	request := WorkflowRequest{Name: "backup", Source: root, Destination: "dst", IdempotencyKey: "atomic"}
	if err := app.store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.queue.AddWorkflow(request); err == nil {
		t.Fatal("expected persistence failure")
	}
	if len(app.store.Snapshot().Jobs) != 0 {
		t.Fatal("half-created workflow remained after failure")
	}
	app = testApp(t)
	first, err := app.queue.AddWorkflow(request)
	if err != nil {
		t.Fatal(err)
	}
	// Files discovered after the original request must not append new steps
	// during an idempotent retry.
	if err := os.WriteFile(filepath.Join(root, "later.txt"), []byte("later"), 0o600); err != nil {
		t.Fatal(err)
	}
	retry, err := app.queue.AddWorkflow(request)
	if err != nil || len(first) != len(retry) || first[0].ID != retry[0].ID {
		t.Fatalf("idempotency failed: %+v / %v", retry, err)
	}
}

func TestExclusionProofRejectsChangesAndPathEscape(t *testing.T) {
	root := t.TempDir()
	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	for _, dir := range []string{"duplicate", "canonical"} {
		path := filepath.Join(root, dir)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(path, "image.jpg")
		if err := os.WriteFile(path, []byte("same"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	proof := map[string]any{"library": root, "directories": []any{map[string]any{
		"source": "duplicate", "keep": "canonical", "files": []any{map[string]any{
			"relative": "image.jpg", "size": 4, "source_mtime_ns": stamp.UnixNano(), "keep_mtime_ns": stamp.UnixNano(), "sha256": strings.Repeat("a", 64),
		}},
	}}}
	raw, _ := json.Marshal(proof)
	file := filepath.Join(t.TempDir(), "proof.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	excludes, err := proofExclusions(file, root)
	if err != nil || len(excludes) != 1 {
		t.Fatalf("proof rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "duplicate", "new.jpg"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := proofExclusions(file, root); err == nil {
		t.Fatal("new file was silently excluded")
	}
	if _, err := confinedPath(root, "../outside"); err == nil {
		t.Fatal("relative path escaped source")
	}
}
