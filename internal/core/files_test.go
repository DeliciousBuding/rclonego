package core

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fileTestApp(t *testing.T) (*App, string) {
	t.Helper()
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("requires rclone")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "state")
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "rclone.conf")
	if err := os.WriteFile(config, []byte("[fixture]\ntype = alias\nremote = "+filepath.ToSlash(source)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	app.Start()
	t.Cleanup(func() { app.Close(); store.Close() })
	return app, source
}

func waitFileJob(t *testing.T, app *App, job Job) Job {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, value := range app.Snapshot().Jobs {
			if value.ID == job.ID && value.Status != "pending" && value.Status != "running" {
				return value
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("file job did not finish: %+v", app.Snapshot().Jobs)
	return Job{}
}

func TestFileChangesUseQueueAndPreserveExistingDestination(t *testing.T) {
	app, root := fileTestApp(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	rename, err := app.AddFileAction(FileActionRequest{Operation: "rename", Path: "fixture:a.txt", Name: "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, rename); result.Status != "failed" {
		t.Fatalf("overwriting rename succeeded: %+v", result)
	}
	data, _ := os.ReadFile(filepath.Join(root, "b.txt"))
	if string(data) != "existing" {
		t.Fatal("existing destination changed")
	}
	rename, err = app.AddFileAction(FileActionRequest{Operation: "rename", Path: "fixture:a.txt", Name: "c.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, rename); result.Status != "completed" {
		t.Fatalf("rename failed: %+v", result)
	}
	if _, err = os.Stat(filepath.Join(root, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("rename left the source")
	}
	data, _ = os.ReadFile(filepath.Join(root, "c.txt"))
	if string(data) != "source" {
		t.Fatal("rename lost content")
	}
	mkdir, err := app.AddFileAction(FileActionRequest{Operation: "mkdir", Path: "fixture:", Name: "folder"})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, mkdir); result.Status != "completed" {
		t.Fatalf("mkdir failed: %+v", result)
	}
	if err := os.WriteFile(filepath.Join(root, "folder", "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	remove, err := app.AddFileAction(FileActionRequest{Operation: "delete", Path: "fixture:folder", IsDir: true, ConfirmDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, remove); result.Status != "failed" {
		t.Fatalf("nonempty folder removed: %+v", result)
	}
	if _, err = os.Stat(filepath.Join(root, "folder", "keep.txt")); err != nil {
		t.Fatal("folder contents were deleted")
	}
	if _, err = app.AddFileAction(FileActionRequest{Operation: "delete", Path: "fixture:c.txt"}); err == nil {
		t.Fatal("unconfirmed delete accepted")
	}
	remove, err = app.AddFileAction(FileActionRequest{Operation: "delete", Path: "fixture:c.txt", ConfirmDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, remove); result.Status != "completed" {
		t.Fatalf("single-file delete failed: %+v", result)
	}
	if _, err = os.Stat(filepath.Join(root, "c.txt")); !os.IsNotExist(err) {
		t.Fatal("file remains after deletion")
	}
	dirRename, err := app.AddFileAction(FileActionRequest{Operation: "rename", Path: "fixture:folder", Name: "renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitFileJob(t, app, dirRename); result.Status != "completed" {
		t.Fatalf("directory rename failed: %+v", result)
	}
	if data, err := os.ReadFile(filepath.Join(root, "renamed", "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatal("directory rename lost contents")
	}
	if len(app.Snapshot().Attempts) < 5 {
		t.Fatal("file operations lost attempt records")
	}
}

func TestPreviewLocalAndRemoteTextAndImageAreBounded(t *testing.T) {
	app, root := fileTestApp(t)
	text := bytes.Repeat([]byte("text 数据\n"), 40000)
	os.WriteFile(filepath.Join(root, "text.txt"), text, 0o600)
	for _, path := range []string{filepath.Join(root, "text.txt"), "fixture:text.txt"} {
		result, err := app.PreviewFile(context.Background(), path)
		if err != nil || result.Kind != "text" || !result.Truncated || len(result.Text) > 256<<10 {
			t.Fatalf("bounded UTF8 text preview: %+v %v", result, err)
		}
	}
	picture := image.NewRGBA(image.Rect(0, 0, 16, 16))
	picture.Set(0, 0, color.RGBA{100, 150, 200, 255})
	var buffer bytes.Buffer
	png.Encode(&buffer, picture)
	os.WriteFile(filepath.Join(root, "image.png"), buffer.Bytes(), 0o600)
	result, err := app.PreviewFile(context.Background(), "fixture:image.png")
	if err != nil || result.Kind != "image" || !bytes.Equal(result.Data, buffer.Bytes()) {
		t.Fatalf("image preview: %+v %v", result, err)
	}
	os.WriteFile(filepath.Join(root, "binary.bin"), []byte{0, 1, 2, 3}, 0o600)
	if _, err = app.PreviewFile(context.Background(), "fixture:binary.bin"); err == nil {
		t.Fatal("binary file became text")
	}
	if _, err = app.PreviewFile(context.Background(), "fixture:"); err == nil {
		t.Fatal("folder preview accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = app.PreviewFile(ctx, filepath.Join(root, "text.txt")); err == nil {
		t.Fatal("cancelled local preview read file")
	}
	for _, name := range []string{"..", "a/b", `a\b`, "a:b", "bad\nname", " name"} {
		if _, err = ChildPath("fixture:", name); err == nil {
			t.Fatalf("unsafe child accepted: %q", name)
		}
	}
}

func TestInterruptedFileChangeRequiresExplicitRetry(t *testing.T) {
	app := testApp(t)
	path := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(path, []byte("sample"), 0o600)
	job, err := app.AddFileAction(FileActionRequest{Operation: "delete", Path: path, ConfirmDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	app.UpdateState(func(state *State) error { state.Jobs[0].Status = "interrupted"; return nil })
	if err = app.queue.ResumeInterrupted(); err != nil {
		t.Fatal(err)
	}
	for _, value := range app.Snapshot().Jobs {
		if value.ID == job.ID && value.Status != "interrupted" {
			t.Fatal("file operation automatically replayed after crash")
		}
	}
}

func TestQueuedFileDeleteRejectsChangedSelection(t *testing.T) {
	app, root := fileTestApp(t)
	if err := app.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "changed.txt")
	os.WriteFile(file, []byte("original"), 0o600)
	job, err := app.AddFileAction(FileActionRequest{Operation: "delete", Path: "fixture:changed.txt", ConfirmDelete: true})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte("replacement with different bytes"), 0o600)
	app.SetPaused(false)
	result := waitFileJob(t, app, job)
	if result.Status != "failed" || !strings.Contains(result.Error, "changed while waiting") {
		t.Fatalf("changed file deleted: %+v", result)
	}
	if actual, _ := os.ReadFile(file); string(actual) != "replacement with different bytes" {
		t.Fatal("changed file was removed")
	}
}
