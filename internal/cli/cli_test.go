package cli

import (
	"bytes"
	"encoding/json"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIJobCreationUsesSharedAPIAndIdempotency(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	args := []string{"jobs", "add", "--api", server.URL, "--json", "--name", "照片备份", "--source", "src", "--destination", "dst", "--idempotency-key", "cli-42"}
	var first, second, stderr bytes.Buffer
	if code := Run(args, &first, &stderr); code != 0 {
		t.Fatalf("CLI failed: %s", stderr.String())
	}
	if code := Run(args, &second, &stderr); code != 0 {
		t.Fatalf("CLI retry failed: %s", stderr.String())
	}
	var a, b core.Job
	if err := json.Unmarshal(first.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID || a.Name != "照片备份" || a.Transfers != 2 || a.Checkers != 4 {
		t.Fatalf("unexpected created job: %+v / %+v", a, b)
	}
}

func TestCLIThemePersistsAndRejectsInvalidInput(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	var out, stderr bytes.Buffer
	if code := Run([]string{"theme", "dark", "--api", server.URL}, &out, &stderr); code != 0 {
		t.Fatalf("theme failed: %s", stderr.String())
	}
	if app.Snapshot().Appearance != "dark" || !strings.Contains(out.String(), "dark") {
		t.Fatal("theme did not persist")
	}
	if code := Run([]string{"theme", "rainbow", "--api", server.URL}, &out, &stderr); code == 0 {
		t.Fatal("invalid appearance accepted")
	}
	if app.Snapshot().Appearance != "dark" {
		t.Fatal("invalid input changed settings")
	}
}

func TestCLIRejectsUnsafeSyncAndMalformedFiles(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	var out, stderr bytes.Buffer
	args := []string{"jobs", "add", "--api", server.URL, "--name", "sync", "--source", "src", "--destination", "dst", "--operation", "sync"}
	if code := Run(args, &out, &stderr); code == 0 {
		t.Fatal("sync deletion accepted without explicit permission")
	}
	if len(app.Snapshot().Jobs) != 0 {
		t.Fatal("unsafe job created")
	}
	file := filepath.Join(t.TempDir(), "task.json")
	if err := os.WriteFile(file, []byte(`{"name":"copy","source":"src","destination":"dst"} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseJobCLI([]string{"--file", file}); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
	if _, _, err := parseJobCLI([]string{"--file", file, "--name", "override"}); err == nil {
		t.Fatal("conflicting file flags accepted")
	}
}

func TestCLIStatusHumanOutputAndRemoteBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/overview" {
			t.Errorf("wrong endpoint %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": "rclone test", "remote_count": 2, "jobs": []core.Job{},
			"activity": core.ActivityStatus{Available: true, Stats: &core.RCStats{Bytes: 1 << 30, TotalBytes: 2 << 30, Speed: 2 << 20, Transferring: []core.RCTransfer{{Name: "sample.bin"}}}},
		})
	}))
	defer server.Close()
	var out, stderr bytes.Buffer
	if code := Run([]string{"status", "--api", server.URL}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(out.String(), "50.0%") || !strings.Contains(out.String(), "2.00 MiB/s") {
		t.Fatal(out.String())
	}
	out.Reset()
	if code := Run([]string{"status", "--api", "http://example.com", "--json"}, &out, &stderr); code == 0 {
		t.Fatal("CLI accepted non-local API")
	}
}
