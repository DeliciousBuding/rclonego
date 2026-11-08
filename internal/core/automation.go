package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	observation "github.com/DeliciousBuding/rclonego/internal/automation"
)

const maxAutomations = 32

// Automation is an opt-in trigger for the shared transfer queue. "watch" copies
// a local directory without deleting destination files. "interval" executes an
// existing saved workflow.
type Automation struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Kind            string           `json:"kind"`
	Enabled         bool             `json:"enabled"`
	Source          string           `json:"source,omitempty"`
	Destination     string           `json:"destination,omitempty"`
	WorkflowID      string           `json:"workflow_id,omitempty"`
	Settings        TransferSettings `json:"settings,omitzero"`
	IntervalSeconds int64            `json:"interval_seconds,omitempty"`
	DebounceSeconds int              `json:"debounce_seconds,omitempty"`
	StableSeconds   int              `json:"stable_seconds,omitempty"`
	RescanSeconds   int              `json:"rescan_seconds,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	NextRunAt       *time.Time       `json:"next_run_at,omitempty"`
	LastScanAt      *time.Time       `json:"last_scan_at,omitempty"`
	LastTriggeredAt *time.Time       `json:"last_triggered_at,omitempty"`
	LastSuccessAt   *time.Time       `json:"last_success_at,omitempty"`
	LastJobID       string           `json:"last_job_id,omitempty"`
	LastRunID       string           `json:"last_run_id,omitempty"`
	Status          string           `json:"status"`
	Error           string           `json:"error,omitempty"`
	WatchWarning    string           `json:"watch_warning,omitempty"`

	// These persisted fields make dispatch idempotent across a restart between
	// reserving a trigger and recording the submitted queue job.
	Requested            bool   `json:"run_requested,omitempty"`
	PendingKey           string `json:"pending_key,omitempty"`
	PendingFingerprint   string `json:"pending_fingerprint,omitempty"`
	LastSubmissionKey    string `json:"last_submission_key,omitempty"`
	SubmittedFingerprint string `json:"submitted_fingerprint,omitempty"`
	LastFingerprint      string `json:"last_fingerprint,omitempty"`
}

func cloneAutomations(items []Automation) []Automation {
	result := append([]Automation(nil), items...)
	for i := range result {
		result[i].NextRunAt = cloneTime(items[i].NextRunAt)
		result[i].LastScanAt = cloneTime(items[i].LastScanAt)
		result[i].LastTriggeredAt = cloneTime(items[i].LastTriggeredAt)
		result[i].LastSuccessAt = cloneTime(items[i].LastSuccessAt)
	}
	return result
}

func (a *App) ListAutomations() []Automation {
	items := a.store.automationSnapshot()
	if a.automations != nil {
		a.automations.failureMu.Lock()
		defer a.automations.failureMu.Unlock()
		for i := range items {
			if message := a.automations.failures[items[i].ID]; message != "" {
				items[i].Status, items[i].Error = "attention", message
			}
		}
	}
	return items
}

func (a *App) SaveAutomation(request Automation) (Automation, error) {
	if err := a.validateAutomation(&request); err != nil {
		return Automation{}, err
	}
	if a.automations != nil {
		a.automations.operationMu.Lock()
		defer a.automations.operationMu.Unlock()
		for _, current := range a.store.automationSnapshot() {
			if current.ID != request.ID || !automationConfigChanged(current, request) {
				continue
			}
			active, _, err := a.automations.taskStatus(current)
			if err != nil {
				return Automation{}, err
			}
			if active != "" {
				return Automation{}, errors.New("wait for the automation's current task before changing its paths or schedule")
			}
		}
	}
	now := time.Now().UTC()
	if request.ID == "" {
		id, err := RandomID()
		if err != nil {
			return Automation{}, err
		}
		request.ID = id
	}
	var saved Automation
	err := a.store.Update(func(state *State) error {
		for i, current := range state.Automations {
			if current.ID != request.ID {
				continue
			}
			saved = automationConfiguration(request, current, now)
			state.Automations[i] = saved
			return nil
		}
		if len(state.Automations) >= maxAutomations {
			return fmt.Errorf("at most %d automations are supported", maxAutomations)
		}
		saved = automationConfiguration(request, Automation{ID: request.ID, CreatedAt: now}, now)
		state.Automations = append(state.Automations, saved)
		return nil
	})
	if err == nil {
		a.clearAutomationFailure(saved.ID)
		a.wakeAutomations()
	}
	return saved, err
}

func automationConfiguration(request, current Automation, now time.Time) Automation {
	changed := automationConfigChanged(current, request)
	current.ID, current.Name, current.Kind, current.Enabled = request.ID, request.Name, request.Kind, request.Enabled
	current.Source, current.Destination, current.WorkflowID = request.Source, request.Destination, request.WorkflowID
	current.Settings = request.Settings
	current.IntervalSeconds, current.DebounceSeconds = request.IntervalSeconds, request.DebounceSeconds
	current.StableSeconds, current.RescanSeconds = request.StableSeconds, request.RescanSeconds
	current.UpdatedAt = now
	if changed {
		current.LastFingerprint, current.SubmittedFingerprint = "", ""
		current.PendingKey, current.PendingFingerprint = "", ""
		current.LastSubmissionKey, current.LastJobID, current.LastRunID = "", "", ""
		current.Requested = false
		current.NextRunAt = nil
		current.Error, current.WatchWarning = "", ""
	}
	if !current.Enabled && !automationBusyStatus(current.Status) {
		current.Status = "disabled"
	} else if current.Status == "" || current.Status == "disabled" {
		current.Status = "waiting"
	}
	return current
}

func automationConfigChanged(a, b Automation) bool {
	return a.Kind != b.Kind || a.Source != b.Source || a.Destination != b.Destination ||
		a.WorkflowID != b.WorkflowID || a.Settings != b.Settings || a.IntervalSeconds != b.IntervalSeconds ||
		a.DebounceSeconds != b.DebounceSeconds || a.StableSeconds != b.StableSeconds || a.RescanSeconds != b.RescanSeconds
}

func (a *App) validateAutomation(request *Automation) error {
	request.Name = strings.TrimSpace(request.Name)
	request.Source, request.Destination = strings.TrimSpace(request.Source), strings.TrimSpace(request.Destination)
	if request.Name == "" || len(request.Name) > 200 || validateSafeText(request.Name) != nil {
		return errors.New("automation requires a name of at most 200 characters")
	}
	if len(request.ID) > 40 || validateSafeText(request.ID) != nil {
		return errors.New("invalid automation id")
	}
	switch request.Kind {
	case "watch":
		if request.Settings.Transfers == 0 {
			request.Settings = a.Snapshot().Defaults
		}
		if err := request.Settings.Validate(); err != nil {
			return err
		}
		if !filepath.IsAbs(request.Source) || request.Destination == "" || validateSafeText(request.Destination) != nil {
			return errors.New("watch requires an absolute local source directory and a destination")
		}
		source, err := filepath.EvalSymlinks(request.Source)
		if err != nil {
			return fmt.Errorf("watch source: %w", err)
		}
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			return errors.New("watch source must be an existing local directory")
		}
		request.Source = filepath.Clean(source)
		if data, err := filepath.EvalSymlinks(a.dataDir); err == nil && pathsOverlap(request.Source, data) {
			return errors.New("automatic copy source must not contain or be inside RcloneGo's private state directory")
		}
		if filepath.IsAbs(request.Destination) {
			destination, err := resolveAutomationDestination(request.Destination)
			if err != nil {
				return err
			}
			if pathsOverlap(request.Source, destination) {
				return errors.New("automatic copy source and destination must not overlap")
			}
			request.Destination = destination
		} else if !strings.Contains(request.Destination, ":") || strings.HasPrefix(request.Destination, ":") {
			return errors.New("automatic copy destination must be an absolute local path or configured remote path")
		}
		request.WorkflowID, request.IntervalSeconds = "", 0
		if request.DebounceSeconds == 0 {
			request.DebounceSeconds = 3
		}
		if request.StableSeconds == 0 {
			request.StableSeconds = 2
		}
		if request.RescanSeconds == 0 {
			request.RescanSeconds = 300
		}
		if request.DebounceSeconds < 1 || request.DebounceSeconds > 3600 ||
			request.StableSeconds < 1 || request.StableSeconds > 300 ||
			request.RescanSeconds < 10 || request.RescanSeconds > 86400 {
			return errors.New("watch debounce must be 1–3600s, stability 1–300s and rescan 10–86400s")
		}
	case "interval":
		if request.Settings != (TransferSettings{}) {
			return errors.New("interval automations use the saved workflow's settings; edit that workflow to change parameters")
		}
		found := false
		for _, workflow := range a.Snapshot().Workflows {
			if workflow.ID == request.WorkflowID {
				found = true
			}
		}
		if !found {
			return errors.New("interval automation requires an existing workflow_id")
		}
		if request.IntervalSeconds < 60 || request.IntervalSeconds > 365*86400 {
			return errors.New("automation interval must be 60 seconds to 365 days")
		}
		request.Source, request.Destination = "", ""
		request.DebounceSeconds, request.StableSeconds, request.RescanSeconds = 0, 0, 0
	default:
		return errors.New("automation kind must be watch or interval")
	}
	return nil
}

func resolveAutomationDestination(path string) (string, error) {
	absolute := filepath.Clean(path)
	existing := absolute
	var suffix []string
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", errors.New("automatic copy destination has no existing parent")
		}
		suffix = append(suffix, filepath.Base(existing))
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, suffix[i])
	}
	return resolved, nil
}

func pathsOverlap(a, b string) bool {
	if runtime.GOOS == "windows" {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (a *App) DeleteAutomation(id string) error {
	if a.automations != nil {
		a.automations.operationMu.Lock()
		defer a.automations.operationMu.Unlock()
	}
	err := a.store.Update(func(state *State) error {
		for i, current := range state.Automations {
			if current.ID == id {
				state.Automations = append(state.Automations[:i], state.Automations[i+1:]...)
				return nil
			}
		}
		return errors.New("automation not found")
	})
	if err == nil {
		a.clearAutomationFailure(id)
		a.wakeAutomations()
	}
	return err
}

func (a *App) TriggerAutomation(id string) error {
	if a.automations != nil {
		a.automations.operationMu.Lock()
		defer a.automations.operationMu.Unlock()
	}
	err := a.editAutomation(id, func(current *Automation) {
		current.Requested, current.Error = true, ""
		if !automationBusyStatus(current.Status) {
			current.Status = "waiting"
		}
	})
	if err == nil {
		a.clearAutomationFailure(id)
		a.wakeAutomations()
	}
	return err
}

func (a *App) editAutomation(id string, edit func(*Automation)) error {
	return a.store.Update(func(state *State) error {
		for i := range state.Automations {
			if state.Automations[i].ID == id {
				edit(&state.Automations[i])
				return nil
			}
		}
		return errors.New("automation not found")
	})
}

func (s *Store) automationSnapshot() []Automation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := cloneAutomations(s.data.Automations)
	if items == nil {
		return []Automation{}
	}
	return items
}

func (a *App) clearAutomationFailure(id string) {
	if a.automations != nil {
		a.automations.failureMu.Lock()
		delete(a.automations.failures, id)
		a.automations.failureMu.Unlock()
	}
}

type automationObservation struct {
	monitor     *observation.Monitor
	source      string
	fingerprint string
	observedAt  time.Time
	scanAfter   time.Time
	nextRescan  time.Time
}

type automationService struct {
	app         *App
	mu          sync.Mutex
	operationMu sync.Mutex
	failureMu   sync.Mutex
	failures    map[string]string
	observed    map[string]*automationObservation
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
	now         func() time.Time
	scan        func(context.Context, string) (observation.Snapshot, error)
	newMonitor  func() (*observation.Monitor, error)
}

func newAutomationService(app *App) *automationService {
	return &automationService{app: app, observed: map[string]*automationObservation{}, failures: map[string]string{}, wake: make(chan struct{}, 1),
		now: time.Now, scan: observation.Scan, newMonitor: observation.NewMonitor}
}

func (a *App) startAutomations() {
	if a.automations == nil {
		return
	}
	a.automations.start()
}

func (a *App) stopAutomations() {
	if a.automations != nil {
		a.automations.stop()
	}
}

func (a *App) wakeAutomations() {
	if a.automations == nil {
		return
	}
	select {
	case a.automations.wake <- struct{}{}:
	default:
	}
}

func (s *automationService) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		s.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				s.closeMonitors()
				return
			case <-ticker.C:
			case <-s.wake:
			}
			s.tick(ctx)
		}
	}()
}

func (s *automationService) stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *automationService) closeMonitors() {
	for id, state := range s.observed {
		if state.monitor != nil {
			state.monitor.Close()
		}
		delete(s.observed, id)
	}
}

func (s *automationService) tick(ctx context.Context) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	now := s.now().UTC()
	current := s.app.store.automationSnapshot()
	exists := map[string]bool{}
	for _, item := range current {
		exists[item.ID] = true
		if ctx.Err() != nil {
			break
		}
		s.process(ctx, item, now)
	}
	for id, state := range s.observed {
		if !exists[id] {
			if state.monitor != nil {
				state.monitor.Close()
			}
			delete(s.observed, id)
		}
	}
}

func automationBusyStatus(status string) bool {
	return status == "queued" || status == "running" || status == "paused" || status == "interrupted"
}

func (s *automationService) process(ctx context.Context, item Automation, now time.Time) {
	if !s.app.Health().Ready {
		return
	}
	if item.Error != "" {
		return
	}
	active, terminal, err := s.taskStatus(item)
	if err != nil {
		s.fail(item.ID, err)
		return
	}
	if active != "" {
		if item.Status != active {
			s.update(item.ID, func(current *Automation) { current.Status = active })
		}
		return
	}
	if terminal != "" && (automationBusyStatus(item.Status) || item.LastSuccessAt == nil ||
		(item.LastTriggeredAt != nil && item.LastSuccessAt.Before(*item.LastTriggeredAt))) {
		if terminal != "completed" {
			if !item.Requested {
				s.fail(item.ID, fmt.Errorf("last automatic task %s; retry or run it explicitly", terminal))
				return
			}
		} else {
			if !s.update(item.ID, func(current *Automation) {
				current.LastSuccessAt = &now
				current.LastFingerprint = current.SubmittedFingerprint
				current.Status = "waiting"
				if current.Kind == "interval" {
					next := now.Add(time.Duration(current.IntervalSeconds) * time.Second)
					current.NextRunAt = &next
				}
			}) {
				return
			}
			item.LastSuccessAt, item.LastFingerprint, item.Status = &now, item.SubmittedFingerprint, "waiting"
			if item.Kind == "interval" {
				next := now.Add(time.Duration(item.IntervalSeconds) * time.Second)
				item.NextRunAt = &next
			}
		}
	}
	if !item.Enabled && !item.Requested && item.PendingKey == "" {
		if state := s.observed[item.ID]; state != nil {
			if state.monitor != nil {
				state.monitor.Close()
			}
			delete(s.observed, item.ID)
		}
		if item.Status != "disabled" {
			s.update(item.ID, func(current *Automation) { current.Status = "disabled" })
		}
		return
	}
	if item.PendingKey != "" {
		s.dispatch(item, now)
		return
	}
	if item.Kind == "interval" {
		if !item.Requested {
			if item.NextRunAt == nil {
				next := now.Add(time.Duration(item.IntervalSeconds) * time.Second)
				s.update(item.ID, func(current *Automation) { current.NextRunAt, current.Status = &next, "waiting" })
				return
			}
			if now.Before(*item.NextRunAt) {
				return
			}
		}
		s.reserve(item, "", now)
		return
	}
	s.processWatch(ctx, item, now)
}

func (s *automationService) taskStatus(item Automation) (active, terminal string, err error) {
	if item.LastSubmissionKey == "" {
		return "", "", nil
	}
	if !automationBusyStatus(item.Status) && item.LastSuccessAt != nil && item.LastTriggeredAt != nil &&
		!item.LastSuccessAt.Before(*item.LastTriggeredAt) {
		return "", "", nil
	}
	status := ""
	if item.LastJobID != "" {
		job, e := s.app.store.JobByKey(item.LastSubmissionKey)
		if e != nil {
			return "", "", e
		}
		if job.ID == "" {
			return "", "", errors.New("automatic task record is unavailable")
		}
		status = job.Status
	} else if item.LastRunID != "" {
		report, e := s.app.ReportRun(item.LastRunID)
		if e != nil {
			return "", "", e
		}
		status = report.Status
	}
	if status == "pending" {
		return "queued", "", nil
	}
	if status == "running" || status == "paused" || status == "interrupted" {
		return status, "", nil
	}
	return "", status, nil
}

func (s *automationService) processWatch(ctx context.Context, item Automation, now time.Time) {
	state := s.observed[item.ID]
	if state == nil || state.source != item.Source {
		if state != nil && state.monitor != nil {
			state.monitor.Close()
		}
		state = &automationObservation{source: item.Source}
		monitor, err := s.newMonitor()
		if err == nil {
			state.monitor = monitor
		} else {
			s.update(item.ID, func(current *Automation) {
				current.WatchWarning = "file notifications unavailable; periodic scan remains enabled"
			})
		}
		s.observed[item.ID] = state
	}
	if state.monitor != nil && state.monitor.Changed() {
		state.scanAfter = now.Add(time.Duration(item.DebounceSeconds) * time.Second)
	}
	periodicDue := state.nextRescan.IsZero() || !now.Before(state.nextRescan)
	changeDue := !state.scanAfter.IsZero() && !now.Before(state.scanAfter)
	stabilityDue := state.fingerprint != "" && (item.Requested || state.fingerprint != item.LastFingerprint) &&
		!now.Before(state.observedAt.Add(time.Duration(item.StableSeconds)*time.Second))
	if !periodicDue && !changeDue && !stabilityDue {
		return
	}
	if !state.scanAfter.IsZero() && now.Before(state.scanAfter) {
		return
	}
	scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := s.scan(scanCtx, item.Source)
	cancel()
	if err != nil {
		if ctx.Err() == nil {
			s.fail(item.ID, fmt.Errorf("automatic scan: %w", err))
		}
		return
	}
	warning := ""
	if state.monitor != nil {
		if err := state.monitor.SyncDirectories(snapshot.Directories); err != nil {
			warning = "some directories could not be watched; periodic scan remains enabled"
		} else if state.monitor.Warning() != nil {
			warning = "file notifications were lost; periodic scan remains enabled"
		}
	} else {
		warning = "file notifications unavailable; periodic scan remains enabled"
	}
	state.nextRescan, state.scanAfter = now.Add(time.Duration(item.RescanSeconds)*time.Second), time.Time{}
	if state.fingerprint != snapshot.Fingerprint {
		state.fingerprint, state.observedAt = snapshot.Fingerprint, now
	}
	if !s.update(item.ID, func(current *Automation) {
		current.LastScanAt, current.WatchWarning, current.Status = &now, warning, "watching"
	}) {
		return
	}
	if snapshot.Fingerprint == item.LastFingerprint && !item.Requested {
		return
	}
	if now.Before(state.observedAt.Add(time.Duration(item.StableSeconds) * time.Second)) {
		return
	}
	s.reserve(item, snapshot.Fingerprint, now)
}

func (s *automationService) reserve(item Automation, fingerprint string, now time.Time) {
	id, err := RandomID()
	if err != nil {
		s.fail(item.ID, err)
		return
	}
	key := "auto-" + item.ID + "-" + id
	if !s.update(item.ID, func(current *Automation) {
		current.PendingKey, current.PendingFingerprint = key, fingerprint
		current.Status = "waiting"
	}) {
		return
	}
	item.PendingKey, item.PendingFingerprint = key, fingerprint
	s.dispatch(item, now)
}

func (s *automationService) dispatch(item Automation, now time.Time) {
	jobID, runID := "", ""
	if item.Kind == "watch" {
		settings := item.Settings
		job, err := s.app.AddJob(JobCreateRequest{
			Name: item.Name, IdempotencyKey: item.PendingKey,
			Operation: "copy", Source: item.Source, Destination: item.Destination,
			Transfers: settings.Transfers, Checkers: settings.Checkers, ChunkSize: settings.ChunkSize,
			BufferSize: settings.BufferSize, Bandwidth: settings.Bandwidth, CPUParallelism: &settings.CPUParallelism,
			CreateDirs: true,
		})
		if err != nil {
			s.fail(item.ID, err)
			return
		}
		jobID = job.ID
	} else {
		run, err := s.app.StartWorkflow(item.WorkflowID, item.PendingKey)
		if err != nil {
			s.fail(item.ID, err)
			return
		}
		runID = run.ID
	}
	s.update(item.ID, func(current *Automation) {
		current.LastJobID, current.LastRunID = jobID, runID
		current.LastSubmissionKey, current.SubmittedFingerprint = item.PendingKey, item.PendingFingerprint
		current.PendingKey, current.PendingFingerprint = "", ""
		current.LastTriggeredAt, current.Requested, current.Status = &now, false, "queued"
	})
}

func (s *automationService) fail(id string, err error) {
	s.update(id, func(current *Automation) {
		current.Error, current.Status = err.Error(), "attention"
	})
}

func (s *automationService) update(id string, edit func(*Automation)) bool {
	err := s.app.editAutomation(id, edit)
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	if err != nil {
		s.failures[id] = "could not persist automation state: " + err.Error()
		return false
	}
	delete(s.failures, id)
	return true
}
