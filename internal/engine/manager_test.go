package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidArchivePreservesSelectedEngine(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{Dir: filepath.Join(dir, "engines")}
	expected := Installation{Version: "synthetic-old", Path: "synthetic-engine"}
	if err := manager.Select(expected); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "invalid.zip")
	os.WriteFile(archive, []byte("invalid"), 0o600)
	if _, err := manager.Install(context.Background(), archive); err == nil {
		t.Fatal("accepted invalid archive")
	}
	actual, err := manager.Selected()
	if err != nil || actual != expected {
		t.Fatal("failed install changed selection")
	}
}
