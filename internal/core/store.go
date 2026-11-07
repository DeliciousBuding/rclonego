package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/persistence"
)

type Store struct {
	mu        sync.RWMutex
	db        *persistence.DB
	data      State
	persisted map[string]string
}

func (s *Store) Close() error { return s.db.Close() }

func OpenStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	db, err := persistence.Open(filepath.Join(dataDir, "state.db"))
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, persisted: map[string]string{}}
	records, err := db.Load()
	if err != nil {
		db.Close()
		return nil, err
	}
	if len(records["settings"]) == 0 {
		store.data = State{Version: stateVersion, Appearance: "light", Defaults: defaultTransferSettings()}
		if err := store.saveLocked(); err != nil {
			db.Close()
			return nil, err
		}
		return store, nil
	}
	if err := json.Unmarshal(records["settings"][0], &store.data); err != nil {
		db.Close()
		return nil, fmt.Errorf("decode state: %w", err)
	}
	for table, output := range map[string]any{"steps": &store.data.Jobs, "workflows": &store.data.Workflows, "runs": &store.data.Runs, "attempts": &store.data.Attempts} {
		raw, _ := json.Marshal(records[table])
		if err := json.Unmarshal(raw, output); err != nil {
			db.Close()
			return nil, err
		}
	}
	if store.data.Version != stateVersion {
		db.Close()
		return nil, fmt.Errorf("unsupported state schema %d", store.data.Version)
	}
	for _, job := range store.data.Jobs {
		store.persisted[job.ID] = job.Status
	}
	changed := store.recoverInterrupted()
	if store.data.Defaults.Transfers == 0 {
		store.data.Defaults = defaultTransferSettings()
		changed = true
	}
	for i := range store.data.Jobs {
		if store.data.Jobs[i].Position == 0 {
			store.data.Jobs[i].Position = i + 1
			changed = true
		}
	}
	if !ValidAppearance(store.data.Appearance) {
		store.data.Appearance = "light"
		changed = true
	}
	if changed {
		if err := store.saveLocked(); err != nil {
			db.Close()
			return nil, err
		}
	}
	return store, nil
}

func ValidAppearance(value string) bool {
	return value == "light" || value == "dark" || value == "system"
}

func (s *Store) recoverInterrupted() bool {
	changed := false
	now := time.Now().UTC()
	for i := range s.data.Jobs {
		if s.data.Jobs[i].Status == "running" {
			s.data.Jobs[i].Status = "interrupted"
			s.data.Jobs[i].Error = "service restarted while job was running"
			s.data.Jobs[i].FinishedAt = &now
			s.data.Jobs[i].LastUpdatedAt = now
			s.data.Jobs[i].Speed = 0
			s.data.Jobs[i].ActiveFiles = nil
			s.data.Jobs[i].CheckingFiles = nil
			s.recordAttemptLocked(s.data.Jobs[i])
			changed = true
		}
	}
	return changed
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.data)
}

func cloneState(data State) State {
	result := data
	if data.MountIdentities != nil {
		result.MountIdentities = map[string]string{}
		for id, identity := range data.MountIdentities {
			result.MountIdentities[id] = identity
		}
	}
	if data.OwnedMounts != nil {
		result.OwnedMounts = map[string]int{}
		for id, pid := range data.OwnedMounts {
			result.OwnedMounts[id] = pid
		}
	}
	result.Mounts = append([]MountConfig(nil), data.Mounts...)
	result.ExternalTransfers = append([]ExternalTransfer(nil), data.ExternalTransfers...)
	result.Jobs = append([]Job(nil), data.Jobs...)
	result.Runs = append([]Run(nil), data.Runs...)
	result.Attempts = append([]Attempt(nil), data.Attempts...)
	result.Workflows = append([]WorkflowDefinition(nil), data.Workflows...)
	for i := range result.Workflows {
		result.Workflows[i].Entries = append([]string(nil), data.Workflows[i].Entries...)
	}
	for i := range result.Jobs {
		result.Jobs[i].NotBefore = cloneTime(data.Jobs[i].NotBefore)
		result.Jobs[i].StartedAt = cloneTime(data.Jobs[i].StartedAt)
		result.Jobs[i].FinishedAt = cloneTime(data.Jobs[i].FinishedAt)
		if data.Jobs[i].ExitCode != nil {
			value := *data.Jobs[i].ExitCode
			result.Jobs[i].ExitCode = &value
		}
		result.Jobs[i].ActiveFiles = append([]RCTransfer(nil), data.Jobs[i].ActiveFiles...)
		result.Jobs[i].CheckingFiles = append([]string(nil), data.Jobs[i].CheckingFiles...)
		result.Jobs[i].Includes = append([]string(nil), data.Jobs[i].Includes...)
		result.Jobs[i].Files = append([]string(nil), data.Jobs[i].Files...)
		result.Jobs[i].Excludes = append([]string(nil), data.Jobs[i].Excludes...)
	}
	for i := range result.Attempts {
		result.Attempts[i].FinishedAt = cloneTime(data.Attempts[i].FinishedAt)
	}
	return result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (s *Store) Events(after int64, limit int) ([]persistence.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db.Events(after, limit)
}

func (s *Store) Runs(offset, limit int) ([]Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records, err := s.db.Runs(offset, limit)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(records)
	var runs []Run
	err = json.Unmarshal(raw, &runs)
	return runs, err
}

func (s *Store) RunState(id string) (State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records, err := s.db.RunRecords(id)
	if err != nil {
		return State{}, err
	}
	var state State
	for table, output := range map[string]any{"runs": &state.Runs, "steps": &state.Jobs, "attempts": &state.Attempts} {
		raw, _ := json.Marshal(records[table])
		if err := json.Unmarshal(raw, output); err != nil {
			return State{}, err
		}
	}
	return state, nil
}

func (s *Store) JobByKey(key string) (Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, err := s.db.JobByKey(key)
	if err != nil || raw == nil {
		return Job{}, err
	}
	var job Job
	err = json.Unmarshal(raw, &job)
	return job, err
}

func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := cloneState(s.data)
	if err := fn(&s.data); err != nil {
		s.data = before
		return err
	}
	if err := s.saveLocked(); err != nil {
		s.data = before
		return err
	}
	return nil
}

func (s *Store) SaveJob(job Job) error {
	return s.Update(func(state *State) error {
		s.recordAttemptLocked(job)
		for i := range state.Jobs {
			if state.Jobs[i].ID == job.ID {
				if state.Jobs[i].Status == "running" {
					job.Bandwidth = state.Jobs[i].Bandwidth
				}
				state.Jobs[i] = job
				return nil
			}
		}
		state.Jobs = append(state.Jobs, job)
		return nil
	})
}

func (s *Store) saveLocked() error {
	settings := s.data
	settings.Jobs, settings.Workflows, settings.Runs, settings.Attempts = nil, nil, nil, nil
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	records := map[string][]persistence.Record{}
	var events []persistence.Event
	for _, job := range s.data.Jobs {
		payload, _ := json.Marshal(job)
		records["steps"] = append(records["steps"], persistence.Record{ID: job.ID, Parent: job.WorkflowID,
			Status: job.Status, Position: job.Position, Payload: payload})
		if old, ok := s.persisted[job.ID]; !ok || old != job.Status {
			events = append(events, persistence.Event{Subject: job.ID, Kind: job.Status})
		}
	}
	for _, definition := range s.data.Workflows {
		payload, _ := json.Marshal(definition)
		records["workflows"] = append(records["workflows"], persistence.Record{ID: definition.ID, Payload: payload})
	}
	for _, run := range s.data.Runs {
		payload, _ := json.Marshal(run)
		records["runs"] = append(records["runs"], persistence.Record{ID: run.ID, Parent: run.WorkflowID, Payload: payload})
	}
	for _, attempt := range s.data.Attempts {
		payload, _ := json.Marshal(attempt)
		records["attempts"] = append(records["attempts"], persistence.Record{ID: attempt.ID, Parent: attempt.StepID, Payload: payload})
	}
	if err := s.db.Save(raw, records, events); err != nil {
		return err
	}
	s.persisted = map[string]string{}
	for _, job := range s.data.Jobs {
		s.persisted[job.ID] = job.Status
	}
	return nil
}

func (s *Store) recordAttemptLocked(job Job) {
	if job.Attempt == 0 || job.StartedAt == nil {
		return
	}
	attempt := attemptForJob(job)
	for i := range s.data.Attempts {
		if s.data.Attempts[i].ID == attempt.ID {
			s.data.Attempts[i] = attempt
			return
		}
	}
	s.data.Attempts = append(s.data.Attempts, attempt)
}
