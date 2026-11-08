package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

func (a *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	request := core.BrowseRequest{Path: r.URL.Query().Get("path")}
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil {
			writeError(w, 400, "offset must be an integer")
			return
		}
		request.Offset = offset
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil {
			writeError(w, 400, "limit must be an integer")
			return
		}
		request.Limit = limit
	}
	if value := r.URL.Query().Get("directories_only"); value != "" {
		directories, err := strconv.ParseBool(value)
		if err != nil {
			writeError(w, 400, "directories_only must be a boolean")
			return
		}
		request.DirectoriesOnly = directories
	}
	grant := delegatedGrant(r)
	if grant != nil && grant.Scope == "tasks" && !grant.allowsPath(request.Path) {
		writeErrorCode(w, 403, "scope_denied", "directory is outside this token's task grant", false, "use a matching path grant")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := a.app.Browse(ctx, request)
	if err != nil {
		writeFailure(w, 400, err)
		return
	}
	if grant != nil && grant.Scope == "tasks" {
		visible := []core.DirectoryEntry{}
		for _, entry := range result.Entries {
			if grant.allowsPath(entry.Path) {
				visible = append(visible, entry)
			}
		}
		result.Entries = visible
		if result.Parent != "" && !grant.allowsPath(result.Parent) {
			result.Parent = ""
		}
	}
	writeJSON(w, 200, result)
}
