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
	LogPath    string     `json:"-"`
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
		for _, saved := range state.Workflows {
			if saved.ID == definition.ID {
				return errors.New("workflow ID already exists")
			}
		}
		state.Workflows = append(state.Workflows, definition)
		return nil
	})
	return definition, err
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
		rank := map[string]int{"completed": 0, "pending": 1, "running": 2, "cancelled": 3, "interrupted": 4, "failed": 5}
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
