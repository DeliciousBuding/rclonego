package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
	"github.com/DeliciousBuding/rclonego/internal/persistence"
)

func TestEventsRejectExpiredCursorAndKeepBootstrapReadable(t *testing.T) {
	server, owner, dir := scopedFixture(t)
	db, err := persistence.Open(filepath.Join(dir, "state", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	events := make([]persistence.Event, 10005)
	for i := range events {
		events[i] = persistence.Event{Subject: "synthetic-task", Kind: "pending"}
	}
	if err := db.Save(nil, nil, events); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/api/v1/events?after=1", "/api/v1/events/watch?after=1"} {
		recorder := apiRequest(t, server, owner, "GET", endpoint, nil)
		if recorder.Code != 410 || !strings.Contains(recorder.Body.String(), `"code":"cursor_expired"`) {
			t.Fatalf("expired cursor status %d: %s", recorder.Code, recorder.Body.String())
		}
	}
	recorder := apiRequest(t, server, owner, "GET", "/api/v1/events?after=0&limit=2", nil)
	var page EventsResponse
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &page) != nil ||
		len(page.Events) != 2 || page.EarliestCursor != 5 || page.NextCursor != 7 {
		t.Fatalf("bootstrap cursor did not expose the retained boundary: %s", recorder.Body.String())
	}
	for _, endpoint := range []string{"/api/v1/events?limit=bad", "/api/v1/events?limit=501", "/api/v1/events?after=-1"} {
		if recorder := apiRequest(t, server, owner, "GET", endpoint, nil); recorder.Code != 400 {
			t.Fatalf("invalid query accepted: %s", endpoint)
		}
	}
}

func readSSEFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}

func TestEventWatchStreamsStoredSequenceAndStopsAfterRevocation(t *testing.T) {
	server, owner, _ := scopedFixture(t)
	tokenResponse := apiRequest(t, server, owner, "POST", "/api/v1/tokens/read", TokenRequest{TTLSeconds: 60})
	var grant TokenResponse
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &grant); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", httpServer.URL+"/api/v1/events/watch", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+grant.Token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("watch status %d", response.StatusCode)
	}
	reader := bufio.NewReader(response.Body)
	if frame := readSSEFrame(t, reader); !strings.Contains(frame, "event: cursor") {
		t.Fatalf("missing initial cursor: %s", frame)
	}
	job, err := server.app.AddJob(core.JobCreateRequest{Name: "Synthetic", Operation: "copy", Source: "source", Destination: "destination"})
	if err != nil {
		t.Fatal(err)
	}
	if frame := readSSEFrame(t, reader); !strings.Contains(frame, "event: event") ||
		!strings.Contains(frame, job.ID) || !strings.Contains(frame, "id: 1") {
		t.Fatalf("missing persisted event: %s", frame)
	}
	if recorder := apiRequest(t, server, owner, "POST", "/api/v1/tokens/"+grant.ID+"/revoke", nil); recorder.Code != 200 {
		t.Fatal("could not revoke the observer")
	}
	if frame := readSSEFrame(t, reader); !strings.Contains(frame, "event: error") || !strings.Contains(frame, "authentication_required") {
		t.Fatalf("revoked observer did not receive a closing error: %s", frame)
	}
}
