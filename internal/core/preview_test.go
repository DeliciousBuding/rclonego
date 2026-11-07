package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncPreviewCannotDeleteAndConfirmationBindsParameters(t *testing.T) {
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	dir := t.TempDir()
	source, target, data := filepath.Join(dir, "source"), filepath.Join(dir, "target"), filepath.Join(dir, "state")
	for _, path := range []string{source, target} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(source, "sample.txt"), []byte("synthetic"), 0o600)
	os.WriteFile(filepath.Join(target, "extra.txt"), []byte("preserve until confirmed"), 0o600)
	config := filepath.Join(dir, "rclone.conf")
	os.WriteFile(config, nil, 0o600)
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	request := JobCreateRequest{Name: "Synthetic mirror", Operation: "sync", Source: source, Destination: target, AllowDelete: true}
	if _, err := app.AddJob(request); err == nil {
		t.Fatal("mirror accepted without preview")
	}
	preview, err := app.PreviewJob(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "extra.txt")); err != nil {
		t.Fatal("preview deleted destination file")
	}
	if _, err := os.Stat(filepath.Join(target, "sample.txt")); !os.IsNotExist(err) {
		t.Fatal("preview copied file")
	}
	request.PreviewID = preview.ID
	modified := request
	modified.Destination = filepath.Join(dir, "another-target")
	if _, err := app.AddJob(modified); err == nil {
		t.Fatal("accepted mismatched preview")
	}
	job, err := app.AddJob(request)
	if err != nil {
		t.Fatal(err)
	}
	app.Start()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, current := range app.Snapshot().Jobs {
			if current.ID != job.ID {
				continue
			}
			if current.Status == "failed" {
				t.Fatal(current.Error)
			}
			if current.Status == "completed" {
				if _, err := os.Stat(filepath.Join(target, "extra.txt")); !os.IsNotExist(err) {
					t.Fatal("confirmed mirror did not delete extra file")
				}
				if _, err := os.Stat(filepath.Join(target, "sample.txt")); err != nil {
					t.Fatal("confirmed mirror did not copy file")
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("mirror did not complete")
}
