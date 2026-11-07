package core

import (
	"context"
	"errors"
)

// App owns the state and scheduler shared by every user interface.
type App struct {
	store   *Store
	queue   *Queue
	rclone  RcloneClient
	dataDir string
	runtime *Runtime
}

func NewApp(store *Store, rclonePath, dataDir string) *App {
	app := &App{store: store, rclone: RcloneClient{Path: rclonePath}, dataDir: dataDir}
	app.queue = NewQueue(store, rclonePath, dataDir)
	app.runtime = newRuntime(store, app.rclone)
	app.queue.externalBusy = app.runtime.Busy
	return app
}

func (a *App) Start() {
	a.runtime.Start()
	_ = a.queue.ResumeInterrupted()
	a.queue.Start()
}

func (a *App) Close() {
	a.queue.Close()
	a.runtime.Close()
}

func (a *App) Snapshot() State                  { return a.store.Snapshot() }
func (a *App) RuntimeSnapshot() RuntimeSnapshot { return a.runtime.Snapshot() }
func (a *App) RefreshRuntime()                  { a.runtime.Refresh() }
func (a *App) UpdateState(fn func(*State) error) error {
	return a.store.Update(fn)
}
func (a *App) AddJob(request JobCreateRequest) (Job, error) { return a.queue.Add(request) }
func (a *App) AddWorkflow(request WorkflowRequest) ([]Job, error) {
	return a.queue.AddWorkflow(request)
}
func (a *App) UpdateParameters(id string, request JobUpdateRequest) error {
	return a.queue.UpdateParameters(id, request)
}
func (a *App) Retry(id string) error           { return a.queue.Retry(id) }
func (a *App) Cancel(id string) error          { return a.queue.Cancel(id) }
func (a *App) Delete(id string) error          { return a.queue.Delete(id) }
func (a *App) DeleteWorkflow(id string) error  { return a.queue.DeleteWorkflow(id) }
func (a *App) Move(id string, delta int) error { return a.queue.Move(id, delta) }
func (a *App) SetPaused(paused bool) error     { return a.queue.SetPaused(paused) }
func (a *App) Wake()                           { a.queue.Wake() }
func (a *App) JobLog(id string, limit int64) (string, error) {
	return a.queue.JobLog(id, limit)
}
func (a *App) Remotes(ctx context.Context) ([]RemoteInfo, error) {
	return a.rclone.Remotes(ctx)
}
func (a *App) InspectMounts(ctx context.Context) ([]MountView, error) {
	return inspectMounts(ctx, a.store.Snapshot().Mounts)
}
func (a *App) StartMount(ctx context.Context, cfg MountConfig) (int, error) {
	pid, err := startMount(ctx, a.rclone, a.dataDir, cfg)
	if err != nil {
		return pid, err
	}
	err = a.store.Update(func(state *State) error {
		if state.OwnedMounts == nil {
			state.OwnedMounts = map[string]int{}
		}
		state.OwnedMounts[cfg.ID] = pid
		return nil
	})
	return pid, err
}
func (a *App) StopMount(ctx context.Context, cfg MountConfig) error {
	pid := a.Snapshot().OwnedMounts[cfg.ID]
	if pid == 0 {
		return errors.New("this instance does not own the mount")
	}
	if err := stopMount(ctx, cfg, pid, a.rclone.RCToken); err != nil {
		return err
	}
	return a.store.Update(func(state *State) error { delete(state.OwnedMounts, cfg.ID); return nil })
}
