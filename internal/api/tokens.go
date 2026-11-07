package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

type readToken struct {
	ExpiresAt time.Time
}

// Delegated preview tokens are read-only and live for this server session.
// Mutations and configuration always require the trusted local owner token.
func (a *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if input.TTLSeconds == 0 {
		input.TTLSeconds = 3600
	}
	if input.TTLSeconds < 60 || input.TTLSeconds > 86400 {
		writeError(w, 400, "ttl_seconds must be 60 through 86400")
		return
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		writeError(w, 500, "could not generate read token")
		return
	}
	token := hex.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(time.Duration(input.TTLSeconds) * time.Second)
	a.tokensMu.Lock()
	if a.readTokens == nil {
		a.readTokens = map[[32]byte]readToken{}
	}
	for id, token := range a.readTokens {
		if time.Now().After(token.ExpiresAt) {
			delete(a.readTokens, id)
		}
	}
	if len(a.readTokens) >= 32 {
		a.tokensMu.Unlock()
		writeError(w, 409, "too many delegated tokens")
		return
	}
	a.readTokens[hash] = readToken{ExpiresAt: expires}
	a.tokensMu.Unlock()
	writeJSON(w, 201, map[string]any{"token": token, "scope": "read", "expires_at": expires, "session_only": true})
}
