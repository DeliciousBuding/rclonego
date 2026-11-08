package core

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Real mount tests are opt-in because the host needs WinFsp/FUSE and an
// interactive Windows filesystem session. Every path and remote is synthetic.
func TestOwnedMountCacheReadsSyntheticVFS(t *testing.T) {
	if os.Getenv("RCLONEGO_TEST_MOUNTS") != "1" {
		t.Skip("set RCLONEGO_TEST_MOUNTS=1 on a host with WinFsp/FUSE")
	}
	if diagnostic := mountDependency(); diagnostic.Status != "ready" {
		t.Skip(diagnostic.Message)
	}
	engine := os.Getenv("RCLONEGO_TEST_ENGINE")
	if engine == "" {
		engine, _ = exec.LookPath("rclone")
	}
	if engine == "" {
		t.Skip("integration test requires a verified rclone executable")
	}
	root := t.TempDir()
	source, data := filepath.Join(root, "source"), filepath.Join(root, "state")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	expected := bytes.Repeat([]byte("synthetic cached file\n"), 256)
	if err := os.WriteFile(filepath.Join(source, "read.bin"), expected, 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "rclone.conf")
	if err := os.WriteFile(config, []byte("[fixture]\ntype = alias\nremote = "+filepath.ToSlash(source)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	app := NewApp(store, engine, data)
	app.ConfigureEngine(config, strings.Repeat("t", 64))
	defer app.Close()
	target := syntheticMountTarget(root)
	if target == "" {
		t.Skip("no unused synthetic test drive available")
	}
	cfg := MountConfig{ID: "synthetic-vfs", Name: "Synthetic VFS", Remote: "fixture:", Drive: target,
		CacheDir: filepath.Join(root, "cache"), CacheMax: "16M", CacheMaxAge: "1h"}
	if err := app.SaveMounts([]MountConfig{cfg}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	pid, err := app.StartMount(ctx, cfg)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.StopMount(ctx, cfg); err != nil {
			state := app.Snapshot()
			if state.OwnedMounts[cfg.ID] == pid && state.MountIdentities[cfg.ID] == processIdentity(pid) {
				if process, findErr := os.FindProcess(pid); findErr == nil {
					_ = process.Kill()
				}
			}
			t.Errorf("synthetic mount cleanup failed: %v", err)
		}
	}()
	fileRoot := target
	if runtime.GOOS == "windows" {
		fileRoot += `\`
	}
	actual, err := os.ReadFile(filepath.Join(fileRoot, "read.bin"))
	if err != nil || !bytes.Equal(actual, expected) {
		log, _ := tailFile(filepath.Join(data, "mounts", cfg.ID+".log"), 3000)
		t.Fatalf("synthetic VFS read differs: %v\n%s", err, log)
	}
	view, err := app.MountCache(context.Background(), cfg.ID)
	if err != nil || !view.Available || view.DiskCache == nil || view.DiskCache.Files < 1 {
		t.Fatalf("real VFS cache counters unavailable after read: %+v / %v", view, err)
	}
	write := []byte("synthetic writeback\n")
	if err := os.WriteFile(filepath.Join(fileRoot, "write.txt"), write, 0o600); err != nil {
		t.Fatal(err)
	}
	view, err = app.MountCache(context.Background(), cfg.ID)
	if err != nil || !view.Available || view.QueueError != "" {
		t.Fatalf("real VFS upload state unavailable: %+v / %v", view, err)
	}
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		actual, err := os.ReadFile(filepath.Join(source, "write.txt"))
		if err == nil && bytes.Equal(actual, write) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("synthetic VFS write did not reach the source backend")
}
