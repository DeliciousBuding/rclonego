package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

type grantContextKey struct{}

func delegatedGrant(r *http.Request) *tokenGrant {
	grant, _ := r.Context().Value(grantContextKey{}).(*tokenGrant)
	return grant
}

func withGrant(r *http.Request, grant tokenGrant) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), grantContextKey{}, &grant))
}

func taskEndpointAllowed(method, endpoint string) bool {
	if method == http.MethodGet {
		switch endpoint {
		case "/api", "/api/v1", "/api/v1/schema", "/api/v1/browse", "/api/v1/jobs", "/api/v1/jobs/log":
			return true
		}
		if strings.HasPrefix(endpoint, "/api/v1/jobs/") && !strings.Contains(strings.TrimPrefix(endpoint, "/api/v1/jobs/"), "/") {
			return true
		}
	}
	if method == http.MethodPost {
		switch endpoint {
		case "/api/v1/jobs", "/api/v1/jobs/preview", "/api/v1/jobs/action":
			return true
		}
	}
	return false
}

func (grant tokenGrant) allowsJob(job core.Job) bool {
	if job.WorkflowID != "" || job.ProofPath != "" || job.ProofRoot != "" {
		return false
	}
	operation := job.Operation
	if operation == "" {
		operation = "copy"
	}
	if !containsString(grant.Operations, operation) {
		return false
	}
	return grant.allowsPath(job.Source) && grant.allowsPath(job.Destination)
}

func (grant tokenGrant) allowsPath(candidate string) bool {
	normalized, err := normalizeGrantPath(candidate)
	if err != nil {
		return false
	}
	for _, root := range grant.Paths {
		// Resolve local roots again: a replaced symlink must not enlarge a grant.
		currentRoot, err := normalizeGrantPath(root)
		if err != nil || currentRoot != root {
			continue
		}
		if normalized == root {
			return true
		}
		if remoteName(root) != "" {
			prefix := root
			if !strings.HasSuffix(prefix, ":") {
				prefix += "/"
			}
			if strings.HasPrefix(normalized, prefix) {
				return true
			}
		} else if remoteName(normalized) == "" {
			relative, err := filepath.Rel(root, normalized)
			if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
				return true
			}
		}
	}
	return false
}

func authorizeJobRequest(w http.ResponseWriter, r *http.Request, request core.JobCreateRequest) bool {
	grant := delegatedGrant(r)
	if grant == nil || grant.Scope == "read" {
		return true
	}
	// Proofs and workflow identifiers are owner-level inputs. Delegated jobs
	// use only the two explicitly granted paths and normal transfer filters.
	if request.ProofPath != "" || request.ProofRoot != "" || request.WorkflowID != "" ||
		len(request.Files) != 0 || request.WorkflowTotalBytes != 0 || request.WorkflowInitialBytes != 0 {
		writeErrorCode(w, http.StatusForbidden, "scope_denied", "delegated tasks cannot supply workflow or proof inputs", false, "use the owner token for workflow management")
		return false
	}
	job := core.Job{Operation: request.Operation, Source: request.Source, Destination: request.Destination}
	if !grant.allowsJob(job) {
		writeErrorCode(w, http.StatusForbidden, "scope_denied", "operation or path is outside this token's task grant", false, "request an owner-issued grant for the required paths and operation")
		return false
	}
	return true
}

func authorizeStoredJob(w http.ResponseWriter, r *http.Request, app *core.App, id string) bool {
	grant := delegatedGrant(r)
	if grant == nil || grant.Scope == "read" {
		return true
	}
	for _, job := range app.Snapshot().Jobs {
		if job.ID == id && grant.allowsJob(job) {
			return true
		}
	}
	writeErrorCode(w, http.StatusForbidden, "scope_denied", "job is outside this token's task grant or is unavailable", false, "use an owner token or a matching task grant")
	return false
}

func visibleJobs(r *http.Request, jobs []core.Job) []core.Job {
	grant := delegatedGrant(r)
	if grant == nil || grant.Scope == "read" {
		return jobs
	}
	result := []core.Job{}
	for _, job := range jobs {
		if grant.allowsJob(job) {
			// Internal process/configuration evidence stays private to the owner.
			job.LogPath, job.ProofPath, job.ProofRoot = "", "", ""
			result = append(result, job)
		}
	}
	return result
}

var remoteScopeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_. -]*$`)

func remoteName(value string) string {
	colon := strings.IndexByte(value, ':')
	if colon < 1 || (colon == 1 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z'))) {
		return ""
	}
	name := value[:colon]
	if !remoteScopeName.MatchString(name) || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}

func normalizeGrantPath(value string) (string, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsRune(value, 0) {
		return "", errors.New("grant paths must be nonempty paths without surrounding whitespace")
	}
	if name := remoteName(value); name != "" {
		suffix := value[len(name)+1:]
		if strings.HasPrefix(suffix, "/") || strings.ContainsAny(suffix, `\`) {
			return "", errors.New("remote grant paths use relative forward-slash paths")
		}
		for _, part := range strings.Split(suffix, "/") {
			if part == ".." {
				return "", errors.New("remote grant paths cannot contain parent traversal")
			}
		}
		suffix = path.Clean(suffix)
		if suffix == "." {
			suffix = ""
		}
		return name + ":" + suffix, nil
	}
	if !filepath.IsAbs(value) {
		return "", errors.New("local grant paths must be absolute")
	}
	// Resolve the existing ancestor even for a not-yet-created destination.
	// This prevents a symlink under a granted tree escaping the tree.
	value = filepath.Clean(value)
	ancestor, tail := value, []string{}
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("grant path has no accessible existing ancestor")
		}
		tail = append(tail, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(tail) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, tail[i])
	}
	if runtime.GOOS == "windows" {
		resolved = strings.ToLower(resolved)
	}
	return resolved, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
