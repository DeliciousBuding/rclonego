package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestQuotaDistinguishesUnknownAndZeroWithoutInventingFreeSpace(t *testing.T) {
	quota, err := decodeRemoteQuota("storage", []byte(`{"total":100,"used":100,"free":0}`))
	if err != nil || quota.Status != "available" || quota.Free == nil || *quota.Free != 0 {
		t.Fatalf("zero free space was lost: %+v %v", quota, err)
	}
	quota, err = decodeRemoteQuota("storage", []byte(`{"used":5,"total":-1,"free":null}`))
	if err != nil || quota.Used == nil || quota.Total != nil || quota.Free != nil {
		t.Fatalf("unknown capacity became a number: %+v %v", quota, err)
	}
	quota, err = decodeRemoteQuota("storage", []byte(`{"other":5}`))
	if err != nil || quota.Status != "unsupported" {
		t.Fatalf("missing quota was reported as available: %+v %v", quota, err)
	}
	if _, err := decodeRemoteQuota("storage", []byte(`{"total":"unlimited"}`)); err == nil {
		t.Fatal("accepted a non-numeric quota")
	}
}

func TestStorageInformationUsesIsolatedEngineAndDoesNotSubmitTasks(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		t.Skip("requires a verified isolated integration engine")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "rclone.conf"), []byte("[storage]\ntype = local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(nil, engine, directory)
	info, err := app.StorageInfo(context.Background(), "storage")
	if err != nil || len(info.Hashes) == 0 || !info.Capabilities.EmptyDirectories {
		t.Fatalf("native backend features: %+v %v", info, err)
	}
	quota, err := app.RemoteQuota(context.Background(), "storage")
	if err != nil || quota.Status != "available" || quota.Total == nil || *quota.Total <= 0 {
		t.Fatalf("native backend quota: %+v %v", quota, err)
	}
	source := filepath.Join(directory, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sample.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	usage, err := app.DirectoryUsage(context.Background(), source)
	if err != nil || usage.Count != 1 || usage.Bytes != 9 || usage.Sizeless != 0 {
		t.Fatalf("native directory inventory: %+v %v", usage, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.DirectoryUsage(ctx, source); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan returned a result: %v", err)
	}
	for _, name := range []string{":local:", "storage:path", "../storage", "--config=x", "storage\n"} {
		if _, err := app.RemoteQuota(context.Background(), name); err == nil {
			t.Fatalf("quota accepted a connection string or directory: %q", name)
		}
		if _, err := app.StorageInfo(context.Background(), name); err == nil {
			t.Fatalf("features accepted a connection string or directory: %q", name)
		}
	}
	if _, err := app.RemoteQuota(context.Background(), "absent"); err == nil {
		t.Fatal("read an unconfigured account")
	}
}

func TestEngineDiagnosticsBelongToOneAttempt(t *testing.T) {
	job := Job{}
	applyStats(&job, &RCStats{Errors: 2, LastError: "network timeout", RetryError: true, Listed: 12, ElapsedTime: 3, ETA: 4})
	if job.EngineErrors != 2 || job.EngineLastError != "network timeout" || !job.EngineRetryError || job.EngineListed != 12 {
		t.Fatal("engine diagnosis was discarded")
	}
	beginAttempt(&job)
	if job.EngineErrors != 0 || job.EngineLastError != "" || job.EngineRetryError || job.EngineListed != 0 || job.EngineElapsed != 0 {
		t.Fatal("new attempt retained the previous attempt's diagnosis")
	}
}

func TestExternalEngineDiscoveryMatchesExecutableNames(t *testing.T) {
	for _, name := range []string{"rclone", "RCLONE.EXE", "/usr/bin/rclone", `C:\tools\rclone.exe`} {
		if !isRcloneProcess(name) {
			t.Fatalf("missed an engine: %q", name)
		}
	}
	for _, name := range []string{"rclonego", "other-rclone.exe", "rclone.exe --config private", "worker"} {
		if isRcloneProcess(name) {
			t.Fatalf("included an unrelated process: %q", name)
		}
	}
}
