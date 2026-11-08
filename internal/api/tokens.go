package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"time"
)

type tokenGrant struct {
	ID         string    `json:"id"`
	Scope      string    `json:"scope"`
	ExpiresAt  time.Time `json:"expires_at"`
	Operations []string  `json:"operations,omitempty"`
	Paths      []string  `json:"paths,omitempty"`
}

type TokenRequest struct {
	TTLSeconds int      `json:"ttl_seconds"`
	Operations []string `json:"operations,omitempty"`
	Paths      []string `json:"paths,omitempty"`
}

type TokenResponse struct {
	tokenGrant
	Token       string `json:"token"`
	SessionOnly bool   `json:"session_only"`
}

// Grants are held only in this server session. The owner can revoke them by
// their public ID; only a SHA256 digest of the bearer token is retained.
func (a *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	var input TokenRequest
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
	scope := "read"
	if r.URL.Path == "/api/v1/tokens/tasks" {
		scope = "tasks"
		if err := validateTaskGrant(&input); err != nil {
			writeError(w, 400, err.Error())
			return
		}
	} else if len(input.Operations) != 0 || len(input.Paths) != 0 {
		writeError(w, 400, "read tokens do not accept task operations or path grants")
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
		a.readTokens = map[[32]byte]tokenGrant{}
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
	grant := tokenGrant{ID: hex.EncodeToString(hash[:16]), Scope: scope, ExpiresAt: expires,
		Operations: input.Operations, Paths: input.Paths}
	a.readTokens[hash] = grant
	a.tokensMu.Unlock()
	writeJSON(w, 201, TokenResponse{tokenGrant: grant, Token: token, SessionOnly: true})
}

func validateTaskGrant(input *TokenRequest) error {
	if len(input.Operations) < 1 || len(input.Operations) > 4 || len(input.Paths) < 1 || len(input.Paths) > 16 {
		return errors.New("task grants require 1..4 operations and 1..16 absolute local or named remote paths")
	}
	operations, paths := map[string]bool{}, map[string]bool{}
	for _, operation := range input.Operations {
		switch operation {
		case "copy", "copyto", "check", "sync":
			operations[operation] = true
		default:
			return errors.New("task operations must be copy, copyto, check or sync")
		}
	}
	for _, value := range input.Paths {
		normalized, err := normalizeGrantPath(value)
		if err != nil {
			return err
		}
		paths[normalized] = true
	}
	input.Operations, input.Paths = nil, nil
	for value := range operations {
		input.Operations = append(input.Operations, value)
	}
	for value := range paths {
		input.Paths = append(input.Paths, value)
	}
	sort.Strings(input.Operations)
	sort.Strings(input.Paths)
	return nil
}

func (a *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.tokensMu.Lock()
	defer a.tokensMu.Unlock()
	for hash, grant := range a.readTokens {
		if grant.ID == id {
			delete(a.readTokens, hash)
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeError(w, 404, "delegated token not found")
}
