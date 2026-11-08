package automation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanDetectsChangeWithoutDependingOnNotifications(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(file, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("second revision"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatal("periodic scan missed a changed file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, root); err == nil {
		t.Fatal("cancelled scan continued")
	}
}

func TestNewSubdirectoryCanBeAddedToNotificationWatch(t *testing.T) {
	root := t.TempDir()
	monitor, err := NewMonitor()
	if err != nil {
		t.Fatal(err)
	}
	defer monitor.Close()
	first, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.SyncDirectories(first.Directories); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "nested")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	waitChange(t, monitor)
	second, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := monitor.SyncDirectories(second.Directories); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "sample.txt"), []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitChange(t, monitor)
}

func waitChange(t *testing.T, monitor *Monitor) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if monitor.Changed() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("filesystem change notification did not arrive")
}

func TestScanDoesNotFollowLinkOutsideSource(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("outside fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skip("symlink creation unavailable on this platform")
	}
	snapshot, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Directories) != 1 || snapshot.Entries != 2 {
		t.Fatalf("scanner followed source symlink: %+v", snapshot)
	}
}
