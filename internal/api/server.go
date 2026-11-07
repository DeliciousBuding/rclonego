package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DeliciousBuding/rclonego/internal/core"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Server struct {
	app        *core.App
	token      string
	tokensMu   sync.Mutex
	readTokens map[[32]byte]readToken
}

const headerGuard = "X-RcloneGo-Client"

func (a *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api", a.handleAPIInfo)
	mux.HandleFunc("GET /api/v1", a.handleAPIInfo)
	mux.HandleFunc("GET /api/v1/schema", a.handleSchema)
	mux.HandleFunc("POST /api/v1/tokens/read", a.mutating(a.handleTokens))
	mux.HandleFunc("GET /api/v1/health", a.handleHealth)
	mux.HandleFunc("GET /api/v1/doctor", a.handleDoctor)
	mux.HandleFunc("GET /api/v1/diagnostics", a.handleDiagnostics)
	mux.HandleFunc("GET /api/v1/events", a.handleEvents)
	mux.HandleFunc("GET /api/v1/engine", a.handleEngine)
	mux.HandleFunc("POST /api/v1/engine/{action}", a.mutating(a.handleEngine))
	mux.HandleFunc("GET /api/v1/providers", a.handleProviders)
	mux.HandleFunc("POST /api/v1/accounts", a.mutating(a.handleAccounts))
	mux.HandleFunc("GET /api/v1/accounts/{id}", a.handleAccounts)
	mux.HandleFunc("POST /api/v1/accounts/{id}/{action}", a.mutating(a.handleAccounts))
	mux.HandleFunc("GET /api/v1/settings", a.handleSettings)
	mux.HandleFunc("PUT /api/v1/settings", a.mutating(a.handleSaveSettings))
	mux.HandleFunc("GET /api/v1/runtime", a.handleRuntime)
	mux.HandleFunc("PUT /api/v1/external", a.mutating(a.handleExternal))
	mux.HandleFunc("POST /api/v1/runtime/bandwidth", a.mutating(a.handleBandwidth))
	mux.HandleFunc("GET /api/v1/overview", a.handleOverview)
	mux.HandleFunc("GET /api/v1/remotes", a.handleRemotes)
	mux.HandleFunc("POST /api/v1/remotes/reconnect", a.mutating(a.handleReconnect))
	mux.HandleFunc("POST /api/v1/remotes/create", a.mutating(a.handleCreateRemote))
	mux.HandleFunc("GET /api/v1/mounts", a.handleMounts)
	mux.HandleFunc("PUT /api/v1/mounts", a.mutating(a.handleSaveMounts))
	mux.HandleFunc("POST /api/v1/mounts/start", a.mutating(a.handleStartMount))
	mux.HandleFunc("POST /api/v1/mounts/stop", a.mutating(a.handleStopMount))
	mux.HandleFunc("GET /api/v1/jobs", a.handleJobs)
	mux.HandleFunc("GET /api/v1/workflow-definitions", a.handleDefinitions)
	mux.HandleFunc("POST /api/v1/workflow-definitions", a.mutating(a.handleDefinitions))
	mux.HandleFunc("PUT /api/v1/workflow-definitions", a.mutating(a.handleDefinitions))
	mux.HandleFunc("DELETE /api/v1/workflow-definitions/{id}", a.mutating(a.handleDefinitions))
	mux.HandleFunc("GET /api/v1/runs", a.handleRuns)
	mux.HandleFunc("POST /api/v1/runs", a.mutating(a.handleRuns))
	mux.HandleFunc("GET /api/v1/runs/{id}", a.handleReport)
	mux.HandleFunc("GET /api/v1/runs/{id}/report", a.handleReport)
	mux.HandleFunc("POST /api/v1/runs/{id}/{action}", a.mutating(a.handleRunAction))
	mux.HandleFunc("GET /api/v1/jobs/{id}", a.handleJob)
	mux.HandleFunc("POST /api/v1/jobs", a.mutating(a.handleAddJob))
	mux.HandleFunc("POST /api/v1/jobs/preview", a.mutating(a.handlePreview))
	mux.HandleFunc("PUT /api/v1/jobs/{id}", a.mutating(a.handleUpdateJob))
	mux.HandleFunc("POST /api/v1/workflows", a.mutating(a.handleWorkflow))
	mux.HandleFunc("POST /api/v1/jobs/action", a.mutating(a.handleJobAction))
	mux.HandleFunc("GET /api/v1/jobs/log", a.handleJobLog)
	mux.HandleFunc("POST /api/v1/queue/pause", a.mutating(a.handleQueuePause))
	return securityHeaders(a.authenticate(mux))
}

func (a *Server) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	var request core.WorkflowRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	jobs, err := a.app.AddWorkflow(request)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, jobs)
}

func (a *Server) handleBandwidth(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID        string `json:"id"`
		Bandwidth string `json:"bandwidth"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := a.app.SetBandwidth(r.Context(), request.ID, request.Bandwidth); err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "bandwidth": request.Bandwidth})
}

func (a *Server) handleRuntime(w http.ResponseWriter, r *http.Request) {
	snapshot := a.app.RuntimeSnapshot()
	switch r.URL.Query().Get("view") {
	case "resources":
		writeJSON(w, 200, snapshot.Resources)
	case "transfers":
		writeJSON(w, 200, snapshot.External)
	default:
		writeJSON(w, 200, snapshot)
	}
}

func (a *Server) handleExternal(w http.ResponseWriter, r *http.Request) {
	var configs []core.ExternalTransfer
	if err := decodeJSON(w, r, &configs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ids, ports := map[string]bool{}, map[int]bool{}
	for _, cfg := range configs {
		if cfg.ID == "" || cfg.Name == "" || cfg.RCPort < 1 || cfg.RCPort > 65535 || cfg.OwnerPID < 0 || ids[cfg.ID] || ports[cfg.RCPort] {
			writeError(w, http.StatusBadRequest, "external transfer requires unique id and RC port")
			return
		}
		ids[cfg.ID] = true
		ports[cfg.RCPort] = true
	}
	if err := a.app.UpdateState(func(state *core.State) error { state.ExternalTransfers = configs; return nil }); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, configs)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (a *Server) mutating(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerGuard) != "1" {
			writeError(w, http.StatusForbidden, "missing local client header")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" &&
			origin != "http://"+r.Host && origin != "https://"+r.Host {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next(w, r)
	}
}

func (a *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "time": time.Now(), "api_version": 1,
		"mutation_header": headerGuard + ": 1",
	})
}

func (a *Server) handleAPIInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name": "RcloneGo", "api_version": 1,
		"listen": "loopback HTTP", "mutation_header": headerGuard + ": 1",
		"operations":     []string{"copy", "copyto", "sync", "check"},
		"authentication": "Bearer token from the private api-token file",
		"endpoints": map[string]string{
			"engine": "GET /api/v1/engine", "install_engine": "POST /api/v1/engine/install",
			"select_engine": "POST /api/v1/engine/select", "providers": "GET /api/v1/providers",
			"accounts": "POST /api/v1/accounts", "account": "GET /api/v1/accounts/{id}",
			"account_action": "POST /api/v1/accounts/{id}/{answer|cancel}", "events": "GET /api/v1/events",
			"schema": "GET /api/v1/schema", "preview": "POST /api/v1/jobs/preview",
			"settings": "GET /api/v1/settings", "save_settings": "PUT /api/v1/settings",
			"runtime": "GET /api/v1/runtime", "external": "PUT /api/v1/external",
			"resources": "GET /api/v1/runtime?view=resources", "job": "GET /api/v1/jobs/{id}",
			"bandwidth": "POST /api/v1/runtime/bandwidth", "workflows": "POST /api/v1/workflows",
			"health": "GET /api/v1/health", "overview": "GET /api/v1/overview",
			"remotes": "GET /api/v1/remotes", "reconnect": "POST /api/v1/remotes/reconnect",
			"create_remote": "POST /api/v1/remotes/create", "mounts": "GET /api/v1/mounts",
			"save_mounts": "PUT /api/v1/mounts", "start_mount": "POST /api/v1/mounts/start",
			"stop_mount": "POST /api/v1/mounts/stop", "jobs": "GET /api/v1/jobs",
			"add_job": "POST /api/v1/jobs", "job_action": "POST /api/v1/jobs/action",
			"update_job": "PUT /api/v1/jobs/{id}",
			"job_log":    "GET /api/v1/jobs/log?id=<id>", "queue_pause": "POST /api/v1/queue/pause",
			"workflow_definitions": "GET/POST /api/v1/workflow-definitions",
			"runs":                 "GET/POST /api/v1/runs", "run_report": "GET /api/v1/runs/{id}/report",
		},
		"safety": map[string]any{
			"copy_never_deletes": true, "sync_requires_allow_delete": true,
			"sync_max_delete": 10, "oauth_tokens_not_exposed": true,
		},
		"defaults": a.app.Snapshot().Defaults,
		"constraints": map[string]any{
			"queue_workers": 1, "transfers": "1..32", "checkers": "1..64",
			"chunk_size": "power of two, 256K..1G", "buffer_size": "0..1G",
			"not_before": "RFC3339 with timezone", "workflow_source": "local directory",
			"workflow_order":            "fixed steps independent of queue position",
			"active_tree":               "currently transferring files only",
			"resource_interval_seconds": 15, "cpu_percent": "whole-machine normalized",
		},
	})
}

func (a *Server) handleSettings(w http.ResponseWriter, _ *http.Request) {
	state := a.app.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"appearance": state.Appearance, "defaults": state.Defaults})
}

func (a *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var request core.SettingsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.app.SaveSettings(request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.handleSettings(w, r)
}

func (a *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	state := a.app.Snapshot()
	activity := core.ActivityStatus{}
	runtime := a.app.RuntimeSnapshot()
	for _, external := range runtime.External {
		if external.Available {
			activity = core.ActivityStatus{Available: true, Stats: external.Stats}
			break
		}
	}
	if !activity.Available {
		for _, job := range state.Jobs {
			if job.Status == "running" {
				activity = core.ActivityStatus{Available: true, Stats: &core.RCStats{
					Bytes: job.Bytes, TotalBytes: job.TotalBytes, Speed: job.Speed,
					Transfers: job.FilesDone, TotalTransfers: job.FilesTotal,
					Checks: job.ChecksDone, TotalChecks: job.ChecksTotal,
					Transferring: job.ActiveFiles, Checking: job.CheckingFiles,
				}}
				break
			}
		}
	}
	result := map[string]any{
		"version": runtime.Version, "activity": activity,
		"remote_count": len(runtime.Remotes), "mounts": runtime.Mounts,
		"queue_paused": state.QueuePaused, "jobs": state.Jobs,
		"runtime": runtime,
	}
	if runtime.CatalogError != "" {
		result["remotes_error"] = runtime.CatalogError
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *Server) handleRemotes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	remotes, err := a.app.Remotes(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, remotes)
}

func (a *Server) handleReconnect(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	account, err := a.app.BeginAccount(r.Context(), core.AccountRequest{Name: request.Name, Reconnect: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, account)
}

func (a *Server) handleCreateRemote(w http.ResponseWriter, r *http.Request) {
	var request core.AccountRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	account, err := a.app.BeginAccount(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, account)
}

func (a *Server) handleMounts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	mounts, err := a.app.InspectMounts(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mounts)
}

func (a *Server) handleSaveMounts(w http.ResponseWriter, r *http.Request) {
	var mounts []core.MountConfig
	if err := decodeJSON(w, r, &mounts); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ids, drives := map[string]bool{}, map[string]bool{}
	for i := range mounts {
		mounts[i].ID = strings.TrimSpace(mounts[i].ID)
		mounts[i].Remote = core.EnsureColonRemote(strings.TrimSpace(mounts[i].Remote))
		mounts[i].Drive = core.NormalizeMountTarget(mounts[i].Drive)
		if err := core.ValidateMountConfig(mounts[i]); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("mount %d: %v", i+1, err))
			return
		}
		if ids[mounts[i].ID] || drives[mounts[i].Drive] {
			writeError(w, http.StatusBadRequest, "mount IDs and drives must be unique")
			return
		}
		ids[mounts[i].ID], drives[mounts[i].Drive] = true, true
	}
	if err := a.app.SaveMounts(mounts); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mounts)
}

func (a *Server) handleStartMount(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := findMount(a.app.Snapshot().Mounts, request.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	pid, err := a.app.StartMount(ctx, cfg)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pid": pid, "id": request.ID})
}

func (a *Server) handleStopMount(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := findMount(a.app.Snapshot().Mounts, request.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.app.StopMount(ctx, cfg); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stopped": true, "id": request.ID})
}

func findMount(mounts []core.MountConfig, id string) (core.MountConfig, error) {
	for _, mount := range mounts {
		if mount.ID == id {
			return mount, nil
		}
	}
	return core.MountConfig{}, errors.New("mount not found")
}

func (a *Server) handleJobs(w http.ResponseWriter, _ *http.Request) {
	state := a.app.Snapshot()
	jobs := append([]core.Job(nil), state.Jobs...)
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	writeJSON(w, http.StatusOK, map[string]any{"paused": state.QueuePaused, "jobs": jobs})
}

func (a *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	for _, job := range a.app.Snapshot().Jobs {
		if job.ID == r.PathValue("id") {
			writeJSON(w, 200, job)
			return
		}
	}
	writeError(w, 404, "job not found")
}

func (a *Server) handleAddJob(w http.ResponseWriter, r *http.Request) {
	var request core.JobCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if key := strings.TrimSpace(r.Header.Get("Idempotency-Key")); key != "" {
		if request.IdempotencyKey != "" && request.IdempotencyKey != key {
			writeError(w, 400, "conflicting idempotency keys")
			return
		}
		request.IdempotencyKey = key
	}
	job, err := a.app.AddJob(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (a *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	var request core.JobUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if err := a.app.UpdateParameters(r.PathValue("id"), request); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	a.app.Wake()
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *Server) handleJobAction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Delta  int    `json:"delta"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var err error
	switch request.Action {
	case "retry":
		err = a.app.Retry(request.ID)
	case "cancel":
		err = a.app.Cancel(request.ID)
	case "pause":
		err = a.app.Pause(request.ID)
	case "delete":
		err = a.app.Delete(request.ID)
	case "move":
		err = a.app.Move(request.ID, request.Delta)
	case "delete-workflow":
		err = a.app.DeleteWorkflow(request.ID)
	default:
		err = errors.New("unknown action")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.app.Wake()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Server) handleQueuePause(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Paused bool `json:"paused"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.app.SetPaused(request.Paused); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"paused": request.Paused})
}

func (a *Server) handleJobLog(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if !regexp.MustCompile(`^[a-f0-9]{16}$`).MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	text, err := a.app.JobLog(id, 256*1024)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	code := map[int]string{400: "invalid_request", 401: "authentication_required", 403: "permission_denied", 404: "not_found",
		409: "conflict", 500: "internal_error", 502: "engine_failed", 503: "engine_unavailable"}[status]
	if code == "" {
		code = "request_failed"
	}
	writeJSON(w, status, map[string]any{"error": message, "code": code, "message": message,
		"retryable": status == 502 || status == 503, "next_action": map[int]string{401: "use a local API token", 503: "install or select an engine"}[status]})
}
