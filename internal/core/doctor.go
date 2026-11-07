package core

import (
	"context"
	"os"
	"runtime"
)

type Diagnostic struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	NextAction string `json:"next_action,omitempty"`
}

func (a *App) Doctor(ctx context.Context) []Diagnostic {
	result := []Diagnostic{{ID: "platform", Status: "ready", Message: runtime.GOOS + "/" + runtime.GOARCH}}
	version, err := a.rclone.Version(ctx)
	if err != nil {
		result = append(result, Diagnostic{ID: "engine", Status: "missing", Message: "rclone is unavailable",
			NextAction: "install rclone from rclone.org or select it with -rclone"})
	} else {
		result = append(result, Diagnostic{ID: "engine", Status: "ready", Message: version})
	}
	if a.rclone.ConfigPath != "" {
		_, err := os.Stat(a.rclone.ConfigPath)
		if err != nil {
			result = append(result, Diagnostic{ID: "config", Status: "unavailable", Message: "selected config is not readable"})
		} else {
			result = append(result, Diagnostic{ID: "config", Status: "ready", Message: "application config selected"})
		}
	}
	mounts := "experimental"
	if runtime.GOOS == "windows" {
		mounts = "requires_winfsp"
	}
	result = append(result, Diagnostic{ID: "mount", Status: mounts, Message: "mount support is separate from transfer support",
		NextAction: "check the platform mount dependency before creating a mount"})
	return result
}
