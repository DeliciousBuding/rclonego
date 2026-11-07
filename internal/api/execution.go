package api

import (
	"net/http"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleDefinitions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, a.app.Snapshot().Workflows)
		return
	}
	var definition core.WorkflowDefinition
	if err := decodeJSON(w, r, &definition); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	saved, err := a.app.SaveWorkflow(definition)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, saved)
}

func (a *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, a.app.Snapshot().Runs)
		return
	}
	var request struct {
		WorkflowID     string `json:"workflow_id"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		if request.IdempotencyKey != "" && request.IdempotencyKey != key {
			writeError(w, 400, "conflicting idempotency keys")
			return
		}
		request.IdempotencyKey = key
	}
	run, err := a.app.StartWorkflow(request.WorkflowID, request.IdempotencyKey)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, run)
}

func (a *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	report, err := a.app.ReportRun(r.PathValue("id"))
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, report)
}
