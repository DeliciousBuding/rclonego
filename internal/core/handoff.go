package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type HandoffRequest struct {
	StatePath  string `json:"state_path"`
	ConfigPath string `json:"config_path"`
	Endpoint   string `json:"endpoint,omitempty"`
	PreviewKey string `json:"preview_key,omitempty"`
}

// Only the one-time handoff is retained. The old client remains paused after
// ownership transfers; ongoing execution uses the normal persistent queue.
type LegacyHandoff struct {
	StatePath      string            `json:"state_path"`
	Endpoint       string            `json:"endpoint"`
	PreviewKey     string            `json:"preview_key"`
	DefinitionKey  string            `json:"definition_key"`
	Status         string            `json:"status"`
	StartedHere    bool              `json:"started_here"`
	Message        string            `json:"message,omitempty"`
	ActiveJobID    string            `json:"active_job_id,omitempty"`
	JobIDs         []string          `json:"job_ids"`
	SourceStatuses map[string]string `json:"source_statuses"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type HandoffPreview struct {
	PreviewKey  string           `json:"preview_key"`
	Jobs        []Job            `json:"jobs"`
	Remotes     []RemoteInfo     `json:"remotes"`
	Defaults    TransferSettings `json:"defaults"`
	QueuePaused bool             `json:"queue_paused"`
	ActiveJobID string           `json:"active_job_id,omitempty"`
	Mounts      int              `json:"mounts"`
}

type legacyState struct {
	Version     int              `json:"version"`
	Appearance  string           `json:"appearance"`
	Defaults    TransferSettings `json:"defaults"`
	QueuePaused bool             `json:"queue_paused"`
	Jobs        []Job            `json:"jobs"`
	Mounts      []MountConfig    `json:"mounts"`
}

func readLegacyState(file string) (legacyState, error) {
	var state legacyState
	if !filepath.IsAbs(file) {
		return state, errors.New("select an absolute legacy state.json path")
	}
	f, err := os.Open(file)
	if err != nil {
		return state, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil || len(raw) > 8<<20 {
		return state, errors.New("legacy state exceeds the 8 MiB import limit")
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, fmt.Errorf("read legacy state: %w", err)
	}
	if state.Version != 1 || len(state.Jobs) == 0 {
		return state, errors.New("select a Data Ops Console version 1 state containing tasks")
	}
	return state, nil
}

func legacyPreviewKey(state legacyState, config []byte) string {
	// Live counters and terminal status may change while the user reads the
	// preview. Transfer definitions, filters, order and credentials must match.
	jobs := append([]Job(nil), state.Jobs...)
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	for i := range jobs {
		job := &jobs[i]
		job.Status, job.Error, job.LogPath, job.ProcessIdentity = "", "", "", ""
		job.PID, job.RCPort, job.Attempt = 0, 0, 0
		job.Bytes, job.TotalBytes, job.Speed = 0, 0, 0
		job.FilesDone, job.FilesTotal, job.ChecksDone, job.ChecksTotal = 0, 0, 0, 0
		job.StartedAt, job.FinishedAt, job.ExitCode = nil, nil, nil
		job.ActiveFiles, job.CheckingFiles = nil, nil
		job.LastUpdatedAt = time.Time{}
	}
	raw, _ := json.Marshal(struct {
		Jobs     []Job
		Defaults TransferSettings
	}{jobs, state.Defaults})
	sum := sha256.Sum256(append(raw, config...))
	return hex.EncodeToString(sum[:])
}

func (a *App) PreviewHandoff(ctx context.Context, request HandoffRequest) (HandoffPreview, error) {
	state, err := readLegacyState(request.StatePath)
	if err != nil {
		return HandoffPreview{}, err
	}
	if err := state.Defaults.Validate(); err != nil {
		return HandoffPreview{}, err
	}
	if !filepath.IsAbs(request.ConfigPath) {
		return HandoffPreview{}, errors.New("select an absolute rclone configuration path")
	}
	config, err := os.ReadFile(request.ConfigPath)
	if err != nil {
		return HandoffPreview{}, err
	}
	if len(config) > 8<<20 {
		return HandoffPreview{}, errors.New("configuration exceeds the import limit")
	}
	result := HandoffPreview{PreviewKey: legacyPreviewKey(state, config), Jobs: state.Jobs,
		Defaults: state.Defaults, QueuePaused: state.QueuePaused, Mounts: len(state.Mounts)}
	ids, positions := map[string]bool{}, map[int]bool{}
	groups := map[string][]Job{}
	for _, job := range state.Jobs {
		if job.ID == "" || ids[job.ID] || job.Position < 1 || positions[job.Position] || validateSafeText(job.ID) != nil {
			return result, errors.New("legacy tasks require unique IDs and positions")
		}
		ids[job.ID], positions[job.Position] = true, true
		switch job.Status {
		case "running":
			if result.ActiveJobID != "" || job.PID < 1 {
				return result, errors.New("handoff supports one recorded active transfer")
			}
			result.ActiveJobID = job.ID
		case "completed":
			if job.ExitCode == nil || *job.ExitCode != 0 || job.FinishedAt == nil {
				return result, errors.New("completed legacy tasks require a successful final result")
			}
		case "pending", "failed", "paused", "cancelled", "interrupted":
		default:
			return result, fmt.Errorf("unsupported legacy task status: %s", job.Status)
		}
		if job.Operation != "copy" && job.Operation != "copyto" && job.Operation != "check" {
			return result, errors.New("legacy import supports copy and verification tasks")
		}
		if _, err := rcloneTransferArgs(job, 1, ""); err != nil {
			return result, err
		}
		if job.Status != "completed" {
			if err := a.queue.validateProof(job); err != nil {
				return result, err
			}
		}
		if job.WorkflowID != "" {
			groups[job.WorkflowID] = append(groups[job.WorkflowID], job)
		}
	}
	for _, jobs := range groups {
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].WorkflowStep < jobs[j].WorkflowStep })
		for i, job := range jobs {
			if job.WorkflowStep != i+1 || job.WorkflowSize != len(jobs) || (i > 0 && job.Position <= jobs[i-1].Position) {
				return result, errors.New("legacy workflow order is incomplete")
			}
		}
	}
	if result.ActiveJobID != "" && request.Endpoint == "" {
		return result, errors.New("a running transfer requires the old client's loopback API endpoint")
	}
	if request.Endpoint != "" {
		if _, err := legacyEndpoint(request.Endpoint); err != nil {
			return result, err
		}
		if err := verifyLegacyEndpoint(ctx, request.Endpoint, state.Jobs); err != nil {
			return result, err
		}
	}
	client := a.EngineClient()
	client.ConfigPath = request.ConfigPath
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result.Remotes, err = client.Remotes(readCtx)
	if err != nil {
		return result, err
	}
	return result, nil
}

func verifyLegacyEndpoint(ctx context.Context, endpoint string, jobs []Job) error {
	base, err := legacyEndpoint(endpoint)
	if err != nil {
		return err
	}
	base.Path = "/api/jobs"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		return errors.New("old client is not reachable")
	}
	defer response.Body.Close()
	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&result) != nil {
		return errors.New("old client did not return its tasks")
	}
	want := map[string]bool{}
	for _, job := range jobs {
		want[job.ID] = true
	}
	for _, job := range result.Jobs {
		if !want[job.ID] {
			return errors.New("endpoint and state file belong to different legacy queues")
		}
		delete(want, job.ID)
	}
	if len(want) != 0 {
		return errors.New("endpoint and state file belong to different legacy queues")
	}
	return nil
}

func legacyEndpoint(value string) (*url.URL, error) {
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Scheme != "http" || endpoint.User != nil ||
		(endpoint.Hostname() != "127.0.0.1" && endpoint.Hostname() != "::1" && endpoint.Hostname() != "localhost") ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("legacy endpoint must be a loopback HTTP origin")
	}
	return endpoint, nil
}

func pauseLegacy(ctx context.Context, endpoint string, paused bool) error {
	base, err := legacyEndpoint(endpoint)
	if err != nil {
		return err
	}
	base.Path = "/api/queue/pause"
	raw, _ := json.Marshal(map[string]bool{"paused": paused})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-DataOps-Console", "1")
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("old client is not reachable")
	}
	defer response.Body.Close()
	var result struct {
		Paused bool `json:"paused"`
	}
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil || result.Paused != paused {
		return errors.New("old client did not confirm its queue state")
	}
	return nil
}

func (a *App) BeginHandoff(ctx context.Context, request HandoffRequest) (result LegacyHandoff, err error) {
	a.handoffMu.Lock()
	defer a.handoffMu.Unlock()
	preview, err := a.PreviewHandoff(ctx, request)
	if err != nil {
		return LegacyHandoff{}, err
	}
	if request.PreviewKey == "" || request.PreviewKey != preview.PreviewKey {
		return LegacyHandoff{}, errors.New("task definitions or configuration changed; preview the import again")
	}
	if current := a.Snapshot().Handoff; current != nil {
		if current.Status == "returned" {
			return LegacyHandoff{}, errors.New("ownership was returned; use a new empty profile for another import")
		}
		if current.PreviewKey == preview.PreviewKey && current.StatePath == request.StatePath {
			return *current, nil
		}
		return LegacyHandoff{}, errors.New("this profile already contains a legacy import")
	}
	if !a.configMu.TryLock() {
		return LegacyHandoff{}, errors.New("finish account configuration before importing")
	}
	defer a.configMu.Unlock()
	client := a.EngineClient()
	importedConfig, err := os.ReadFile(request.ConfigPath)
	if err != nil {
		return LegacyHandoff{}, err
	}
	oldConfig, err := os.ReadFile(client.ConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LegacyHandoff{}, err
	}
	if len(bytes.TrimSpace(oldConfig)) > 0 && !bytes.Equal(oldConfig, importedConfig) {
		return LegacyHandoff{}, errors.New("use an empty profile or the same rclone configuration for legacy import")
	}
	committed := false
	defer func() {
		if !committed && request.Endpoint != "" && !preview.QueuePaused {
			if rollback := pauseLegacy(context.Background(), request.Endpoint, false); rollback != nil {
				err = fmt.Errorf("%w; previous queue is still paused; resume it from its client", err)
			}
		}
	}()
	if request.Endpoint != "" {
		if err := pauseLegacy(ctx, request.Endpoint, true); err != nil {
			return LegacyHandoff{}, fmt.Errorf("%w; previous queue state is unconfirmed", err)
		}
	}
	if request.Endpoint == "" {
		lock, err := AcquireInstance(filepath.Join(filepath.Dir(request.StatePath), ".instance.lock"))
		if err != nil {
			return LegacyHandoff{}, errors.New("stop the idle old client before importing without its endpoint")
		}
		defer lock()
	}
	// The pause may race the old queue's reservation of its next step. Validate
	// the entire final snapshot, including terminal evidence and a usable PID.
	finalPreview, err := a.PreviewHandoff(ctx, request)
	if err != nil {
		return LegacyHandoff{}, err
	}
	if finalPreview.PreviewKey != preview.PreviewKey {
		return LegacyHandoff{}, errors.New("legacy definitions changed during handoff; preview again")
	}
	old := legacyState{Version: 1, Jobs: finalPreview.Jobs, Defaults: finalPreview.Defaults, QueuePaused: finalPreview.QueuePaused}
	if request.Endpoint != "" && !old.QueuePaused {
		return LegacyHandoff{}, errors.New("legacy queue must remain paused during handoff")
	}
	handoff := LegacyHandoff{StatePath: request.StatePath, Endpoint: request.Endpoint, PreviewKey: preview.PreviewKey,
		DefinitionKey: legacyPreviewKey(old, nil), SourceStatuses: map[string]string{}, Status: "complete", UpdatedAt: time.Now().UTC()}
	for _, job := range old.Jobs {
		handoff.SourceStatuses[job.ID] = job.Status
		if job.Status == "running" {
			handoff.ActiveJobID, handoff.Status = job.ID, "waiting"
		}
		handoff.JobIDs = append(handoff.JobIDs, job.ID)
	}
	a.queue.addMu.Lock()
	defer a.queue.addMu.Unlock()
	err = a.store.Update(func(state *State) error {
		hasHistory, err := a.store.db.HasTaskHistory()
		if err != nil {
			return err
		}
		if hasHistory || len(state.Jobs) != 0 || len(state.Runs) != 0 || len(state.Attempts) != 0 {
			return errors.New("legacy import requires a profile with no task history; choose a new empty profile")
		}
		// Hold both dispatch and state locks before selecting credentials, so
		// concurrent submissions cannot run against a provisional import.
		if err := os.WriteFile(client.ConfigPath, importedConfig, 0o600); err != nil {
			return err
		}
		position := lastPosition(state.Jobs)
		groups := map[string]bool{}
		for _, oldJob := range old.Jobs {
			job := oldJob
			job.Origin = "data-ops-console"
			if job.StartedAt != nil {
				job.Attempt = 1
			}
			job.ProcessIdentity = ""
			if job.ProofPath != "" {
				proof, err := os.ReadFile(job.ProofPath)
				if err != nil {
					return err
				}
				sum := sha256.Sum256(proof)
				proofDir := filepath.Join(a.dataDir, "proofs")
				if err := os.MkdirAll(proofDir, 0o700); err != nil {
					return err
				}
				job.ProofPath = filepath.Join(proofDir, "legacy-"+hex.EncodeToString(sum[:8])+".json")
				if err := os.WriteFile(job.ProofPath, proof, 0o600); err != nil {
					return err
				}
			}
			job.Position += position
			if job.Status == "running" {
				job.Status = "handoff"
			}
			// No process or mount is adopted. Interrupted legacy work needs
			// explicit retry after the handoff instead of automatic relaunch.
			if job.Status == "interrupted" {
				job.Status = "paused"
			}
			job.LogPath, err = a.copyLegacyLog(oldJob)
			if err != nil {
				return err
			}
			state.Jobs = append(state.Jobs, job)
			a.store.recordAttemptLocked(job)
			if job.WorkflowID != "" && !groups[job.WorkflowID] {
				groups[job.WorkflowID] = true
				state.Runs = append(state.Runs, Run{ID: job.WorkflowID, Name: legacyRunName(old.Jobs, job.WorkflowID), CreatedAt: job.CreatedAt})
			}
		}
		state.Handoff = &handoff
		state.Defaults = old.Defaults
		// Original mount configuration remains in the old client. Visible
		// mounts are not ownership proof and cannot be adopted by importing.
		return nil
	})
	if err != nil {
		if len(oldConfig) == 0 {
			_ = os.WriteFile(client.ConfigPath, oldConfig, 0o600)
		}
		return LegacyHandoff{}, err
	}
	committed = true
	a.RefreshRuntime()
	a.Wake()
	return handoff, nil
}

func legacyRunName(jobs []Job, id string) string {
	for _, job := range jobs {
		if job.WorkflowID == id {
			name, _, _ := strings.Cut(job.Name, " · ")
			return name
		}
	}
	return id
}

func (a *App) copyLegacyLog(job Job) (string, error) {
	if job.LogPath == "" {
		return "", nil
	}
	raw, err := tailFile(job.LogPath, taskLogLimit)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(job.ID))
	file := filepath.Join(a.dataDir, "logs", "legacy-"+hex.EncodeToString(sum[:8])+".log")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return "", err
	}
	return file, os.WriteFile(file, []byte(raw), 0o600)
}

func (a *App) refreshHandoff(ctx context.Context) {
	if !a.handoffMu.TryLock() {
		return
	}
	defer a.handoffMu.Unlock()
	current := a.Snapshot().Handoff
	if current == nil || current.Status == "complete" || current.Status == "returned" {
		return
	}
	old, err := readLegacyState(current.StatePath)
	message := ""
	if err != nil {
		message = "Cannot read the previous client's final task result"
	} else if legacyPreviewKey(old, nil) != current.DefinitionKey {
		message = "The previous task definitions changed; return ownership to the previous client"
	} else if !old.QueuePaused {
		message = "The previous queue was resumed; pause it before continuing the handoff"
	}
	var active Job
	if message == "" {
		statuses := current.SourceStatuses
		if statuses == nil {
			statuses = map[string]string{}
			for _, imported := range a.Snapshot().Jobs {
				statuses[imported.ID] = imported.Status
			}
			current.SourceStatuses = statuses
		}
		for _, job := range old.Jobs {
			if job.ID == current.ActiveJobID {
				active = job
			}
			if job.ID != current.ActiveJobID {
				if statuses[job.ID] != job.Status {
					message = "The previous queue advanced to another step; return ownership to the previous client"
				}
			}
		}
		if active.ID == "" {
			message = "The previous client no longer contains the active task"
		}
	}
	next := *current
	next.Message = message
	next.UpdatedAt = time.Now().UTC()
	if message != "" {
		next.Status = "blocked"
	} else if active.Status == "running" {
		next.Status = "waiting"
		if active.PID <= 0 || !processExists(active.PID) {
			next.Status, next.Message = "blocked", "The previous client has not recorded the exited transfer's result"
		}
	} else if active.Status == "completed" && active.ExitCode != nil && *active.ExitCode == 0 && active.FinishedAt != nil {
		next.Status, next.ActiveJobID = "complete", ""
	} else if active.Status == "failed" || active.Status == "cancelled" || active.Status == "interrupted" {
		next.Status = "complete" // handoff finished, transfer did not succeed.
	} else {
		next.Status, next.Message = "blocked", "The previous client's final result is not available"
	}
	if message == "" && active.Status != "running" && active.PID > 0 && processExists(active.PID) {
		next.Status, next.Message = "blocked", "Waiting for the previous transfer process to exit"
	}
	err = a.store.Update(func(state *State) error {
		if next.Message == "" {
			for i := range state.Jobs {
				if state.Jobs[i].ID != current.ActiveJobID {
					continue
				}
				job := active
				job.Position, job.Origin, job.LogPath, job.ProofPath = state.Jobs[i].Position, "data-ops-console", state.Jobs[i].LogPath, state.Jobs[i].ProofPath
				job.ProcessIdentity = ""
				if job.StartedAt != nil {
					job.Attempt = 1
				}
				if job.Status == "running" {
					job.Status = "handoff"
				}
				if job.Status == "interrupted" {
					job.Status = "paused"
				}
				if next.Status == "complete" {
					job.LogPath, err = a.copyLegacyLog(active)
					if err != nil {
						return err
					}
				}
				state.Jobs[i] = job
				a.store.recordAttemptLocked(job)
			}
		}
		state.Handoff = &next
		return nil
	})
	if err == nil && next.Status == "complete" {
		a.Wake()
	}
}

// ReturnHandoff relinquishes an import only before this client starts any
// imported work. The old queue can then resume its recorded active process.
func (a *App) ReturnHandoff(ctx context.Context) (LegacyHandoff, error) {
	a.handoffMu.Lock()
	defer a.handoffMu.Unlock()
	a.queue.addMu.Lock()
	defer a.queue.addMu.Unlock()
	state := a.Snapshot()
	if state.Handoff == nil {
		return LegacyHandoff{}, errors.New("this profile has no legacy handoff")
	}
	if state.Handoff.Status == "returned" {
		return *state.Handoff, nil
	}
	if state.Handoff.StartedHere {
		return LegacyHandoff{}, errors.New("new client has started imported work; finish it here before returning ownership")
	}
	for _, job := range state.Jobs {
		if job.Origin == "data-ops-console" && (job.Status == "running" || job.ProcessIdentity != "" || (job.Attempt > 1)) {
			return LegacyHandoff{}, errors.New("new client has started imported work; finish it here before returning ownership")
		}
	}
	next := *state.Handoff
	next.Status, next.Message, next.UpdatedAt = "returned", "", time.Now().UTC()
	if err := a.store.Update(func(state *State) error {
		for i := range state.Jobs {
			job := &state.Jobs[i]
			if job.Origin == "data-ops-console" && job.Status != "completed" {
				job.Status, job.Speed = "cancelled", 0
			}
		}
		state.Handoff = &next
		return nil
	}); err != nil {
		return next, err
	}
	if next.Endpoint != "" {
		if err := pauseLegacy(ctx, next.Endpoint, false); err != nil {
			return next, errors.New("import was relinquished; resume the old queue from its client")
		}
	}
	a.RefreshRuntime()
	return next, nil
}

func transferPathsOverlap(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	lroot, _, lk, le := browsePath(left)
	rroot, _, rk, re := browsePath(right)
	if le != nil || re != nil || lk != rk {
		return false
	}
	if lk == "local" {
		relative, err := filepath.Rel(lroot, rroot)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return true
		}
		relative, err = filepath.Rel(rroot, lroot)
		return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
	}
	prefix := func(value string) string {
		if strings.HasSuffix(value, ":") {
			return value
		}
		return strings.TrimSuffix(value, "/") + "/"
	}
	return lroot == rroot || strings.HasPrefix(lroot, prefix(rroot)) || strings.HasPrefix(rroot, prefix(lroot))
}

func (a *App) dispatchBlocked(job Job, state State) bool {
	mounts := a.runtime.Snapshot().Mounts
	job.Source = mountedTransferPath(job.Source, mounts)
	job.Destination = mountedTransferPath(job.Destination, mounts)
	if state.Handoff != nil && state.Handoff.Status == "returned" && job.Origin == "data-ops-console" {
		return true
	}
	if state.Handoff != nil && state.Handoff.Status != "complete" && state.Handoff.Status != "returned" {
		for _, id := range state.Handoff.JobIDs {
			if job.ID == id {
				return true
			}
		}
		for _, imported := range state.Jobs {
			if imported.Origin == "data-ops-console" && (transferPathsOverlap(job.Source, imported.Source) ||
				transferPathsOverlap(job.Destination, imported.Destination) || transferPathsOverlap(job.Source, imported.Destination) ||
				transferPathsOverlap(job.Destination, imported.Source)) {
				return true
			}
		}
	}
	for _, external := range state.ExternalTransfers {
		busy := external.OwnerPID > 0 && processExists(external.OwnerPID)
		for _, view := range a.runtime.Snapshot().External {
			busy = busy || (view.ID == external.ID && view.Available)
		}
		if busy && (external.BlockQueue || transferPathsOverlap(job.Source, external.Source) || transferPathsOverlap(job.Source, external.Destination) ||
			transferPathsOverlap(job.Destination, external.Source) || transferPathsOverlap(job.Destination, external.Destination)) {
			return true
		}
	}
	return false
}
