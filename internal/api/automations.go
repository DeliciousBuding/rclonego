package api

import (
	"net/http"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

type AutomationRequest struct {
	ID              string                `json:"id,omitempty"`
	Name            string                `json:"name"`
	Kind            string                `json:"kind"`
	Enabled         bool                  `json:"enabled"`
	Source          string                `json:"source,omitempty"`
	Destination     string                `json:"destination,omitempty"`
	WorkflowID      string                `json:"workflow_id,omitempty"`
	Settings        core.TransferSettings `json:"settings,omitempty"`
	IntervalSeconds int64                 `json:"interval_seconds,omitempty"`
	DebounceSeconds int                   `json:"debounce_seconds,omitempty"`
	StableSeconds   int                   `json:"stable_seconds,omitempty"`
	RescanSeconds   int                   `json:"rescan_seconds,omitempty"`
}

func (a *Server) handleAutomations(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, a.app.ListAutomations())
		return
	}
	var request AutomationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if id := r.PathValue("id"); id != "" {
		if request.ID != "" && request.ID != id {
			writeError(w, 400, "automation ID in the body must match the URL")
			return
		}
		request.ID = id
	}
	saved, err := a.app.SaveAutomation(core.Automation{
		ID: request.ID, Name: request.Name, Kind: request.Kind, Enabled: request.Enabled,
		Source: request.Source, Destination: request.Destination, WorkflowID: request.WorkflowID,
		Settings: request.Settings, IntervalSeconds: request.IntervalSeconds,
		DebounceSeconds: request.DebounceSeconds, StableSeconds: request.StableSeconds, RescanSeconds: request.RescanSeconds,
	})
	if err != nil {
		a.writeAppFailure(w, 400, err)
		return
	}
	writeJSON(w, 201, saved)
}

func (a *Server) handleAutomationAction(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.Method == http.MethodDelete {
		err = a.app.DeleteAutomation(r.PathValue("id"))
	} else {
		err = a.app.TriggerAutomation(r.PathValue("id"))
	}
	if err != nil {
		a.writeAppFailure(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
