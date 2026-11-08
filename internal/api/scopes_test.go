package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func scopedFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := core.OpenStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	owner := strings.Repeat("o", 64)
	server, err := New(core.NewApp(store, "", dir), owner)
	if err != nil {
		t.Fatal(err)
	}
	return server, owner, dir
}

func apiRequest(t *testing.T, server *Server, bearer, method, endpoint string, value any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if value != nil {
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, endpoint, strings.NewReader(string(raw)))
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set(headerGuard, "1")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func issueTaskToken(t *testing.T, server *Server, owner string, paths []string, operations ...string) TokenResponse {
	t.Helper()
	recorder := apiRequest(t, server, owner, http.MethodPost, "/api/v1/tokens/tasks",
		TokenRequest{TTLSeconds: 60, Paths: paths, Operations: operations})
	if recorder.Code != 201 {
		t.Fatalf("issue grant status %d", recorder.Code)
	}
	var result TokenResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTaskGrantEnforcesOperationsPathBoundariesAndRevocation(t *testing.T) {
	server, owner, dir := scopedFixture(t)
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "target")
	for _, path := range []string{source, target} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	grant := issueTaskToken(t, server, owner, []string{source, target}, "copy")
	request := core.JobCreateRequest{Name: "Allowed", Operation: "copy", Source: source, Destination: target}
	recorder := apiRequest(t, server, grant.Token, "POST", "/api/v1/jobs", request)
	if recorder.Code != 202 {
		t.Fatalf("allowed task status %d: %s", recorder.Code, recorder.Body.String())
	}
	for _, test := range []struct {
		name, source, destination, operation string
	}{
		{"sibling", source + "-outside", target, "copy"},
		{"parent", filepath.Join(source, ".."), target, "copy"},
		{"ungranted operation", source, target, "sync"},
		{"relative path", "source", target, "copy"},
		{"inline backend", ":local:" + source, target, "copy"},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := len(server.app.Snapshot().Jobs)
			value := core.JobCreateRequest{Name: test.name, Source: test.source, Destination: test.destination, Operation: test.operation}
			recorder := apiRequest(t, server, grant.Token, "POST", "/api/v1/jobs", value)
			if recorder.Code != 403 || len(server.app.Snapshot().Jobs) != before {
				t.Fatalf("scope bypass status %d changed queue", recorder.Code)
			}
		})
	}
	for _, endpoint := range []string{"/api/v1/tokens/read", "/api/v1/queue/pause", "/api/v1/accounts", "/api/v1/runs"} {
		if recorder := apiRequest(t, server, grant.Token, "POST", endpoint, map[string]any{}); recorder.Code != 403 {
			t.Fatalf("configuration endpoint %s status %d", endpoint, recorder.Code)
		}
	}
	for _, endpoint := range []string{"/api/v1/events", "/api/v1/runtime", "/api/v1/workflow-definitions", "/api/v1/automations",
		"/api/v1/remotes/storage/quota", "/api/v1/remotes/storage/info", "/api/v1/storage/usage?path=storage:"} {
		if recorder := apiRequest(t, server, grant.Token, "GET", endpoint, nil); recorder.Code != 403 {
			t.Fatalf("global state endpoint %s status %d", endpoint, recorder.Code)
		}
	}
	if recorder := apiRequest(t, server, owner, "POST", "/api/v1/tokens/"+grant.ID+"/revoke", nil); recorder.Code != 200 {
		t.Fatalf("revoke status %d", recorder.Code)
	}
	if recorder := apiRequest(t, server, grant.Token, "GET", "/api/v1/jobs", nil); recorder.Code != 401 {
		t.Fatalf("revoked token status %d", recorder.Code)
	}
}

func TestTaskGrantCannotRetrieveOutsideJobByIdempotencyKey(t *testing.T) {
	server, owner, dir := scopedFixture(t)
	allowed, outside := filepath.Join(dir, "allowed"), filepath.Join(dir, "outside")
	for _, path := range []string{allowed, outside} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	key := "outside-job-key"
	existing, err := server.app.AddJob(core.JobCreateRequest{Name: "Outside", Operation: "copy", Source: outside, Destination: filepath.Join(outside, "copy"), IdempotencyKey: key})
	if err != nil {
		t.Fatal(err)
	}
	grant := issueTaskToken(t, server, owner, []string{allowed}, "copy")
	recorder := apiRequest(t, server, grant.Token, "POST", "/api/v1/jobs",
		core.JobCreateRequest{Name: "Allowed", Operation: "copy", Source: allowed, Destination: filepath.Join(allowed, "copy"), IdempotencyKey: key})
	if recorder.Code != 403 || strings.Contains(recorder.Body.String(), existing.ID) {
		t.Fatal("idempotency key exposed a job outside the grant")
	}
	recorder = apiRequest(t, server, grant.Token, "GET", "/api/v1/jobs/"+existing.ID, nil)
	if recorder.Code != 403 {
		t.Fatal("direct job lookup escaped the grant")
	}
	recorder = apiRequest(t, server, grant.Token, "GET", "/api/v1/jobs", nil)
	if recorder.Code != 200 || strings.Contains(recorder.Body.String(), existing.ID) || strings.Contains(recorder.Body.String(), outside) {
		t.Fatal("job listing exposed outside jobs")
	}
}

func TestTaskGrantResolvesSymlinksAndRejectsRemoteTraversal(t *testing.T) {
	server, owner, dir := scopedFixture(t)
	root, outside := filepath.Join(dir, "root"), filepath.Join(dir, "outside")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	grant := issueTaskToken(t, server, owner, []string{root, "example-remote:archive"}, "copy")
	var stored tokenGrant
	for _, value := range server.readTokens {
		if value.ID == grant.ID {
			stored = value
		}
	}
	for _, candidate := range []string{"example-remote:archive-other", "example-remote:archive/../outside", "another-remote:archive", "example-remote:/archive"} {
		if stored.allowsPath(candidate) {
			t.Fatalf("remote path escaped the grant: %s", candidate)
		}
	}
	if !stored.allowsPath("example-remote:archive/folder/file.txt") {
		t.Fatal("remote child path should match its grant")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err == nil {
		if stored.allowsPath(filepath.Join(link, "not-yet-created.txt")) {
			t.Fatal("symlink ancestor escaped the grant")
		}
	}
	server.tokensMu.Lock()
	for hash, value := range server.readTokens {
		value.ExpiresAt = time.Now().Add(-time.Second)
		server.readTokens[hash] = value
	}
	server.tokensMu.Unlock()
	if recorder := apiRequest(t, server, grant.Token, "GET", "/api/v1/jobs", nil); recorder.Code != 401 {
		t.Fatal("expired token remained active")
	}
}

func TestScopedBrowseDoesNotExposeParentOutsideGrant(t *testing.T) {
	server, owner, dir := scopedFixture(t)
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	grant := issueTaskToken(t, server, owner, []string{root}, "copy")
	recorder := apiRequest(t, server, grant.Token, "GET", "/api/v1/browse?path="+url.QueryEscape(root), nil)
	if recorder.Code != 200 {
		t.Fatalf("browse status %d", recorder.Code)
	}
	var listing core.BrowseResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if listing.Parent != "" {
		t.Fatal("browse exposed parent outside the grant")
	}
}
