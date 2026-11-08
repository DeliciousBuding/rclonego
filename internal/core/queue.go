package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Queue struct {
	store        *Store
	rclone       string
	client       RcloneClient
	dataDir      string
	mu           sync.Mutex
	addMu        sync.Mutex
	wake         chan struct{}
	stop         chan struct{}
	running      string
	cancel       context.CancelFunc
	done         chan struct{}
	closed       bool
	closing      bool
	pausing      bool
	fault        error
	faultAt      time.Time
	unsavedFinal *Job
	cleanupAt    time.Time
	cleanupError error
	blocked      func(Job, State) bool
}

func NewQueue(store *Store, rclonePath, dataDir string) *Queue {
	return &Queue{
		store: store, rclone: rclonePath, dataDir: dataDir, client: RcloneClient{Path: rclonePath},
		wake: make(chan struct{}, 1), stop: make(chan struct{}),
	}
}

func (q *Queue) Start() {
	q.mu.Lock()
	if q.done != nil || q.closed {
		q.mu.Unlock()
		return
	}
	q.done = make(chan struct{})
	q.mu.Unlock()
	go q.loop()
}

func (q *Queue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.closing = true
	if q.done == nil {
		q.mu.Unlock()
		return
	}
	close(q.stop)
	if q.cancel != nil {
		q.cancel()
	}
	done := q.done
	q.mu.Unlock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}

func (q *Queue) Wake() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue) Health() ServiceHealth {
	q.mu.Lock()
	failed, since, cleanupFailed := q.fault != nil, q.faultAt, q.cleanupError != nil
	q.mu.Unlock()
	if failed {
		return storageHealth(since)
	}
	if health := q.store.Health(); !health.Ready {
		return health
	}
	for _, job := range q.store.Snapshot().Jobs {
		if job.Status == "interrupted" && interruptedProcessActive(job) {
			return ServiceHealth{Code: "previous_process_running",
				Message:    "A previous transfer process may still be running",
				NextAction: "Wait for that process to stop before resuming this queue"}
		}
	}
	if cleanupFailed {
		return ServiceHealth{Ready: true, Code: "log_cleanup_failed",
			Message:    "Some completed logs could not be removed",
			NextAction: "Check permissions in the application's task log directory"}
	}
	return readyHealth()
}

func (q *Queue) storageFailure(err error, final *Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.fault == nil {
		q.faultAt = time.Now().UTC()
	}
	q.fault = err
	if final != nil {
		copy := *final
		q.unsavedFinal = &copy
	}
}

// Recover persists a completion that could not be recorded before allowing
// another launch. It never guesses that an unrecorded transfer succeeded.
func (q *Queue) Recover() error {
	q.mu.Lock()
	pending := q.unsavedFinal
	active := q.running != ""
	if q.running != "" && (q.fault != nil || pending != nil) {
		q.mu.Unlock()
		return errors.New("wait for the active process to stop before recovering storage")
	}
	q.mu.Unlock()
	if pending != nil {
		if err := q.store.SaveJob(*pending); err != nil {
			q.storageFailure(err, pending)
			return err
		}
	}
	if err := q.store.CheckStorage(); err != nil {
		q.storageFailure(err, pending)
		return err
	}
	if active {
		q.Wake()
		return nil
	}
	if err := q.ResumeInterrupted(); err != nil {
		q.storageFailure(err, nil)
		return err
	}
	q.mu.Lock()
	q.fault, q.unsavedFinal, q.faultAt = nil, nil, time.Time{}
	q.mu.Unlock()
	q.Wake()
	return nil
}

func (q *Queue) loop() {
	q.mu.Lock()
	done := q.done
	q.mu.Unlock()
	defer close(done)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	q.cleanupLogsIfDue(false)
	for {
		select {
		case <-q.stop:
			return
		case <-q.wake:
		case <-ticker.C:
		}
		q.cleanupLogsIfDue(false)
		q.mu.Lock()
		active := q.running != "" || q.closed
		missing := q.client.Path == ""
		failed := q.fault != nil
		q.mu.Unlock()
		if active || missing || failed || !q.store.Health().Ready {
			continue
		}
		if err := q.ResumeInterrupted(); err != nil {
			q.storageFailure(err, nil)
			continue
		}
		q.addMu.Lock()
		snapshot := q.store.Snapshot()
		var job Job
		ok := false
		for _, candidate := range PendingJobs(snapshot.Jobs) {
			if WorkflowBlocked(snapshot.Jobs, candidate) || (q.blocked != nil && q.blocked(candidate, snapshot)) {
				continue
			}
			if candidate.NotBefore != nil && time.Now().Before(*candidate.NotBefore) {
				continue
			}
			job, ok = candidate, true
			break
		}
		if !ok || snapshot.QueuePaused {
			q.addMu.Unlock()
			continue
		}
		job.Status = "running" // Reserve before allowing edit/delete/reorder.
		err := q.store.SaveJob(job)
		q.addMu.Unlock()
		if err != nil {
			q.storageFailure(err, nil)
			continue
		}
		q.execute(job)
		q.cleanupLogsIfDue(true)
	}
}

func nextRunnable(jobs []Job) (Job, bool) {
	for _, job := range PendingJobs(jobs) {
		if WorkflowBlocked(jobs, job) {
			continue
		}
		if job.NotBefore != nil && time.Now().Before(*job.NotBefore) {
			return Job{}, false
		}
		return job, true
	}
	return Job{}, false
}

func nextPending(jobs []Job) (Job, bool) {
	pending := PendingJobs(jobs)
	if len(pending) == 0 {
		return Job{}, false
	}
	if pending[0].NotBefore != nil && time.Now().Before(*pending[0].NotBefore) {
		return Job{}, false
	}
	return pending[0], true
}

func (q *Queue) Add(request JobCreateRequest) (Job, error) {
	q.addMu.Lock()
	defer q.addMu.Unlock()
	return q.addLocked(request)
}

func (q *Queue) addLocked(request JobCreateRequest) (Job, error) {
	if request.WorkflowID != "" || request.ProofPath != "" || request.ProofRoot != "" {
		return Job{}, errors.New("use the workflow endpoint for dependent tasks and exclusion proofs")
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey != "" {
		for _, existing := range q.store.Snapshot().Jobs {
			if existing.IdempotencyKey == request.IdempotencyKey {
				return existing, nil
			}
		}
	}
	job, err := q.buildJob(request)
	if err != nil {
		return Job{}, err
	}
	if err := q.store.Update(func(state *State) error {
		job.Position = lastPosition(state.Jobs) + 1
		state.Jobs = append(state.Jobs, job)
		return nil
	}); err != nil {
		return Job{}, err
	}
	q.Wake()
	return job, nil
}

func lastPosition(jobs []Job) int {
	position := 0
	for _, job := range jobs {
		if job.Position > position {
			position = job.Position
		}
	}
	return position
}

func (q *Queue) buildJob(request JobCreateRequest) (Job, error) {
	if request.MaxDelete < 0 || request.MaxDelete > 10000 {
		return Job{}, errors.New("max_delete must be between 0 and 10000")
	}
	if request.MaxDelete == 0 {
		request.MaxDelete = 10
	}
	request.Name = strings.TrimSpace(request.Name)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Operation = strings.TrimSpace(request.Operation)
	request.Source = strings.TrimSpace(request.Source)
	request.Destination = strings.TrimSpace(request.Destination)
	if request.Name == "" {
		return Job{}, errors.New("task name is required")
	}
	if len(request.IdempotencyKey) > 128 || strings.ContainsAny(request.IdempotencyKey, "\x00\r\n") {
		return Job{}, errors.New("invalid idempotency key")
	}
	allowed := map[string]bool{"copy": true, "copyto": true, "sync": true, "check": true,
		"oauth-reconnect": true, "oauth-create": true}
	if !allowed[request.Operation] {
		return Job{}, errors.New("unsupported operation")
	}
	defaults := q.store.Snapshot().Defaults
	if request.Transfers == 0 {
		request.Transfers = defaults.Transfers
	}
	if request.Checkers == 0 {
		request.Checkers = defaults.Checkers
	}
	settings := TransferSettings{
		Transfers: request.Transfers, Checkers: request.Checkers,
		ChunkSize:      ValueOr(request.ChunkSize, defaults.ChunkSize),
		BufferSize:     ValueOr(request.BufferSize, defaults.BufferSize),
		Bandwidth:      strings.TrimSpace(request.Bandwidth),
		CPUParallelism: defaults.CPUParallelism,
	}
	if request.Bandwidth == "" {
		settings.Bandwidth = defaults.Bandwidth
	} else if request.Bandwidth == "off" {
		settings.Bandwidth = ""
	}
	if request.CPUParallelism != nil {
		settings.CPUParallelism = *request.CPUParallelism
	}
	if err := settings.Validate(); err != nil {
		return Job{}, err
	}
	id, err := RandomID()
	if err != nil {
		return Job{}, err
	}
	now := time.Now().UTC()
	job := Job{
		VerificationMode: request.VerificationMode,
		ID:               id, Name: request.Name, IdempotencyKey: request.IdempotencyKey,
		Operation: request.Operation,
		Source:    request.Source, Destination: request.Destination,
		Includes: append([]string(nil), request.Includes...),
		Files:    append([]string(nil), request.Files...),
		Excludes: append([]string(nil), request.Excludes...), Immutable: request.Immutable,
		ChunkSize: settings.ChunkSize, BufferSize: settings.BufferSize,
		CPUParallelism: settings.CPUParallelism,
		WorkflowID:     request.WorkflowID, NotBefore: request.NotBefore,
		WorkflowTotalBytes: request.WorkflowTotalBytes, WorkflowInitialBytes: request.WorkflowInitialBytes,
		ProofPath: request.ProofPath, ProofRoot: request.ProofRoot,
		Transfers: request.Transfers, Checkers: request.Checkers,
		Bandwidth: settings.Bandwidth, Checksum: request.Checksum,
		CreateDirs: request.CreateDirs, AllowDelete: request.AllowDelete,
		DryRun: request.DryRun, MaxDelete: request.MaxDelete,
		Status: "pending", CreatedAt: now, LastUpdatedAt: now,
	}
	if _, err := rcloneTransferArgs(job, 1, ""); err != nil {
		return Job{}, err
	}
	return job, nil
}

func RandomID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (q *Queue) execute(job Job) {
	client := q.engineClient()
	beginAttempt(&job)
	if err := q.validateImmutableSource(job); err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	if err := q.validateProof(job); err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	rcPort, err := freePort()
	if err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	logDir := filepath.Join(q.dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("%s-%d.log", job.ID, job.Attempt))
	if len(job.Files) > 0 {
		if err := os.WriteFile(logPath+".files", []byte(strings.Join(job.Files, "\n")+"\n"), 0o600); err != nil {
			q.finish(job, "failed", nil, err)
			return
		}
	}
	args, err := rcloneTransferArgs(job, rcPort, logPath)
	if err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	logFile, err := openTaskLog(logPath)
	if err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	defer logFile.Close()
	fmt.Fprintf(logFile, "%s START operation=%s attempt=%d\n", time.Now().Format(time.RFC3339), job.Operation, job.Attempt)

	ctx, cancel := context.WithCancel(context.Background())
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		cancel()
		q.finish(job, "interrupted", nil, errors.New("queue closed before launch"))
		return
	}
	q.running = job.ID
	q.cancel = cancel
	q.mu.Unlock()
	defer func() {
		cancel()
		q.mu.Lock()
		q.running = ""
		q.cancel = nil
		q.pausing = false
		q.mu.Unlock()
	}()

	cmd := exec.CommandContext(ctx, client.Path, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = client.environment()
	if job.CPUParallelism > 0 {
		cmd.Env = append(cmd.Env, "GOMAXPROCS="+strconv.Itoa(job.CPUParallelism))
	}
	hideProcess(cmd)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	now := time.Now().UTC()
	job.Status, job.StartedAt, job.PID, job.RCPort = "running", &now, 0, rcPort
	job.ProcessIdentity = ""
	job.LogPath, job.LastUpdatedAt = logPath, now
	if err := q.store.SaveJob(job); err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	if err := cmd.Start(); err != nil {
		q.finish(job, "failed", nil, err)
		return
	}
	job.PID = cmd.Process.Pid
	job.ProcessIdentity = processIdentity(job.PID)
	job.LastUpdatedAt = time.Now().UTC()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if job.ProcessIdentity == "" {
		cancel()
		<-done
		q.finish(job, "failed", nil, errors.New("process stopped because its start identity could not be verified"))
		return
	}
	if err := q.store.SaveJob(job); err != nil {
		q.storageFailure(err, nil)
		cancel()
		<-done
		q.finish(job, "interrupted", nil, errors.New("process stopped because its ownership could not be saved"))
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var saveErr error
	for {
		select {
		case waitErr := <-done:
			if stats := finalLogStats(logPath); stats != nil {
				applyStats(&job, stats)
			}
			exitCode := 0
			if waitErr != nil {
				exitCode = 1
				var exitError *exec.ExitError
				if errors.As(waitErr, &exitError) {
					exitCode = exitError.ExitCode()
				}
			}
			if ctx.Err() != nil {
				if saveErr != nil {
					q.finish(job, "interrupted", &exitCode, errors.New("process stopped because task state could not be saved"))
					return
				}
				q.mu.Lock()
				closing := q.closing
				pausing := q.pausing
				q.mu.Unlock()
				if closing {
					q.finish(job, "interrupted", &exitCode, errors.New("service stopped while job was running"))
				} else if pausing {
					q.finish(job, "paused", &exitCode, nil)
				} else {
					q.finish(job, "cancelled", &exitCode, nil)
				}
				return
			}
			if waitErr != nil {
				q.finish(job, "failed", &exitCode, transferError(waitErr, logPath))
				return
			}
			q.finish(job, "completed", &exitCode, nil)
			return
		case <-ticker.C:
			statsCtx, statsCancel := context.WithTimeout(context.Background(), 2*time.Second)
			stats, statsErr := client.Stats(statsCtx, rcPort)
			statsCancel()
			if statsErr == nil {
				applyStats(&job, stats)
			}
			job.LastUpdatedAt = time.Now().UTC()
			if err := q.store.SaveJob(job); err != nil {
				saveErr = err
				q.storageFailure(err, nil)
				cancel()
			}
		}
	}
}

func (q *Queue) validateImmutableSource(job Job) error {
	if !job.Immutable || job.Operation != "copy" {
		return nil
	}
	isRemote := strings.Contains(job.Source, ":") && !filepath.IsAbs(job.Source)
	if !isRemote {
		info, err := os.Stat(job.Source)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("immutable copy requires a directory source; use files to select individual files")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := q.engineClient().command(ctx, "lsjson", job.Source, "--stat")
	if err != nil {
		return err
	}
	var entry struct {
		IsDir bool `json:"IsDir"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil || !entry.IsDir {
		return errors.New("immutable copy requires a directory source")
	}
	return nil
}

func (q *Queue) setEngine(client RcloneClient) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.client, q.rclone = client, client.Path
}

func (q *Queue) engineClient() RcloneClient {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.client
}

func applyStats(job *Job, stats *RCStats) {
	job.Bytes, job.TotalBytes, job.Speed = stats.Bytes, stats.TotalBytes, stats.Speed
	job.ActiveFiles = stats.Transferring
	job.FilesDone, job.FilesTotal = stats.Transfers, stats.TotalTransfers
	job.ChecksDone, job.ChecksTotal = stats.Checks, stats.TotalChecks
	job.CheckingFiles = append([]string(nil), stats.Checking...)
	job.EngineErrors, job.EngineListed = stats.Errors, stats.Listed
	job.EngineElapsed, job.EngineETA = stats.ElapsedTime, stats.ETA
	job.EngineFatalError, job.EngineRetryError, job.EngineLastError = stats.FatalError, stats.RetryError, stats.LastError
}

func finalLogStats(path string) *RCStats {
	text, err := tailFile(path, 64*1024)
	if err != nil {
		return nil
	}
	var final *RCStats
	for _, line := range strings.Split(text, "\n") {
		var entry struct {
			Stats *RCStats `json:"stats"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Stats != nil {
			final = entry.Stats
		}
	}
	return final
}

func transferError(runErr error, path string) error {
	text, _ := tailFile(path, 16*1024)
	message := ""
	for _, line := range strings.Split(text, "\n") {
		var entry struct {
			Level   string `json:"level"`
			Message string `json:"msg"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && (entry.Level == "error" || entry.Level == "critical" || entry.Level == "fatal") {
			message = entry.Message
		}
	}
	if message != "" {
		return fmt.Errorf("%v: %s", runErr, message)
	}
	return runErr
}

func (q *Queue) finish(job Job, status string, exitCode *int, runErr error) {
	now := time.Now().UTC()
	job.Status = status
	job.ActiveFiles = nil
	job.CheckingFiles = nil
	job.Speed = 0
	job.FinishedAt = &now
	job.LastUpdatedAt = now
	job.ExitCode = exitCode
	if runErr != nil {
		job.Error = runErr.Error()
	}
	if err := q.store.SaveJob(job); err != nil {
		q.storageFailure(err, &job)
	}
	logFile, err := os.OpenFile(job.LogPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintf(logFile, "%s END status=%s error=%s\n", now.Format(time.RFC3339), status, job.Error)
		logFile.Close()
	}
	q.Wake()
}

func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	if q.running == id && q.cancel != nil {
		q.cancel()
		q.mu.Unlock()
		return nil
	}
	q.mu.Unlock()
	return q.store.Update(func(state *State) error {
		for i := range state.Jobs {
			if state.Jobs[i].ID == id && (state.Jobs[i].Status == "pending" || state.Jobs[i].Status == "paused") {
				now := time.Now().UTC()
				state.Jobs[i].Status, state.Jobs[i].FinishedAt = "cancelled", &now
				return nil
			}
		}
		return errors.New("job is not running, pending or paused")
	})
}

func (q *Queue) Pause(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != id || q.cancel == nil {
		return errors.New("job is not running")
	}
	q.pausing = true
	q.cancel()
	return nil
}

func (q *Queue) Retry(id string) error {
	q.addMu.Lock()
	defer q.addMu.Unlock()
	err := q.store.Update(func(state *State) error {
		for i := range state.Jobs {
			job := &state.Jobs[i]
			if job.ID != id {
				continue
			}
			if job.Status == "handoff" || job.Status == "running" || job.Status == "pending" {
				return errors.New("only finished or interrupted jobs can be retried")
			}
			for _, peer := range state.Jobs {
				if job.WorkflowID != "" && peer.WorkflowID == job.WorkflowID && peer.Status == "running" {
					return errors.New("workflow is still running")
				}
			}
			job.Status = "pending"
			if job.WorkflowID == "" {
				job.Position = lastPosition(state.Jobs) + 1
			} else {
				for j := range state.Jobs {
					peer := &state.Jobs[j]
					if peer.WorkflowID == job.WorkflowID && peer.WorkflowStep > job.WorkflowStep && peer.Status == "completed" {
						peer.Status = "pending"
						peer.StartedAt, peer.FinishedAt, peer.ExitCode = nil, nil, nil
						peer.Bytes, peer.TotalBytes, peer.Speed, peer.Error = 0, 0, 0, ""
					}
				}
			}
			job.StartedAt, job.FinishedAt, job.ExitCode = nil, nil, nil
			job.Bytes, job.TotalBytes, job.Speed, job.Error = 0, 0, 0, ""
			job.FilesDone, job.FilesTotal, job.ChecksDone, job.ChecksTotal = 0, 0, 0, 0
			job.PID, job.RCPort = 0, 0
			job.ProcessIdentity = ""
			job.ActiveFiles = nil
			job.CheckingFiles = nil
			job.LastUpdatedAt = time.Now().UTC()
			return nil
		}
		return errors.New("job not found")
	})
	if err == nil {
		q.Wake()
	}
	return err
}

func (q *Queue) ResumeInterrupted() error {
	for _, job := range q.store.Snapshot().Jobs {
		if job.Status != "interrupted" {
			continue
		}
		if interruptedProcessActive(job) {
			continue
		}
		if err := q.Retry(job.ID); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) Delete(id string) error {
	q.addMu.Lock()
	defer q.addMu.Unlock()
	return q.store.Update(func(state *State) error {
		for i, job := range state.Jobs {
			if job.ID == id {
				if job.Status == "running" || job.Status == "handoff" {
					return errors.New("running job cannot be deleted")
				}
				if job.WorkflowID != "" {
					return errors.New("workflow steps must be kept together; delete the workflow instead")
				}
				state.Jobs = append(state.Jobs[:i], state.Jobs[i+1:]...)
				return nil
			}
		}
		return errors.New("job not found")
	})
}

func (q *Queue) DeleteWorkflow(workflowID string) error {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return errors.New("workflow id is required")
	}
	q.addMu.Lock()
	defer q.addMu.Unlock()
	err := q.store.Update(func(state *State) error {
		kept := make([]Job, 0, len(state.Jobs))
		removed := 0
		for _, job := range state.Jobs {
			if job.WorkflowID != workflowID {
				kept = append(kept, job)
				continue
			}
			if job.Status == "running" || job.Status == "handoff" {
				return errors.New("running workflow must be cancelled first")
			}
			removed++
		}
		if removed == 0 {
			return errors.New("workflow not found")
		}
		state.Jobs = kept
		return nil
	})
	if err == nil {
		q.Wake()
	}
	return err
}

func (q *Queue) SetPaused(paused bool) error {
	if !paused {
		if err := q.Recover(); err != nil {
			return err
		}
	}
	q.addMu.Lock()
	defer q.addMu.Unlock()
	if err := q.store.Update(func(state *State) error {
		state.QueuePaused = paused
		return nil
	}); err != nil {
		return err
	}
	if !paused {
		q.Wake()
	}
	return nil
}

func (q *Queue) Move(id string, delta int) error {
	if delta != -1 && delta != 1 {
		return errors.New("move must be -1 or 1")
	}
	q.addMu.Lock()
	defer q.addMu.Unlock()
	return q.store.Update(func(state *State) error {
		pending := PendingJobs(state.Jobs)
		at := -1
		for i, job := range pending {
			if job.ID == id {
				at = i
				break
			}
		}
		if at < 0 {
			return errors.New("only pending jobs can be reordered")
		}
		next := at + delta
		if next < 0 || next >= len(pending) {
			return nil
		}
		if pending[at].WorkflowID != "" && pending[at].WorkflowID == pending[next].WorkflowID {
			return errors.New("workflow steps have a fixed execution order")
		}
		pending[at], pending[next] = pending[next], pending[at]
		positions := map[string]int{
			pending[at].ID:   pending[next].Position,
			pending[next].ID: pending[at].Position,
		}
		for i := range state.Jobs {
			if position, ok := positions[state.Jobs[i].ID]; ok {
				state.Jobs[i].Position = position
			}
		}
		return nil
	})
}

func (q *Queue) JobLog(id string, limit int64) (string, error) {
	if limit <= 0 || limit > 1024*1024 {
		limit = 256 * 1024
	}
	for _, job := range q.store.Snapshot().Jobs {
		if job.ID == id {
			if job.LogPath == "" {
				return "", errors.New("job has no log yet")
			}
			return tailFile(job.LogPath, limit)
		}
	}
	return "", errors.New("job not found")
}

type JobUpdateRequest struct {
	Settings  TransferSettings `json:"settings"`
	NotBefore *time.Time       `json:"not_before,omitempty"`
}

func (q *Queue) UpdateParameters(id string, request JobUpdateRequest) error {
	if err := request.Settings.Validate(); err != nil {
		return err
	}
	q.addMu.Lock()
	defer q.addMu.Unlock()
	return q.store.Update(func(state *State) error {
		for i := range state.Jobs {
			job := &state.Jobs[i]
			if job.ID != id {
				continue
			}
			if job.Status != "pending" {
				return errors.New("parameters can only be changed while a job is pending")
			}
			job.Transfers, job.Checkers = request.Settings.Transfers, request.Settings.Checkers
			job.ChunkSize, job.BufferSize = request.Settings.ChunkSize, request.Settings.BufferSize
			job.CPUParallelism, job.Bandwidth = request.Settings.CPUParallelism, request.Settings.Bandwidth
			job.NotBefore = request.NotBefore
			job.LastUpdatedAt = time.Now().UTC()
			return nil
		}
		return errors.New("job not found")
	})
}
