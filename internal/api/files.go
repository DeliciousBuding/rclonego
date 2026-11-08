package api

import (
	"net/http"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleFilePreview(w http.ResponseWriter, r *http.Request) {
	result, err := a.app.PreviewFile(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		writeFailure(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *Server) handleMountTargets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.app.MountTargets())
}

func (a *Server) handleFileAction(w http.ResponseWriter, r *http.Request) {
	var request core.FileActionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := a.app.AddFileAction(request)
	if err != nil {
		a.writeAppFailure(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
