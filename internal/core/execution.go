package core

import (
	"errors"
	"fmt"
	"time"
)

// WorkflowDefinition is reusable; Run identifies one concrete execution.
type WorkflowDefinition struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Source           string           `json:"source"`
	Destination      string           `json:"destination"`
	Entries          []string         `json:"entries,omitempty"`
	VerificationMode string           `json:"verification_mode,omitempty"`
	Settings         TransferSettings `json:"settings"`
	CreatedAt        time.Time        `json:"created_at"`
}

type Run struct {
	ID         string    `json:"id"`
	WorkflowID string    `json:"workflow_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Attempt struct {
	ID         string     `json:"id"`
	StepID     string     `json:"step_id"`
	Number     int        `json:"number"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Bytes      int64      `json:"bytes"`
	Checks     int64      `json:"checks"`
	Error      string     `json:"error,omitempty"`
	LogPath    string     `json:"log_path,omitempty"`
}

type RunReport struct {
	Run          Run       `json:"run"`
	Status       string    `json:"status"`
	Steps        []Job     `json:"steps"`
	Attempts     []Attempt `json:"attempts"`
	WireBytes    int64     `json:"wire_bytes"`
	Verification string    `json:"verification"`
	Scope        string    `json:"scope"`
}

func (a *App) SaveWorkflow(definition WorkflowDefinition) (WorkflowDefinition, error) {
	if definition.Name == "" || definition.Source == "" || definition.Destination == "" {
		return definition, errors.New("name, source and destination are required")
	}
	if definition.Settings.Transfers == 0 {
		definition.Settings = a.Snapshot().Defaults
	}
	if err := definition.Settings.Validate(); err != nil {
		return definition, err
	}
	if definition.VerificationMode != "" && definition.VerificationMode != "backend" && definition.VerificationMode != "content" && definition.VerificationMode != "size" {
		return definition, errors.New("verification_mode must be backend, content or size")
	}
	if definition.ID == "" {
		id, err := RandomID()
		if err != nil {
			return definition, err
		}
		definition.ID = id
	}
	definition.CreatedAt = time.Now().UTC()
	err := a.store.Update(func(state *State) error {
		for i, saved := range state.Workflows {
			if saved.ID == definition.ID {
				definition.CreatedAt = saved.CreatedAt
				state.Workflows[i] = definition
				return nil
			}
		}
		state.Workflows = append(state.Workflows, definition)
		return nil
	})
	return definition, err
}

func (a *App) DeleteDefinition(id string) error {
	return a.store.Update(func(state *State) error {
		for i := range state.Workflows {
			if state.Workflows[i].ID == id {
				state.Workflows = append(state.Workflows[:i], state.Workflows[i+1:]...)
				return nil
			}
		}
		return errors.New("workflow definition not found")
	})
}

func (a *App) ActOnRun(id, action string) error {
	report, err := a.ReportRun(id)
	if err != nil {
		return err
	}
	switch action {
	case "pause":
		for _, job := range report.Steps {
			if job.Status == "running" {
				return a.Pause(job.ID)
			}
		}
		return errors.New("run has no active step")
	case "cancel":
		a.queue.addMu.Lock()
		defer a.queue.addMu.Unlock()
		err := a.store.Update(func(state *State) error {
			now := time.Now().UTC()
			for i := range state.Jobs {
				job := &state.Jobs[i]
				if job.WorkflowID == id && (job.Status == "pending" || job.Status == "paused") {
					job.Status, job.FinishedAt, job.LastUpdatedAt = "cancelled", &now, now
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, job := range report.Steps {
			if job.Status == "running" {
				return a.Cancel(job.ID)
			}
		}
		return nil
	case "retry", "resume":
		if report.Status == "running" || report.Status == "pending" {
			return errors.New("run is already active")
		}
		a.queue.addMu.Lock()
		defer a.queue.addMu.Unlock()
		err := a.store.Update(func(state *State) error {
			for _, job := range state.Jobs {
				if job.WorkflowID == id && job.Status == "running" {
					return errors.New("run is already active")
				}
			}
			changed := false
			for i := range state.Jobs {
				job := &state.Jobs[i]
				if job.WorkflowID != id || job.Status == "completed" {
					continue
				}
				changed = true
				job.Status, job.Error, job.Speed = "pending", "", 0
				job.StartedAt, job.FinishedAt, job.ExitCode = nil, nil, nil
				job.Bytes, job.TotalBytes, job.FilesDone, job.FilesTotal, job.ChecksDone, job.ChecksTotal = 0, 0, 0, 0, 0, 0
				job.PID, job.RCPort = 0, 0
				job.ActiveFiles, job.CheckingFiles = nil, nil
				job.LastUpdatedAt = time.Now().UTC()
			}
			if !changed {
				return errors.New("completed runs should be executed again as a new run")
			}
			return nil
		})
		if err == nil {
			a.Wake()
		}
		return err
	default:
		return errors.New("run action must be pause, resume, retry or cancel")
	}
}

func (a *App) StartWorkflow(id, key string) (Run, error) {
	for _, definition := range a.Snapshot().Workflows {
		if definition.ID != id {
			continue
		}
		if key == "" {
			var err error
			key, err = RandomID()
			if err != nil {
				return Run{}, err
			}
		}
		if state, err := a.store.RunState(key); err != nil {
			return Run{}, err
		} else if len(state.Runs) > 0 {
			if state.Runs[0].WorkflowID != definition.ID {
				return Run{}, errors.New("idempotency key belongs to another workflow")
			}
			return state.Runs[0], nil
		}
		for _, run := range a.Snapshot().Runs {
			if run.ID == key && run.WorkflowID != definition.ID {
				return Run{}, errors.New("idempotency key belongs to another workflow")
			}
		}
		_, err := a.AddWorkflow(WorkflowRequest{
			Name: definition.Name, Source: definition.Source, Destination: definition.Destination,
			Entries: definition.Entries, Settings: &definition.Settings, IdempotencyKey: key,
			DefinitionID:     definition.ID,
			VerificationMode: definition.VerificationMode,
		})
		if err != nil {
			return Run{}, err
		}
		for _, run := range a.Snapshot().Runs {
			if run.ID == key {
				return run, nil
			}
		}
		return Run{}, errors.New("run was not persisted")
	}
	return Run{}, errors.New("workflow not found")
}

func (a *App) ReportRun(id string) (RunReport, error) {
	state := a.Snapshot()
	found := false
	for _, run := range state.Runs {
		if run.ID == id {
			found = true
			break
		}
	}
	if !found {
		var err error
		state, err = a.store.RunState(id)
		if err != nil {
			return RunReport{}, err
		}
	}
	result := RunReport{Status: "completed", Verification: "not_run",
		Scope: "one-way: only source entries; extra destination files are outside this report"}
	for _, run := range state.Runs {
		if run.ID == id {
			result.Run = run
		}
	}
	if result.Run.ID == "" {
		return result, errors.New("run not found")
	}
	stepIDs := map[string]bool{}
	for _, job := range state.Jobs {
		if job.WorkflowID != id {
			continue
		}
		stepIDs[job.ID] = true
		result.Steps = append(result.Steps, job)
		rank := map[string]int{"completed": 0, "pending": 1, "running": 2, "paused": 3, "cancelled": 4, "interrupted": 5, "failed": 6}
		if rank[job.Status] > rank[result.Status] {
			result.Status = job.Status
		}
		if job.Operation == "check" && job.Status == "completed" {
			if job.VerificationMode == "content" {
				result.Verification = "content_compared"
			} else if job.VerificationMode == "size" {
				result.Verification = "size_compared"
			} else {
				result.Verification = "backend_check_completed: size and available hashes; hash availability not attested"
			}
		}
	}
	for _, attempt := range state.Attempts {
		if stepIDs[attempt.StepID] {
			result.Attempts = append(result.Attempts, attempt)
			result.WireBytes += attempt.Bytes
		}
	}
	if len(result.Steps) == 0 {
		result.Status = "empty"
	}
	return result, nil
}

func (a *App) ListRuns(offset, limit int) ([]Run, error) { return a.store.Runs(offset, limit) }

func beginAttempt(job *Job) {
	job.Attempt++
	now := time.Now().UTC()
	job.StartedAt = &now
	job.LastUpdatedAt = now
}

func attemptForJob(job Job) Attempt {
	return Attempt{
		ID: fmt.Sprintf("%s-%d", job.ID, job.Attempt), StepID: job.ID, Number: job.Attempt,
		Status: job.Status, StartedAt: *job.StartedAt, FinishedAt: job.FinishedAt,
		Bytes: job.Bytes, Checks: job.ChecksDone, Error: job.Error, LogPath: job.LogPath,
	}
}
