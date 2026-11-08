package core

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/persistence"
)

func TestCriticalEngineFailureKeepsTheCauseInTaskResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempt.log")
	log := `{"level":"critical","msg":"Failed to create file system: missing configured remote"}` + "\n"
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	message := transferError(errors.New("exit status 1"), path).Error()
	if !strings.Contains(message, "missing configured remote") {
		t.Fatalf("A task result lost the engine's actual failure cause: %s", message)
	}
}

func TestActiveTransferStopsWhenStateBecomesUnwritable(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("integration test requires a verified local rclone executable")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "synthetic.bin"), bytes.Repeat([]byte("synthetic\n"), 800000), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, filepath.Join(dir, "state"))
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	first, err := app.AddJob(JobCreateRequest{Name: "Synthetic slow copy", Operation: "copy",
		Source: source, Destination: filepath.Join(dir, "first"), Transfers: 1, Checkers: 1, Bandwidth: "128K"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.AddJob(JobCreateRequest{Name: "Synthetic next copy", Operation: "copy",
		Source: source, Destination: filepath.Join(dir, "second")})
	if err != nil {
		t.Fatal(err)
	}
	app.queue.Start()
	defer app.queue.Close()
	deadline := time.Now().Add(10 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		for _, job := range app.Snapshot().Jobs {
			if job.ID == first.ID && job.PID > 0 && job.ProcessIdentity != "" {
				observed = true
				break
			}
		}
		if observed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("did not observe a recorded engine process")
	}
	store.mu.Lock()
	store.db.Close()
	store.mu.Unlock()
	stopped := false
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		app.queue.mu.Lock()
		stopped = app.queue.running == "" && app.queue.fault != nil
		app.queue.mu.Unlock()
		if stopped {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !stopped || app.Health().Ready {
		t.Fatal("engine continued after its state could not be saved")
	}
	for _, job := range app.Snapshot().Jobs {
		if job.ID == second.ID && (job.Status != "pending" || job.PID != 0) {
			t.Fatal("another task was dispatched while completion could not be persisted")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "second")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("next task copied data while storage was unavailable")
	}
}

func TestFailedFinalWriteBlocksDispatchAndCanRecoverWithoutLosingResult(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, "", dir)
	job, err := app.AddJob(JobCreateRequest{Name: "Synthetic transfer", Operation: "copy", Source: "source", Destination: "destination"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	job.Bytes, job.TotalBytes = 100, 100
	app.queue.finish(job, "completed", nil, nil)
	health := app.Health()
	if health.Ready || health.Code != "storage_unavailable" || health.Since == nil {
		t.Fatalf("final write failure was not surfaced: %+v", health)
	}
	if store.Snapshot().Jobs[0].Status != "pending" {
		t.Fatal("failed transaction changed the authoritative in-memory status")
	}
	if err := app.Recover(); err == nil {
		t.Fatal("closed database was declared recovered")
	}
	reopened, err := persistence.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Load(); err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	store.db = reopened
	if err := app.Recover(); err != nil {
		t.Fatal(err)
	}
	recovered := app.Snapshot().Jobs[0]
	if !app.Health().Ready || recovered.Status != "completed" || recovered.Bytes != 100 {
		t.Fatalf("recovery lost the actual completion: %+v %+v", recovered, app.Health())
	}
}

func TestStartupRecoveryFailureIsVisibleAndDoesNotDispatch(t *testing.T) {
	app := testApp(t)
	config := filepath.Join(app.dataDir, "synthetic-rclone.conf")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	job, err := app.AddJob(JobCreateRequest{Name: "Synthetic interrupted transfer", Operation: "copy", Source: "source", Destination: "destination"})
	if err != nil {
		t.Fatal(err)
	}
	job.Status = "interrupted"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	app.store.db.Close()
	app.Start()
	defer app.Close()
	if health := app.Health(); health.Ready || health.Code != "storage_unavailable" {
		t.Fatalf("startup silently ignored recovery failure: %+v", health)
	}
	if got := app.Snapshot().Jobs[0]; got.Status != "interrupted" || got.PID != 0 {
		t.Fatalf("startup dispatched unrecorded work: %+v", got)
	}
}

func TestRecoveryDistinguishesReusedPIDFromRunningOwnedProcess(t *testing.T) {
	identity := processIdentity(os.Getpid())
	if identity == "" {
		t.Fatal("current process start identity unavailable")
	}
	app := testApp(t)
	job, err := app.AddJob(JobCreateRequest{Name: "Synthetic recovery", Operation: "copy", Source: "source", Destination: "destination"})
	if err != nil {
		t.Fatal(err)
	}
	job.Status, job.PID, job.ProcessIdentity = "interrupted", os.Getpid(), identity
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.queue.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	if app.Snapshot().Jobs[0].Status != "interrupted" || !app.runtime.Busy() {
		t.Fatal("live owned process could have a competing transfer")
	}
	job.ProcessIdentity = identity + "-different-start"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if app.runtime.Busy() {
		t.Fatal("reused PID blocked the queue as if it were the old engine")
	}
	if err := app.queue.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	got := app.Snapshot().Jobs[0]
	if got.Status != "pending" || got.PID != 0 || got.ProcessIdentity != "" {
		t.Fatalf("reused PID prevented safe recovery: %+v", got)
	}
}

func TestLegacyPIDDoesNotAcquireProcessOwnership(t *testing.T) {
	app := testApp(t)
	job, err := app.AddJob(JobCreateRequest{Name: "Legacy recovery", Operation: "copy", Source: "source", Destination: "destination"})
	if err != nil {
		t.Fatal(err)
	}
	job.Status, job.PID = "interrupted", os.Getpid()
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.queue.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	if app.Snapshot().Jobs[0].Status != "interrupted" || app.Health().Code != "previous_process_running" {
		t.Fatal("a legacy PID was treated as safe ownership")
	}
}

func TestMountOwnershipWriteFailureRunsCompensation(t *testing.T) {
	app := testApp(t)
	app.store.db.Close()
	compensated := false
	err := app.saveMountOwner(MountConfig{ID: "synthetic-mount"}, 123, "synthetic-start", func() error {
		compensated = true
		return nil
	})
	if err == nil || !compensated {
		t.Fatal("failed ownership registration left the mount process running")
	}
	if len(app.Snapshot().OwnedMounts) != 0 {
		t.Fatal("failed registration claimed ownership in memory")
	}
}

func TestMountCompensationFailurePreservesBothCauses(t *testing.T) {
	app := testApp(t)
	cleanup := errors.New("synthetic cleanup failure")
	err := app.saveMountOwner(MountConfig{ID: "synthetic-mount"}, 123, "", func() error { return cleanup })
	if !errors.Is(err, cleanup) {
		t.Fatal("cleanup failure disappeared from mount error")
	}
}

func TestStateMigrationPreservesLegacyJobsAndRejectsFutureState(t *testing.T) {
	dir := t.TempDir()
	db, err := persistence.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	record := persistence.Record{ID: "legacy-job", Status: "completed", Position: 1,
		Payload: []byte(`{"id":"legacy-job","name":"Legacy task","status":"completed","bytes":42}`)}
	if err := db.Save([]byte(`{"version":1,"appearance":"dark"}`), map[string][]persistence.Record{"steps": {record}}, nil); err != nil {
		t.Fatal(err)
	}
	db.Close()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := store.Snapshot()
	if state.Version != stateVersion || state.Appearance != "dark" || len(state.Jobs) != 1 || state.Jobs[0].Bytes != 42 {
		t.Fatalf("legacy state changed during migration: %+v", state)
	}
	store.Close()
	db, err = persistence.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Load(); err != nil {
		t.Fatal(err)
	}
	future := []byte(`{"version":999,"appearance":"dark"}`)
	if err := db.Save(future, map[string][]persistence.Record{"steps": {record}}, nil); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if opened, err := OpenStore(dir); err == nil {
		opened.Close()
		t.Fatal("future state was silently downgraded")
	}
	db, err = persistence.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	loaded, err := db.Load()
	if err != nil || !bytes.Equal(loaded["settings"][0], future) || len(loaded["steps"]) != 1 {
		t.Fatal("unsupported state was modified")
	}
}

func TestLiveLogOutputIsBoundedAndLatestOutputSurvives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.log")
	log, err := openTaskLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	chunk := bytes.Repeat([]byte("synthetic log line\n"), 100000)
	for i := 0; i < 12; i++ {
		if _, err := log.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	last := []byte("LATEST SYNTHETIC RESULT\n")
	if _, err := log.Write(last); err != nil {
		t.Fatal(err)
	}
	log.Close()
	info, err := os.Stat(path)
	if err != nil || info.Size() > taskLogLimit {
		t.Fatal("active engine log grew beyond its limit")
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.HasSuffix(raw, last) {
		t.Fatal("bounded log lost the latest engine output")
	}
}

func TestLogRetentionPreservesActiveDataAndUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	active := filepath.Join(dir, "active.log")
	archived := filepath.Join(dir, "archived.log")
	unrelated := filepath.Join(dir, "notes.txt")
	for _, path := range []string{active, archived, unrelated} {
		if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	err := pruneTaskLogs(dir, map[string]bool{active: true},
		LogRetentionPolicy{MaxAge: 24 * time.Hour, MaxFiles: 1, MaxBytes: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archived); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired completed log was retained")
	}
	for _, path := range []string{active, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retention removed protected/unrelated data: %s", filepath.Base(path))
		}
	}
}
