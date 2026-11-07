package core

import (
	"context"
	"os"
	"sync"
	"time"
)

type RuntimeSnapshot struct {
	UpdatedAt        time.Time              `json:"updated_at"`
	External         []ExternalTransferView `json:"external"`
	Resources        ResourceSnapshot       `json:"resources"`
	Version          string                 `json:"version"`
	Remotes          []RemoteInfo           `json:"remotes"`
	Mounts           []MountView            `json:"mounts"`
	CatalogError     string                 `json:"catalog_error,omitempty"`
	QueueBlockReason string                 `json:"queue_block_reason,omitempty"`
}

type Runtime struct {
	store       *Store
	rclone      RcloneClient
	mu          sync.RWMutex
	snapshot    RuntimeSnapshot
	cancel      context.CancelFunc
	done        chan struct{}
	resourceAt  time.Time
	resources   ResourceSnapshot
	cpuReadings map[int]cpuReading
	catalogAt   time.Time
	wake        chan struct{}
}

func newRuntime(store *Store, rclone RcloneClient) *Runtime {
	return &Runtime{store: store, rclone: rclone, cpuReadings: map[int]cpuReading{}, wake: make(chan struct{}, 1)}
}

func (r *Runtime) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			r.refresh(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-r.wake:
				r.catalogAt = time.Time{}
			}
		}
	}()
}

func (r *Runtime) Refresh() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runtime) Close() {
	if r.cancel != nil {
		r.cancel()
		<-r.done
	}
}

func (r *Runtime) refresh(ctx context.Context) {
	next := RuntimeSnapshot{UpdatedAt: time.Now()}
	state := r.store.Snapshot()
	for _, external := range state.ExternalTransfers {
		view := ExternalTransferView{ExternalTransfer: external, UpdatedAt: next.UpdatedAt}
		stats, err := r.rclone.Stats(ctx, external.RCPort)
		if err == nil {
			view.Available = true
			view.Stats = stats
			view.Tree = TransferTree(stats.Transferring)
		}
		next.External = append(next.External, view)
		if view.Available || (external.OwnerPID > 0 && processExists(external.OwnerPID)) {
			next.QueueBlockReason = external.Name + "运行中，新队列等待接续"
		}
	}
	for _, job := range state.Jobs {
		if job.Status == "interrupted" && job.PID > 0 && processExists(job.PID) {
			next.QueueBlockReason = "等待上次任务的进程结束"
		}
	}
	if time.Since(r.resourceAt) > 15*time.Second {
		r.resources, r.cpuReadings = sampleResources(ctx, os.Getpid(), r.cpuReadings)
		r.resourceAt = time.Now()
	}
	next.Resources = r.resources
	old := r.Snapshot()
	next.Version, next.Remotes, next.CatalogError = old.Version, old.Remotes, old.CatalogError
	if time.Since(r.catalogAt) > time.Minute {
		catalogCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		next.Version, _ = r.rclone.Version(catalogCtx)
		remotes, err := r.rclone.Remotes(catalogCtx)
		cancel()
		next.Remotes = remotes
		if err != nil {
			next.CatalogError = err.Error()
		} else {
			next.CatalogError = ""
		}
		r.catalogAt = time.Now()
	}
	next.Mounts, _ = inspectMounts(ctx, state.Mounts)
	r.mu.Lock()
	r.snapshot = next
	r.mu.Unlock()
}

func (r *Runtime) Snapshot() RuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := r.snapshot
	result.External = append([]ExternalTransferView(nil), result.External...)
	result.Remotes = append([]RemoteInfo(nil), result.Remotes...)
	result.Mounts = append([]MountView(nil), result.Mounts...)
	result.Resources.Processes = append([]ProcessResource(nil), result.Resources.Processes...)
	return result
}

func (r *Runtime) Busy() bool {
	state := r.store.Snapshot()
	for _, job := range state.Jobs {
		if job.Status == "interrupted" && job.PID > 0 && processExists(job.PID) {
			return true
		}
	}
	views := r.Snapshot().External
	for _, external := range state.ExternalTransfers {
		if external.OwnerPID > 0 && processExists(external.OwnerPID) {
			return true
		}
		for _, view := range views {
			if view.ID == external.ID && view.Available {
				return true
			}
		}
	}
	return false
}
