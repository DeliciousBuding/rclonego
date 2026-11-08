package session

import (
	"path/filepath"
	"testing"
)

func TestProfileOverrideUsesTheSameDirectoryForLaunchAndDiscovery(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "profile with spaces")
	t.Setenv("RCLONEGO_DATA", profile)
	got, err := DataDir()
	if err != nil || got != profile {
		t.Fatalf("profile directory = %q, err = %v", got, err)
	}
	defaultDirectory, err := DefaultDataDir()
	if err != nil || defaultDirectory == profile {
		t.Fatalf("profile replaced default directory = %q, err = %v", defaultDirectory, err)
	}
}
