package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Missing values stay nil: an unknown quota is different from zero free space.
// These are account-level figures returned by the provider, not folder sizes.
type RemoteQuota struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Total     *int64    `json:"total,omitempty"`
	Used      *int64    `json:"used,omitempty"`
	Free      *int64    `json:"free,omitempty"`
	Trashed   *int64    `json:"trashed,omitempty"`
	Other     *int64    `json:"other,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *App) RemoteQuota(ctx context.Context, name string) (RemoteQuota, error) {
	name = strings.TrimSuffix(name, ":")
	if !remotePattern.MatchString(name + ":") {
		return RemoteQuota{}, errors.New("use a configured remote name without a directory")
	}
	client := a.EngineClient()
	if client.Path == "" {
		return RemoteQuota{}, errors.New("rclone engine is not installed")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// about queries the backend's quota API. Never fall back to a recursive size
	// scan, which is both expensive and a different measurement.
	raw, err := client.command(ctx, "about", name+":", "--json", "--retries", "1")
	if ctx.Err() != nil {
		return RemoteQuota{}, ctx.Err()
	}
	if err != nil {
		if strings.Contains(err.Error(), "doesn't support about") || strings.Contains(err.Error(), "does not support about") {
			return RemoteQuota{Name: name, Status: "unsupported", UpdatedAt: time.Now()}, nil
		}
		return RemoteQuota{}, err
	}
	return decodeRemoteQuota(name, raw)
}

func decodeRemoteQuota(name string, raw []byte) (RemoteQuota, error) {
	result := RemoteQuota{Name: name, Status: "available", UpdatedAt: time.Now()}
	if err := json.Unmarshal(raw, &result); err != nil {
		return RemoteQuota{}, fmt.Errorf("decode account quota: %w", err)
	}
	for _, field := range []**int64{&result.Total, &result.Used, &result.Free, &result.Trashed, &result.Other} {
		if *field != nil && **field < 0 {
			*field = nil
		}
	}
	if result.Total == nil && result.Used == nil && result.Free == nil {
		result.Status = "unsupported"
	}
	return result, nil
}
