package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

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
			authorization := r.Header.Get("Authorization")
			if !strings.HasPrefix(authorization, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "local API bearer token required")
				return
			}
			token := strings.TrimPrefix(authorization, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) != 1 {
				hash := sha256.Sum256([]byte(token))
				a.tokensMu.Lock()
				grant, exists := a.readTokens[hash]
				a.tokensMu.Unlock()
				if !exists || time.Now().After(grant.ExpiresAt) {
					writeError(w, http.StatusUnauthorized, "local API token required")
					return
				}
				if grant.Scope == "read" && r.Method != http.MethodGet {
					writeErrorCode(w, 403, "scope_denied", "delegated token is read-only", false, "use the owner token or an explicit task grant")
					return
				}
				if grant.Scope == "tasks" && !taskEndpointAllowed(r.Method, r.URL.Path) {
					writeErrorCode(w, 403, "scope_denied", "endpoint is outside this token's task grant", false, "use the owner token for configuration, workflows and global state")
					return
				}
				r = withGrant(r, grant)
			}
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			writeError(w, http.StatusForbidden, "browser origins are not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}
