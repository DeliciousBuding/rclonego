package api

import "net/http"

func (a *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.app.Doctor(r.Context()))
}
