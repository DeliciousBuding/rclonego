package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var contract []byte

func Contract() []byte { return append([]byte(nil), contract...) }

func (a *Server) handleSchema(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(contract)
}
