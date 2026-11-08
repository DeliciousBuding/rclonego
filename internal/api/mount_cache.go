package api

import (
	"net/http"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleMountCache(w http.ResponseWriter, r *http.Request) {
	view, err := a.app.MountCache(r.Context(), r.PathValue("id"))
	if err != nil {
		writeFailure(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// Offline copies and log retention are owner operations. Delegated task
// grants can submit regular scoped copy jobs through the existing job API.
func (a *Server) handleOfflineCopy(w http.ResponseWriter, r *http.Request) {
	var request core.OfflineCopyRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeFailure(w, http.StatusBadRequest, err)
		return
	}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}
	job, err := a.app.AddOfflineCopy(request)
	if err != nil {
		writeFailure(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *Server) handleCleanupLogs(w http.ResponseWriter, _ *http.Request) {
	if err := a.app.CleanupLogs(); err != nil {
		writeFailure(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
