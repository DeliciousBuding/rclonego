package core

import (
	"context"
	"runtime"
	"time"
)

// DiagnosticReport intentionally excludes locations, names, tokens, logs,
// environment variables, process command lines and account configuration.
type DiagnosticReport struct {
	GeneratedAt   time.Time        `json:"generated_at"`
	Platform      string           `json:"platform"`
	EngineVersion string           `json:"engine_version"`
	QueuePaused   bool             `json:"queue_paused"`
	JobCounts     map[string]int   `json:"job_counts"`
	AccountCount  int              `json:"account_count"`
	MountCount    int              `json:"mount_count"`
	Resources     ResourceSnapshot `json:"resources"`
}

func (a *App) Diagnostics(ctx context.Context) DiagnosticReport {
	state := a.Snapshot()
	snapshot := a.RuntimeSnapshot()
	resources := snapshot.Resources
	for i := range resources.Processes {
		if resources.Processes[i].PID == 0 {
			continue
		}
		resources.Processes[i].PID = 0
		resources.Processes[i].Name, resources.Processes[i].Error = "", ""
	}
	resources.Error = ""
	report := DiagnosticReport{GeneratedAt: time.Now().UTC(), Platform: runtime.GOOS + "/" + runtime.GOARCH,
		EngineVersion: snapshot.Version, QueuePaused: state.QueuePaused, JobCounts: map[string]int{},
		AccountCount: len(snapshot.Remotes), MountCount: len(state.Mounts), Resources: resources}
	for _, job := range state.Jobs {
		report.JobCounts[job.Status]++
	}
	return report
}
