package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestCLIBrowseUsesSharedDirectoryServiceAndPagination(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	directory := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "file.txt"), []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := Run([]string{"browse", directory, "--directories", "--limit", "1", "--json", "--api", server.URL}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var listing core.BrowseResult
	if err := json.Unmarshal(out.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "alpha" || !listing.HasMore || listing.NextOffset != 1 {
		t.Fatalf("unexpected directory page: %+v", listing)
	}
}

func TestCLIEventWatchReturnsNumericCursorAndActionableErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/events/watch" || r.URL.Query().Get("after") != "9" || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("wrong event watch request %s", r.URL)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "id: 10\nevent: event\ndata: {\"seq\":10,\"subject\":\"synthetic\",\"kind\":\"completed\",\"occurred_at\":\"2026-01-01T00:00:00Z\"}\n\n")
	}))
	defer server.Close()
	var out, stderr bytes.Buffer
	if code := Run([]string{"events", "watch", "--after", "9", "--api", server.URL, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var frame struct {
		Type   string          `json:"type"`
		Cursor int64           `json:"cursor"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &frame); err != nil || frame.Cursor != 10 || frame.Type != "event" {
		t.Fatalf("invalid event watch output: %s", out.String())
	}
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "id: 9\nevent: error\ndata: {\"code\":\"cursor_expired\",\"message\":\"refresh task state\",\"next_action\":\"restart with after=0\",\"retryable\":false}\n\n")
	}))
	defer errorServer.Close()
	out.Reset()
	if code := Run([]string{"events", "watch", "--api", errorServer.URL, "--json"}, &out, &stderr); code == 0 ||
		!strings.Contains(stderr.String(), `"code":"cursor_expired"`) || !strings.Contains(stderr.String(), `"http_status":410`) {
		t.Fatal("event error did not retain structured recovery information")
	}
}

func TestCLIAutomationFileUsesConfigurationOnly(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "automation.json")
	body, err := json.Marshal(map[string]any{
		"name": "Synthetic upload", "kind": "watch", "enabled": false,
		"source": source, "destination": filepath.Join(directory, "target")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := Run([]string{"automations", "save", "--file", filename, "--api", server.URL, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var saved core.Automation
	if err := json.Unmarshal(out.Bytes(), &saved); err != nil || saved.ID == "" || saved.Enabled {
		t.Fatal("CLI did not preserve the disabled automation configuration")
	}
	out.Reset()
	if code := Run([]string{"automations", "delete", saved.ID, "--api", server.URL}, &out, &stderr); code != 0 ||
		len(app.ListAutomations()) != 0 {
		t.Fatal("CLI did not delete the saved automation")
	}
}

func TestCLIOfflineCopyUsesPersistentQueueAndLogCleanup(t *testing.T) {
	app := testApp(t)
	if err := app.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(testHandler(t, app))
	defer server.Close()
	directory := t.TempDir()
	filename := filepath.Join(directory, "copy.json")
	request := core.OfflineCopyRequest{
		Name: "Synthetic offline copy", Source: "fixture:archive",
		Destination: filepath.Join(directory, "destination"), IdempotencyKey: "synthetic-offline",
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsCLI([]string{"offline", "--file", filename}) || !IsCLI([]string{"logs", "cleanup"}) {
		t.Fatal("Offline-copy and log commands would open the GUI")
	}
	for range 2 {
		var out, stderr bytes.Buffer
		if code := Run([]string{"offline", "--file", filename, "--api", server.URL, "--json"}, &out, &stderr); code != 0 {
			t.Fatal(stderr.String())
		}
		var job core.Job
		if err := json.Unmarshal(out.Bytes(), &job); err != nil || job.Operation != "copy" || job.Status != "pending" {
			t.Fatalf("Offline copy did not join the paused queue: %s / %v", out.String(), err)
		}
	}
	if len(app.Snapshot().Jobs) != 1 {
		t.Fatal("Idempotent offline submission created another task")
	}
	var out, stderr bytes.Buffer
	if code := Run([]string{"logs", "cleanup", "--api", server.URL, "--json"}, &out, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.OK {
		t.Fatal("Log cleanup did not confirm retention")
	}
}
