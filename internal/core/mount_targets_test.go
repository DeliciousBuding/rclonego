package core

import (
	"path/filepath"
	"testing"
)

func TestMountedTaskSharesItsRemoteOverlapBoundary(t *testing.T) {
	root := t.TempDir()
	mounts := []MountView{{MountConfig: MountConfig{Drive: root, Remote: "fixture:archive"}, External: true, DriveAvailable: true}}
	if got := mountedTransferPath(filepath.Join(root, "Photos", "a.txt"), mounts); got != "fixture:archive/Photos/a.txt" {
		t.Fatalf("mounted route is not canonical: %s", got)
	}
	if got := mountedTransferPath(root, mounts); got != "fixture:archive" {
		t.Fatal("mount root route lost")
	}
	other := filepath.Join(filepath.Dir(root), "other")
	if got := mountedTransferPath(other, mounts); got != other {
		t.Fatal("unrelated local path routed into mount")
	}
	app := testApp(t)
	app.runtime.mu.Lock()
	app.runtime.snapshot.Mounts = mounts
	app.runtime.mu.Unlock()
	state := State{Handoff: &LegacyHandoff{Status: "waiting"},
		Jobs: []Job{{Origin: "data-ops-console", Source: "fixture:archive/Photos", Destination: "other:Photos"}}}
	job := Job{Source: filepath.Join(root, "Photos", "a.txt"), Destination: filepath.Join(root, "Photos", "b.txt")}
	if !app.dispatchBlocked(job, state) {
		t.Fatal("mounted file change bypassed active legacy handoff")
	}
}
