package cli

import (
	"testing"
)

func TestFilesCLIUsesExplicitPreviewAndQueuedActions(t *testing.T) {
	method, path, _, _, err := cliRequest([]string{"files", "preview", "example:a b.txt"})
	if err != nil || method != "GET" || path != "/api/v1/files/preview?path=example%3Aa+b.txt" {
		t.Fatalf("preview request: %s %s %v", method, path, err)
	}
	method, path, _, _, err = cliRequest([]string{"mounts", "targets"})
	if err != nil || method != "GET" || path != "/api/v1/mounts/targets" {
		t.Fatal("mount targets request failed")
	}
	for _, args := range [][]string{{"files"}, {"files", "delete"}, {"files", "purge", "--file", "missing.json"}} {
		if _, _, _, _, err = cliRequest(args); err == nil {
			t.Fatalf("invalid file command accepted: %v", args)
		}
	}
	if !IsCLI([]string{"files", "preview", "example:a.txt"}) {
		t.Fatal("file CLI launched GUI")
	}
}
