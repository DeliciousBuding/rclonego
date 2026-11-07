package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func New(app *core.App, token string) (*Server, error) {
	if len(token) < 32 {
		return nil, errors.New("local API requires a random authentication token")
	}
	return &Server{app: app, token: token}, nil
}

func (a *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
				writeError(w, http.StatusUnauthorized, "local API token required")
				return
			}
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			writeError(w, http.StatusForbidden, "browser origins are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}
