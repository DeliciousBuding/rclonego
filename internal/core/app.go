package core

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/DeliciousBuding/rclonego/internal/engine"
	"github.com/DeliciousBuding/rclonego/internal/persistence"
)

// App owns the state and scheduler shared by every user interface.
type App struct {
	store      *Store
	queue      *Queue
	rclone     RcloneClient
	dataDir    string
	runtime    *Runtime
	engineMu   sync.RWMutex
	engines    *engine.Manager
	configMu   sync.Mutex
	accountsMu sync.Mutex
	accounts   map[string]*accountSession
	previewMu  sync.Mutex
	previews   map[string]Preview
	mountMu    sync.Mutex
}

func NewApp(store *Store, rclonePath, dataDir string) *App {
	app := &App{store: store, rclone: RcloneClient{Path: rclonePath}, dataDir: dataDir,
		engines: &engine.Manager{Dir: filepath.Join(dataDir, "engines")}}
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
	a.accountsMu.Lock()
	ids := make([]string, 0, len(a.accounts))
	for id := range a.accounts {
		ids = append(ids, id)
	}
	a.accountsMu.Unlock()
	for _, id := range ids {
		_ = a.CancelAccount(id)
	}
	a.queue.Close()
	a.runtime.Close()
}

func (a *App) Snapshot() State                  { return a.store.Snapshot() }
func (a *App) RuntimeSnapshot() RuntimeSnapshot { return a.runtime.Snapshot() }
func (a *App) RefreshRuntime()                  { a.runtime.Refresh() }
func (a *App) UpdateState(fn func(*State) error) error {
	return a.store.Update(fn)
}
func (a *App) AddJob(request JobCreateRequest) (Job, error) {
	if request.IdempotencyKey != "" {
		job, err := a.store.JobByKey(request.IdempotencyKey)
		if err != nil {
			return Job{}, err
		}
		if job.ID != "" {
			return job, nil
		}
	}
	if err := a.confirmPreview(request); err != nil {
		return Job{}, err
	}
	return a.queue.Add(request)
}
func (a *App) AddWorkflow(request WorkflowRequest) ([]Job, error) {
	if request.IdempotencyKey != "" {
		state, err := a.store.RunState(request.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if len(state.Runs) > 0 {
			if state.Runs[0].WorkflowID != request.DefinitionID {
				return nil, errors.New("idempotency key belongs to another workflow")
			}
			return state.Jobs, nil
		}
	}
	return a.queue.AddWorkflow(request)
}
func (a *App) UpdateParameters(id string, request JobUpdateRequest) error {
	return a.queue.UpdateParameters(id, request)
}
func (a *App) Retry(id string) error           { return a.queue.Retry(id) }
func (a *App) Cancel(id string) error          { return a.queue.Cancel(id) }
func (a *App) Pause(id string) error           { return a.queue.Pause(id) }
func (a *App) Delete(id string) error          { return a.queue.Delete(id) }
func (a *App) DeleteWorkflow(id string) error  { return a.queue.DeleteWorkflow(id) }
func (a *App) Move(id string, delta int) error { return a.queue.Move(id, delta) }
func (a *App) SetPaused(paused bool) error     { return a.queue.SetPaused(paused) }
func (a *App) Wake()                           { a.queue.Wake() }
func (a *App) Events(after int64, limit int) ([]persistence.Event, error) {
	return a.store.Events(after, limit)
}
func (a *App) JobLog(id string, limit int64) (string, error) {
	return a.queue.JobLog(id, limit)
}
func (a *App) Remotes(ctx context.Context) ([]RemoteInfo, error) {
	return a.EngineClient().Remotes(ctx)
}
func (a *App) InspectMounts(ctx context.Context) ([]MountView, error) {
	return inspectMounts(ctx, a.store.Snapshot().Mounts)
}
func (a *App) StartMount(ctx context.Context, cfg MountConfig) (int, error) {
	a.mountMu.Lock()
	defer a.mountMu.Unlock()
	pid, err := startMount(ctx, a.EngineClient(), a.dataDir, cfg)
	if err != nil {
		return pid, err
	}
	err = a.store.Update(func(state *State) error {
		if state.OwnedMounts == nil {
			state.OwnedMounts = map[string]int{}
		}
		state.OwnedMounts[cfg.ID] = pid
		if state.MountIdentities == nil {
			state.MountIdentities = map[string]string{}
		}
		state.MountIdentities[cfg.ID] = processIdentity(pid)
		return nil
	})
	return pid, err
}
func (a *App) StopMount(ctx context.Context, cfg MountConfig) error {
	a.mountMu.Lock()
	defer a.mountMu.Unlock()
	state := a.Snapshot()
	pid := state.OwnedMounts[cfg.ID]
	if pid == 0 {
		return errors.New("this instance does not own the mount")
	}
	if recorded := state.MountIdentities[cfg.ID]; recorded == "" || recorded != processIdentity(pid) {
		return errors.New("mount process identity changed or is unavailable; refusing to stop another process")
	}
	if err := stopMount(ctx, cfg, pid, a.EngineClient().RCToken); err != nil {
		return err
	}
	return a.store.Update(func(state *State) error {
		delete(state.OwnedMounts, cfg.ID)
		delete(state.MountIdentities, cfg.ID)
		return nil
	})
}
