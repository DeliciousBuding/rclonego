package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func TestLocalAPIProtectsReadAndWriteAndRejectsOrigins(t *testing.T) {
	dir := t.TempDir()
	store, err := core.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	token := strings.Repeat("a", 64)
	server, err := New(core.NewApp(store, "", dir), token)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, method, token, origin string
		status                      int
	}{
		{"/api/v1/health", "GET", "", "", 200},
		{"/api/v1/jobs", "GET", "", "", 401},
		{"/api/v1/jobs", "GET", "wrong", "", 401},
		{"/api/v1/jobs", "GET", token, "", 200},
		{"/api/v1/jobs", "GET", token, "https://example.com", 403},
		{"/api/v1/queue/pause", "POST", "", "", 401},
	} {
		t.Run(test.method+test.path+test.token[:min(5, len(test.token))]+test.origin, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"paused":true}`))
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status %d, expected %d", recorder.Code, test.status)
			}
		})
	}
}
