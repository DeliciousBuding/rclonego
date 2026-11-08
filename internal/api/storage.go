package api

import "net/http"

func (a *Server) handleRemoteQuota(w http.ResponseWriter, r *http.Request) {
	result, err := a.app.RemoteQuota(r.Context(), r.PathValue("name"))
	if err != nil {
		writeFailure(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *Server) handleStorageInfo(w http.ResponseWriter, r *http.Request) {
	result, err := a.app.StorageInfo(r.Context(), r.PathValue("name"))
	if err != nil {
		writeFailure(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *Server) handleDirectoryUsage(w http.ResponseWriter, r *http.Request) {
	result, err := a.app.DirectoryUsage(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		writeFailure(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
