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
	client := a.EngineClient()
	version, err := client.Version(ctx)
	if err != nil {
		result = append(result, Diagnostic{ID: "engine", Status: "missing", Message: "rclone is unavailable",
			NextAction: "use engine install, engine select, or the engine setup in Resources"})
	} else {
		result = append(result, Diagnostic{ID: "engine", Status: "ready", Message: version})
	}
	if client.ConfigPath != "" {
		_, err := os.Stat(client.ConfigPath)
		if err != nil {
			result = append(result, Diagnostic{ID: "config", Status: "unavailable", Message: "selected config is not readable"})
		} else {
			result = append(result, Diagnostic{ID: "config", Status: "ready", Message: "application config selected"})
		}
	}
	result = append(result, mountDependency())
	return result
}
