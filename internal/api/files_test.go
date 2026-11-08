package api

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestFileAPIKeepsReadAndTaskDelegationBoundaries(t *testing.T) {
	dir := t.TempDir()
	store, err := core.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := strings.Repeat("a", 64)
	app := core.NewApp(store, "", dir)
	server, err := New(app, owner)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "sample.txt")
	os.WriteFile(file, []byte("synthetic text"), 0o600)
	recorder := apiRequest(t, server, owner, "GET", "/api/v1/files/preview?path="+url.QueryEscape(file), nil)
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), "synthetic text") {
		t.Fatalf("file preview failed: %s", recorder.Body.String())
	}
	read := apiRequest(t, server, owner, "POST", "/api/v1/tokens/read", map[string]any{"ttl_seconds": 60})
	var grant TokenResponse
	json.Unmarshal(read.Body.Bytes(), &grant)
	action := core.FileActionRequest{Operation: "delete", Path: file, ConfirmDelete: true}
	if recorder := apiRequest(t, server, grant.Token, "POST", "/api/v1/files/actions", action); recorder.Code != 403 {
		t.Fatal("read token queued deletion")
	}
	task := apiRequest(t, server, owner, "POST", "/api/v1/tokens/tasks", map[string]any{
		"ttl_seconds": 60, "operations": []string{"copy"}, "paths": []string{dir},
	})
	json.Unmarshal(task.Body.Bytes(), &grant)
	if recorder := apiRequest(t, server, grant.Token, "POST", "/api/v1/files/actions", action); recorder.Code != 403 {
		t.Fatal("transfer grant gained file deletion")
	}
	if recorder := apiRequest(t, server, owner, "POST", "/api/v1/files/actions", action); recorder.Code != 202 {
		t.Fatalf("owner action failed: %s", recorder.Body.String())
	}
	if len(app.Snapshot().Jobs) != 1 || app.Snapshot().Jobs[0].Operation != "deletefile" {
		t.Fatal("file action bypassed shared queue")
	}
	request := httptest.NewRequest("GET", "/api/v1/mounts/targets", nil)
	request.Header.Set("Authorization", "Bearer "+owner)
	targets := httptest.NewRecorder()
	server.Handler().ServeHTTP(targets, request)
	if targets.Code != 200 || !strings.Contains(targets.Body.String(), "dependency") {
		t.Fatal("mount targets missing")
	}
}
