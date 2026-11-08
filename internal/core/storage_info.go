package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type StorageCapabilities struct {
	Quota            bool `json:"quota"`
	ServerSideCopy   bool `json:"server_side_copy"`
	ServerSideMove   bool `json:"server_side_move"`
	PublicLink       bool `json:"public_link"`
	EmptyDirectories bool `json:"empty_directories"`
	RecursiveListing bool `json:"recursive_listing"`
	ReadMetadata     bool `json:"read_metadata"`
	WriteMetadata    bool `json:"write_metadata"`
	CaseInsensitive  bool `json:"case_insensitive"`
}

type StorageInfo struct {
	Name         string              `json:"name"`
	Hashes       []string            `json:"hashes"`
	PrecisionNS  int64               `json:"precision_ns"`
	Capabilities StorageCapabilities `json:"capabilities"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

type DirectoryUsage struct {
	Path      string    `json:"path"`
	Count     int64     `json:"count"`
	Bytes     int64     `json:"bytes"`
	Sizeless  int64     `json:"sizeless"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *App) StorageInfo(ctx context.Context, name string) (StorageInfo, error) {
	name = strings.TrimSuffix(name, ":")
	if !remotePattern.MatchString(name + ":") {
		return StorageInfo{}, errors.New("use a configured remote name without a directory")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client := a.EngineClient()
	if client.Path == "" {
		return StorageInfo{}, errors.New("rclone engine is not installed")
	}
	raw, err := client.command(ctx, "backend", "features", name+":")
	if ctx.Err() != nil {
		return StorageInfo{}, ctx.Err()
	}
	if err != nil {
		return StorageInfo{}, err
	}
	return decodeStorageInfo(name, raw)
}

func decodeStorageInfo(name string, raw []byte) (StorageInfo, error) {
	var data struct {
		Hashes    []string
		Precision int64
		Features  map[string]bool
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return StorageInfo{}, fmt.Errorf("decode storage capabilities: %w", err)
	}
	f := data.Features
	return StorageInfo{Name: name, Hashes: data.Hashes, PrecisionNS: data.Precision, UpdatedAt: time.Now(),
		Capabilities: StorageCapabilities{
			Quota: f["About"], ServerSideCopy: f["Copy"], ServerSideMove: f["Move"],
			PublicLink: f["PublicLink"], EmptyDirectories: f["CanHaveEmptyDirectories"],
			RecursiveListing: f["ListR"], ReadMetadata: f["ReadMetadata"],
			WriteMetadata: f["WriteMetadata"], CaseInsensitive: f["CaseInsensitive"],
		}}, nil
}

// DirectoryUsage is a user-requested recursive inventory, never UI polling.
// A canceled or timed-out scan returns an error instead of a partial total.
func (a *App) DirectoryUsage(ctx context.Context, value string) (DirectoryUsage, error) {
	root, _, _, err := browsePath(value)
	if err != nil {
		return DirectoryUsage{}, err
	}
	client := a.EngineClient()
	if client.Path == "" {
		return DirectoryUsage{}, errors.New("rclone engine is not installed")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	raw, err := client.command(ctx, "size", root, "--json", "--retries", "1")
	if ctx.Err() != nil {
		return DirectoryUsage{}, ctx.Err()
	}
	if err != nil {
		return DirectoryUsage{}, err
	}
	result := DirectoryUsage{Path: root, UpdatedAt: time.Now()}
	if err := json.Unmarshal(raw, &result); err != nil {
		return DirectoryUsage{}, fmt.Errorf("decode directory usage: %w", err)
	}
	return result, nil
}
