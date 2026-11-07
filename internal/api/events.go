package api

import (
	"net/http"
	"strconv"
)

func (a *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		after, err = 0, nil
	}
	if err != nil || after < 0 {
		writeError(w, 400, "after must be a nonnegative event sequence")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := a.app.Events(after, limit)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	next := after
	if len(events) > 0 {
		next = events[len(events)-1].Seq
	}
	writeJSON(w, 200, map[string]any{"events": events, "next_cursor": next, "retention": 10000})
}
