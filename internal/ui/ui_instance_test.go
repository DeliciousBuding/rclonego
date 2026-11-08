package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopActivationKeepsProfilesSeparateAndResolvesAliases(t *testing.T) {
	directory := t.TempDir()
	if instanceName(directory) == instanceName(filepath.Join(directory, "other")) {
		t.Fatal("independent profiles would activate the same window")
	}
	if instanceName(directory) != instanceName(filepath.Join(directory, ".")) {
		t.Fatal("the same profile through an equivalent path would create another window")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(directory, alias); err == nil && instanceName(directory) != instanceName(alias) {
		t.Fatal("a profile symlink would create another window")
	}
}
