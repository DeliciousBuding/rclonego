package api

import (
	"context"
	"net/http"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleEngine(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, a.app.EngineStatus(r.Context()))
		return
	}
	var input struct {
		Path    string `json:"path"`
		Archive string `json:"archive"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if r.PathValue("action") == "install" {
		result, err := a.app.InstallEngine(ctx, input.Archive)
		if err != nil {
			writeFailure(w, 400, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if r.PathValue("action") != "select" || input.Path == "" {
		writeError(w, 400, "use install or select with a path")
		return
	}
	result, err := a.app.SelectEngine(ctx, input.Path)
	if err != nil {
		writeFailure(w, 400, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	result, err := a.app.Providers(r.Context())
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (a *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var input core.JobCreateRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if input.Operation == "" {
		input.Operation = "copy"
	}
	if !authorizeJobRequest(w, r, input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	result, err := a.app.PreviewJob(ctx, input)
	if err != nil {
		writeFailure(w, 400, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		result, err := a.app.Account(r.PathValue("id"))
		if err != nil {
			writeError(w, 404, err.Error())
			return
		}
		writeJSON(w, 200, result)
		return
	}
	switch r.PathValue("action") {
	case "cancel":
		if err := a.app.CancelAccount(r.PathValue("id")); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	case "answer":
		var input struct {
			Answer string `json:"answer"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if err := a.app.AnswerAccount(r.PathValue("id"), input.Answer); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 202, map[string]bool{"ok": true})
		return
	case "":
	default:
		writeError(w, 404, "unknown account action")
		return
	}
	var input core.AccountRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	result, err := a.app.BeginAccount(r.Context(), input)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, result)
}
