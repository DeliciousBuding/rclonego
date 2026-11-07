package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Settings describe rclone concurrency, not OS threads. A single queue worker
// owns transfers; rclone schedules its own goroutines and system threads.
type TransferSettings struct {
	Transfers      int    `json:"transfers"`
	Checkers       int    `json:"checkers"`
	ChunkSize      string `json:"chunk_size"`
	BufferSize     string `json:"buffer_size"`
	Bandwidth      string `json:"bandwidth"`
	CPUParallelism int    `json:"cpu_parallelism"`
}

func defaultTransferSettings() TransferSettings {
	return TransferSettings{Transfers: 2, Checkers: 4, ChunkSize: "16M", BufferSize: "8M", CPUParallelism: 2}
}

func sizeBytes(value string) (int64, error) {
	if !bandwidthPattern.MatchString(value) {
		return 0, errors.New("use a size such as 8M or 512K")
	}
	multiplier := float64(1)
	if last := value[len(value)-1]; last >= 'A' {
		multiplier = math.Pow(1024, float64(strings.IndexByte("KMGTP", last)+1))
		value = value[:len(value)-1]
	}
	number, err := strconv.ParseFloat(value, 64)
	scaled := number * multiplier
	if err != nil || scaled > math.MaxInt64 || scaled < 0 {
		return 0, errors.New("size is out of range")
	}
	return int64(scaled), nil
}

func (s TransferSettings) Validate() error {
	if s.CPUParallelism < 0 || s.CPUParallelism > 64 {
		return errors.New("CPU parallelism must be 0 (automatic) or 1–64")
	}
	if s.Transfers < 1 || s.Transfers > 32 || s.Checkers < 1 || s.Checkers > 64 {
		return errors.New("transfers must be 1–32 and checkers 1–64")
	}
	chunk, err := sizeBytes(s.ChunkSize)
	if err != nil || chunk < 256*1024 || chunk > 1024*1024*1024 || chunk&(chunk-1) != 0 {
		return errors.New("Drive chunk size must be a power of two from 256K to 1G")
	}
	buffer, err := sizeBytes(s.BufferSize)
	if err != nil || buffer > 1024*1024*1024 {
		return errors.New("buffer size must be between 0 and 1G")
	}
	if s.Bandwidth != "" {
		if _, err := sizeBytes(s.Bandwidth); err != nil {
			return fmt.Errorf("bandwidth: %w", err)
		}
	}
	return nil
}

func (s TransferSettings) BufferBudget() int64 {
	chunk, _ := sizeBytes(s.ChunkSize)
	buffer, _ := sizeBytes(s.BufferSize)
	return int64(s.Transfers) * (chunk + buffer)
}

type SettingsRequest struct {
	Appearance *string           `json:"appearance,omitempty"`
	Defaults   *TransferSettings `json:"defaults,omitempty"`
}

func (a *App) SaveSettings(request SettingsRequest) error {
	if request.Appearance == nil && request.Defaults == nil {
		return errors.New("no settings supplied")
	}
	if request.Appearance != nil && !ValidAppearance(*request.Appearance) {
		return errors.New("appearance must be light, dark or system")
	}
	if request.Defaults != nil {
		if err := request.Defaults.Validate(); err != nil {
			return err
		}
	}
	return a.store.Update(func(state *State) error {
		if request.Appearance != nil {
			state.Appearance = *request.Appearance
		}
		if request.Defaults != nil {
			state.Defaults = *request.Defaults
		}
		return nil
	})
}

func (a *App) SetBandwidth(ctx context.Context, id, rate string) error {
	if rate != "off" {
		if _, err := sizeBytes(rate); err != nil {
			return fmt.Errorf("bandwidth: %w", err)
		}
	}
	port := 0
	state := a.store.Snapshot()
	for _, job := range state.Jobs {
		if job.ID == id && job.Status == "running" {
			port = job.RCPort
		}
	}
	for _, external := range state.ExternalTransfers {
		if external.ID == id {
			port = external.RCPort
		}
	}
	if port == 0 {
		return errors.New("running transfer not found")
	}
	if err := a.rclone.SetBandwidth(ctx, port, rate); err != nil {
		return err
	}
	return a.store.Update(func(state *State) error {
		for i := range state.Jobs {
			if state.Jobs[i].ID == id {
				state.Jobs[i].Bandwidth = rate
				if rate == "off" {
					state.Jobs[i].Bandwidth = ""
				}
			}
		}
		return nil
	})
}
