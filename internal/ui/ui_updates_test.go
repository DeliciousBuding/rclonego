package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestUpdateRelaunchWaitsForOwnedWork(t *testing.T) {
	state := core.State{Jobs: []core.Job{{ID: "synthetic", Status: "running"}}}
	if updateBusyReason(state, nil) == "" {
		t.Fatal("running transfer did not defer update relaunch")
	}
	state.Jobs[0].Status = "completed"
	state.OwnedMounts = map[string]int{"synthetic-mount": 42}
	mounts := []core.MountView{{MountConfig: core.MountConfig{ID: "synthetic-mount"}, PID: 42, Running: true}}
	if updateBusyReason(state, mounts) == "" {
		t.Fatal("owned active mount did not defer update relaunch")
	}
	mounts[0].PID = 0
	if updateBusyReason(state, mounts) == "" {
		t.Fatal("owned POSIX mount with an unavailable sampled PID did not defer relaunch")
	}
	mounts[0].Running = false
	if updateBusyReason(state, mounts) != "" {
		t.Fatal("stale stopped ownership blocks updates")
	}
}

func TestUpdateChannelPersistsWithoutDowngradingPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-channel.json")
	if channel, err := loadUpdateChannel(path); err != nil || channel != "stable" {
		t.Fatalf("initial channel=%s err=%v", channel, err)
	}
	for _, channel := range []string{"preview", "stable"} {
		if err := saveUpdateChannel(path, channel); err != nil {
			t.Fatal(err)
		}
		if got, err := loadUpdateChannel(path); err != nil || got != channel {
			t.Fatalf("saved channel=%s got=%s err=%v", channel, got, err)
		}
	}
	if err := saveUpdateChannel(path, "unsupported"); err == nil {
		t.Fatal("unsupported update channel saved")
	}
	if err := os.WriteFile(path, []byte(`{"channel":"unsupported"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if channel, err := loadUpdateChannel(path); err == nil || channel != "stable" {
		t.Fatalf("invalid channel policy=%s err=%v", channel, err)
	}
}

func TestVersionNamedPortableExecutableUsesManualUpdate(t *testing.T) {
	if selfUpdateExecutableSupported("rclonego-1.0.0-windows-amd64.exe", "windows") {
		t.Fatal("version-named download would relaunch its old executable after MyGo installed RcloneGo.exe")
	}
	if !selfUpdateExecutableSupported("RcloneGo.exe", "windows") {
		t.Fatal("official executable name cannot use MyGo's update archive")
	}
}
