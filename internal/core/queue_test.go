package core

import (
	"testing"
)

func testApp(t *testing.T) *App {
	t.Helper()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewApp(store, "rclone", t.TempDir())
}

func TestQueueIdempotencyKeyReturnsExistingJob(t *testing.T) {
	app := testApp(t)
	request := JobCreateRequest{
		Name: "copy photos", IdempotencyKey: "agent-request-1",
		Operation: "copy", Source: `X:\sample`, Destination: "example-remote:archive",
		Transfers: 4, Checkers: 8,
	}
	first, err := app.queue.Add(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.queue.Add(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent retry created another job: %s != %s", first.ID, second.ID)
	}
}

func TestSyncRequiresExplicitDeletePermission(t *testing.T) {
	app := testApp(t)
	_, err := app.queue.Add(JobCreateRequest{
		Name: "unsafe sync", Operation: "sync", Source: "src", Destination: "dst",
	})
	if err == nil {
		t.Fatal("sync without allow_delete was accepted")
	}
}

func TestUnsafeOAuthNameIsRejected(t *testing.T) {
	app := testApp(t)
	_, err := app.queue.Add(JobCreateRequest{
		Name: "oauth", Operation: "oauth-reconnect", Source: "bad name",
	})
	if err == nil {
		t.Fatal("unsafe remote name was accepted")
	}
}

func TestFailedJobRetainsMeasuredBytes(t *testing.T) {
	app := testApp(t)
	job, err := app.queue.Add(JobCreateRequest{Name: "copy", Operation: "copy", Source: "s", Destination: "d"})
	if err != nil {
		t.Fatal(err)
	}
	job.Bytes, job.TotalBytes = 4, 100
	app.queue.finish(job, "failed", nil, nil)
	got := app.store.Snapshot().Jobs[0]
	if got.Bytes != 4 || got.Status != "failed" {
		t.Fatalf("failure claimed unmeasured bytes: %+v", got)
	}
}

func TestInterruptedJobResumesOnStart(t *testing.T) {
	app := testApp(t)
	job, err := app.queue.Add(JobCreateRequest{Name: "interrupted", Operation: "copy", Source: "s", Destination: "d"})
	if err != nil {
		t.Fatal(err)
	}
	job.Status = "interrupted"
	job.PID = 0
	job.Error = "service restarted while job was running"
	if err := app.store.SaveJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.queue.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	resumed := app.store.Snapshot().Jobs[0]
	if resumed.Status != "pending" || resumed.Error != "" || resumed.PID != 0 {
		t.Fatalf("interrupted job was not reset: %+v", resumed)
	}
}
