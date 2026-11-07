package api

import (
	"encoding/json"
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

func TestDelegatedTokenCanObserveButCannotMutate(t *testing.T) {
	dir := t.TempDir()
	store, err := core.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rootToken := strings.Repeat("a", 64)
	server, err := New(core.NewApp(store, "", dir), rootToken)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/api/v1/tokens/read", strings.NewReader(`{"ttl_seconds":60}`))
	request.Header.Set("Authorization", "Bearer "+rootToken)
	request.Header.Set(headerGuard, "1")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != 201 {
		t.Fatalf("create token: %d %s", recorder.Code, recorder.Body)
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		Method, Path string
		Status       int
	}{
		{"GET", "/api/v1", 200}, {"GET", "/api/v1/events", 200}, {"POST", "/api/v1/queue/pause", 403},
		{"POST", "/api/v1/tokens/read", 403},
	} {
		request := httptest.NewRequest(check.Method, check.Path, strings.NewReader(`{"paused":true}`))
		request.Header.Set("Authorization", "Bearer "+result.Token)
		request.Header.Set(headerGuard, "1")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != check.Status {
			t.Fatalf("%s %s: %d", check.Method, check.Path, recorder.Code)
		}
	}
}
