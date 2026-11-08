package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/persistence"
)

type EventsResponse struct {
	Events         []persistence.Event `json:"events"`
	NextCursor     int64               `json:"next_cursor"`
	EarliestCursor int64               `json:"earliest_cursor"`
	Retention      int                 `json:"retention"`
}

var errCursorExpired = errors.New("event cursor is older than retained history")

func eventQuery(r *http.Request) (after int64, limit int, err error) {
	after, limit = 0, 100
	if value := r.URL.Query().Get("after"); value != "" {
		after, err = strconv.ParseInt(value, 10, 64)
	}
	if err != nil || after < 0 {
		return 0, 0, errors.New("after must be a nonnegative event sequence")
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
	}
	if err != nil || limit < 1 || limit > 500 {
		return 0, 0, errors.New("limit must be between 1 and 500")
	}
	return after, limit, nil
}

func (a *Server) eventPage(after int64, limit int) (EventsResponse, error) {
	oldest, err := a.app.Events(0, 1)
	if err != nil {
		return EventsResponse{}, err
	}
	result := EventsResponse{Events: []persistence.Event{}, NextCursor: after, Retention: 10000}
	if len(oldest) != 0 {
		result.EarliestCursor = oldest[0].Seq - 1
	}
	if after != 0 && after < result.EarliestCursor {
		return result, errCursorExpired
	}
	result.Events, err = a.app.Events(after, limit)
	if err != nil {
		return result, err
	}
	if len(result.Events) != 0 {
		// The retention boundary may advance between the two bounded reads.
		if after != 0 && result.Events[0].Seq > after+1 {
			return result, errCursorExpired
		}
		result.NextCursor = result.Events[len(result.Events)-1].Seq
	}
	return result, nil
}

func writeEventError(w http.ResponseWriter, err error, page EventsResponse) {
	if errors.Is(err, errCursorExpired) {
		writeJSON(w, http.StatusGone, map[string]any{
			"error": err.Error(), "message": err.Error(), "code": "cursor_expired",
			"retryable": false, "next_action": "refresh current task state, then restart events with after=0",
			"earliest_cursor": page.EarliestCursor,
		})
		return
	}
	writeError(w, 500, err.Error())
}

func (a *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	after, limit, err := eventQuery(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	page, err := a.eventPage(after, limit)
	if err != nil {
		writeEventError(w, err, page)
		return
	}
	writeJSON(w, 200, page)
}

// Watch uses SSE with a sequence cursor. It reads bounded pages from the same
// authoritative events table as polling; reconnect never changes task state.
func (a *Server) handleWatchEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("after") == "" && r.Header.Get("Last-Event-ID") != "" {
		query := r.URL.Query()
		query.Set("after", r.Header.Get("Last-Event-ID"))
		r.URL.RawQuery = query.Encode()
	}
	after, limit, err := eventQuery(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	page, err := a.eventPage(after, limit)
	if err != nil {
		writeEventError(w, err, page)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming is unavailable on this transport")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(kind string, id int64, value any) bool {
		raw, err := json.Marshal(value)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, kind, raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send("cursor", after, map[string]any{"next_cursor": after, "earliest_cursor": page.EarliestCursor, "retention": 10000}) {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		if grant := delegatedGrant(r); grant != nil && !a.grantActive(*grant) {
			_ = send("error", after, map[string]any{"code": "authentication_required", "message": "delegated token expired or was revoked", "retryable": false})
			return
		}
		for _, event := range page.Events {
			if !send("event", event.Seq, event) {
				return
			}
			after = event.Seq
		}
		if len(page.Events) == limit {
			page, err = a.eventPage(after, limit)
		} else {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
				continue
			case <-ticker.C:
			}
			page, err = a.eventPage(after, limit)
		}
		if err != nil {
			code := "internal_error"
			if errors.Is(err, errCursorExpired) {
				code = "cursor_expired"
			}
			_ = send("error", after, map[string]any{"code": code, "message": err.Error(),
				"earliest_cursor": page.EarliestCursor, "retryable": false,
				"next_action": "refresh current task state, then reconnect with after=0"})
			return
		}
	}
}

func (a *Server) grantActive(want tokenGrant) bool {
	a.tokensMu.Lock()
	defer a.tokensMu.Unlock()
	for _, grant := range a.readTokens {
		if grant.ID == want.ID && time.Now().Before(grant.ExpiresAt) {
			return true
		}
	}
	return false
}
