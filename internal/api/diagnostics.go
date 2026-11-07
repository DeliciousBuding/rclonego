package api

import "net/http"

func (a *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.app.Diagnostics(r.Context()))
}
