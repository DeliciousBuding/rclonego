package api

import (
	"net/http"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, a.app.Snapshot().Handoff)
		return
	}
	if r.PathValue("action") == "return" {
		result, err := a.app.ReturnHandoff(r.Context())
		if err != nil {
			writeFailure(w, 400, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	var request core.HandoffRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeFailure(w, 400, err)
		return
	}
	if r.PathValue("action") == "preview" {
		result, err := a.app.PreviewHandoff(r.Context(), request)
		if err != nil {
			writeFailure(w, 400, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if r.PathValue("action") != "begin" {
		writeError(w, 400, "use preview, begin or return")
		return
	}
	result, err := a.app.BeginHandoff(r.Context(), request)
	if err != nil {
		writeFailure(w, 400, err)
		return
	}
	writeJSON(w, 200, result)
}
