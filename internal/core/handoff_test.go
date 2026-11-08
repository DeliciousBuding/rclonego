package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type legacyFixture struct {
	mu               sync.Mutex
	state            legacyState
	file             string
	onPause          func()
	failPauseReceipt bool
}

func (f *legacyFixture) save(t *testing.T) {
	t.Helper()
	raw, err := json.Marshal(f.state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func handoffFixture(t *testing.T) (*App, *legacyFixture, HandoffRequest) {
	t.Helper()
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		t.Skip("requires a verified isolated engine")
	}
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "profile")
	store, err := OpenStore(profile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	app := NewApp(store, engine, profile)
	config := filepath.Join(root, "legacy.conf")
	if err := os.WriteFile(config, []byte("[storage]\ntype = local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	f := &legacyFixture{file: filepath.Join(root, "state.json"), state: legacyState{
		Version: 1, Defaults: defaultTransferSettings(),
		Jobs: []Job{
			{ID: "old-copy", Name: "Synthetic · copy", Operation: "copy", Position: 1,
				WorkflowID: "synthetic-run", WorkflowStep: 1, WorkflowSize: 2,
				Source: source, Destination: destination, Status: "running", PID: os.Getpid(), RCPort: 1,
				Transfers: 2, Checkers: 4, ChunkSize: "16M", BufferSize: "8M", CreatedAt: now, StartedAt: &now, LastUpdatedAt: now},
			{ID: "old-check", Name: "Synthetic · check", Operation: "check", Position: 2,
				WorkflowID: "synthetic-run", WorkflowStep: 2, WorkflowSize: 2,
				Source: source, Destination: destination, Status: "pending",
				Transfers: 2, Checkers: 4, CreatedAt: now, LastUpdatedAt: now},
		},
	}}
	f.save(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/api/queue/pause":
			if r.Header.Get("X-DataOps-Console") != "1" {
				w.WriteHeader(403)
				return
			}
			var input struct {
				Paused bool `json:"paused"`
			}
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				w.WriteHeader(400)
				return
			}
			f.state.QueuePaused = input.Paused
			if input.Paused && f.onPause != nil {
				f.onPause()
			}
			f.save(t)
			if input.Paused && f.failPauseReceipt {
				_, _ = w.Write([]byte("unconfirmed"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"paused": input.Paused})
		case "/api/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": f.state.Jobs, "paused": f.state.QueuePaused})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return app, f, HandoffRequest{StatePath: f.file, ConfigPath: config, Endpoint: server.URL}
}

func TestHandoffPreviewsAndWaitsForRecordedCompletion(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if f.state.QueuePaused || len(app.Snapshot().Jobs) != 0 || len(preview.Jobs) != 2 {
		t.Fatal("preview changed either queue")
	}
	request.PreviewKey = preview.PreviewKey
	result, err := app.BeginHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	state := app.Snapshot()
	if !f.state.QueuePaused || result.Status != "waiting" || state.Jobs[0].Status != "handoff" ||
		state.Jobs[0].ProcessIdentity != "" || state.Jobs[1].Status != "pending" {
		t.Fatalf("ownership was not separated: %+v", state.Handoff)
	}
	if !app.dispatchBlocked(state.Jobs[1], state) {
		t.Fatal("dependent verification started before handoff")
	}
	if _, err := app.BeginHandoff(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(app.Snapshot().Jobs) != 2 {
		t.Fatal("retrying import duplicated tasks")
	}
	unrelated := Job{Source: filepath.Join(t.TempDir(), "source"), Destination: "different:archive"}
	if app.dispatchBlocked(unrelated, state) {
		t.Fatal("unrelated task was globally blocked")
	}
	conflict := Job{Source: state.Jobs[0].Source, Destination: "different:archive"}
	if !app.dispatchBlocked(conflict, state) {
		t.Fatal("overlapping source bypassed handoff")
	}
	f.mu.Lock()
	f.state.Jobs[0].PID = 0
	f.state.Jobs[0].Status = "completed"
	f.save(t)
	f.mu.Unlock()
	app.refreshHandoff(context.Background())
	if app.Snapshot().Handoff.Status != "blocked" {
		t.Fatal("completion without exit evidence was accepted")
	}
	now, success := time.Now().UTC(), 0
	f.mu.Lock()
	f.state.Jobs[0].ExitCode, f.state.Jobs[0].FinishedAt = &success, &now
	f.save(t)
	f.mu.Unlock()
	app.refreshHandoff(context.Background())
	state = app.Snapshot()
	if state.Handoff.Status != "complete" || state.Jobs[0].Status != "completed" || WorkflowBlocked(state.Jobs, state.Jobs[1]) {
		t.Fatalf("successful recorded result did not release next step: %+v", state.Handoff)
	}
	if state.Jobs[0].Attempt != 1 || len(state.Attempts) != 1 {
		t.Fatal("legacy result lost its attempt record")
	}
}

func TestHandoffFailureCannotReleaseFinalVerificationAndCanReturn(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	if _, err = app.BeginHandoff(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	now, code := time.Now().UTC(), 1
	f.mu.Lock()
	f.state.Jobs[0].Status, f.state.Jobs[0].PID = "failed", 0
	f.state.Jobs[0].ExitCode, f.state.Jobs[0].FinishedAt = &code, &now
	f.save(t)
	f.mu.Unlock()
	app.refreshHandoff(context.Background())
	state := app.Snapshot()
	if state.Jobs[0].Status != "failed" || !WorkflowBlocked(state.Jobs, state.Jobs[1]) {
		t.Fatal("failed copy released verification")
	}
	if _, err = app.ReturnHandoff(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.state.QueuePaused || app.Snapshot().Handoff.Status != "returned" {
		t.Fatal("return did not restore legacy dispatch")
	}
	if !app.dispatchBlocked(state.Jobs[1], app.Snapshot()) {
		t.Fatal("returned import remained dispatchable")
	}
}

func TestHandoffRejectsChangedDefinitionsAndMismatchedClient(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	f.state.Jobs[1].Destination += "-changed"
	f.save(t)
	if _, err := app.BeginHandoff(context.Background(), request); err == nil {
		t.Fatal("used a stale preview")
	}
	if f.state.QueuePaused {
		t.Fatal("stale preview paused old queue")
	}
	for _, endpoint := range []string{"https://localhost:8787", "http://example.com", "http://localhost:8787/api", "http://user@localhost:8787"} {
		if _, err := legacyEndpoint(endpoint); err == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
}

func TestTransferPathOverlapRespectsRemoteBoundaries(t *testing.T) {
	for _, pair := range [][2]string{{"storage:", "storage:photos"}, {"storage:photos", "storage:photos/2026"}} {
		if !transferPathsOverlap(pair[0], pair[1]) {
			t.Fatal("missed overlapping remote")
		}
	}
	for _, pair := range [][2]string{{"storage:photos", "storage:photos-old"}, {"storage:photos", "other:photos"}, {"", "storage:"}} {
		if transferPathsOverlap(pair[0], pair[1]) {
			t.Fatal("unrelated remote was blocked")
		}
	}
}

func TestHandoffPersistsAcrossRestartAndContinuesThroughSharedQueue(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	if _, err := app.BeginHandoff(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	// Open a second independent DB connection as a fresh service would. The
	// recorded external step must remain handoff, never auto-retry on startup.
	reopened, err := OpenStore(app.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := NewApp(reopened, app.EngineClient().Path, app.dataDir)
	if restarted.Snapshot().Jobs[0].Status != "handoff" {
		t.Fatal("restart changed external ownership")
	}
	// Simulate the old copy actually completing its selected source files.
	source, destination := f.state.Jobs[0].Source, f.state.Jobs[0].Destination
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(source, "sample.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "sample.txt"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	now, code := time.Now().UTC(), 0
	f.mu.Lock()
	f.state.Jobs[0].Status, f.state.Jobs[0].PID = "completed", 0
	f.state.Jobs[0].FinishedAt, f.state.Jobs[0].ExitCode = &now, &code
	f.save(t)
	f.mu.Unlock()
	restarted.Start()
	defer restarted.Close()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		state := restarted.Snapshot()
		if state.Jobs[1].Status == "completed" {
			report, err := restarted.ReportRun("synthetic-run")
			if err != nil || report.Status != "completed" || report.Verification == "not_run" || len(report.Attempts) != 2 {
				t.Fatalf("verification result lost imported or new evidence: %+v %v", report, err)
			}
			if _, err := restarted.ReturnHandoff(context.Background()); err == nil {
				t.Fatal("returned after new work started")
			}
			if !f.state.QueuePaused {
				t.Fatal("old scheduler was resumed after handoff")
			}
			return
		}
		if state.Jobs[1].Status == "failed" {
			t.Fatalf("native verification failed: %s", state.Jobs[1].Error)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("new queue did not continue after the recorded old result")
}

func TestHandoffRevalidatesAfterPausingAndRollsBackDispatch(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	f.onPause = func() { f.state.Jobs[0].Status = "completed" }
	if _, err := app.BeginHandoff(context.Background(), request); err == nil {
		t.Fatal("accepted an unverified post-pause completion")
	}
	if f.state.QueuePaused || app.Snapshot().Handoff != nil || len(app.Snapshot().Jobs) != 0 {
		t.Fatal("failed final validation did not restore old dispatch")
	}
}

func TestHandoffRejectsHistoryCreatedDuringPause(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	f.onPause = func() {
		if err := app.store.Update(func(state *State) error {
			state.Runs = append(state.Runs, Run{ID: "synthetic-run", Name: "Concurrent run"})
			return nil
		}); err != nil {
			t.Error(err)
		}
	}
	if _, err := app.BeginHandoff(context.Background(), request); err == nil {
		t.Fatal("combined legacy tasks with a concurrent run")
	}
	if app.Snapshot().Handoff != nil || len(app.Snapshot().Jobs) != 0 || f.state.QueuePaused {
		t.Fatal("failed import altered task ownership")
	}
}

func TestHandoffCannotReturnAfterNewAttemptIsRetriedOrDeleted(t *testing.T) {
	app, _, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	if _, err := app.BeginHandoff(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := app.store.Update(func(state *State) error {
		state.Handoff.Status = "complete"
		state.Jobs[0].Status = "completed"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	job := app.Snapshot().Jobs[1]
	job.Status = "running"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	job.Status = "failed"
	job.Attempt = 1
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.Retry(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReturnHandoff(context.Background()); err == nil {
		t.Fatal("retry cleared irreversible ownership evidence")
	}
	if err := app.DeleteWorkflow(job.WorkflowID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReturnHandoff(context.Background()); err == nil {
		t.Fatal("deleting history cleared irreversible ownership evidence")
	}
}

func TestHandoffRestoresDispatchAfterUnconfirmedPause(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	f.failPauseReceipt = true
	if _, err := app.BeginHandoff(context.Background(), request); err == nil {
		t.Fatal("accepted an unconfirmed pause")
	}
	if f.state.QueuePaused || app.Snapshot().Handoff != nil {
		t.Fatal("pause response failure left old dispatch paused")
	}
}

func TestHandoffCannotContinueWhenPreviousQueueAdvancedElsewhere(t *testing.T) {
	app, f, request := handoffFixture(t)
	preview, err := app.PreviewHandoff(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewKey = preview.PreviewKey
	if _, err := app.BeginHandoff(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	now, code := time.Now().UTC(), 0
	f.mu.Lock()
	f.state.Jobs[0].Status, f.state.Jobs[0].PID = "completed", 0
	f.state.Jobs[0].FinishedAt, f.state.Jobs[0].ExitCode = &now, &code
	f.state.Jobs[1].Status, f.state.Jobs[1].PID = "running", os.Getpid()
	f.save(t)
	f.mu.Unlock()
	app.refreshHandoff(context.Background())
	if app.Snapshot().Handoff.Status != "blocked" {
		t.Fatal("old client advancing to another step released new dispatch")
	}
}
