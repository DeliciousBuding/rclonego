package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/engine"
)

func (r RcloneClient) environment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "RCLONE_") {
			continue
		}
		result = append(result, entry)
	}
	if r.ConfigPath != "" {
		result = append(result, "RCLONE_CONFIG="+r.ConfigPath)
	}
	if r.RCToken != "" {
		result = append(result, "RCLONE_RC_USER=rclonego", "RCLONE_RC_PASS="+r.RCToken)
	}
	return result
}

// ConfigureEngine must be called before Start.
func (a *App) ConfigureEngine(configPath, rcToken string) {
	a.engineMu.Lock()
	defer a.engineMu.Unlock()
	a.rclone.ConfigPath, a.rclone.RCToken = configPath, rcToken
	a.queue.setEngine(a.rclone)
	a.runtime.setEngine(a.rclone)
}

func (a *App) EngineClient() RcloneClient {
	a.engineMu.RLock()
	defer a.engineMu.RUnlock()
	return a.rclone
}

func (a *App) EngineStatus(ctx context.Context) engine.Installation {
	client := a.EngineClient()
	version, _ := client.Version(ctx)
	result := engine.Installation{Path: client.Path, Version: version, Source: "external", Platform: runtime.GOOS + "/" + runtime.GOARCH}
	if selected, err := a.engines.Selected(); err == nil && selected.Path == client.Path {
		result.Source, result.SHA256 = selected.Source, selected.SHA256
	}
	return result
}

func (a *App) InstallEngine(ctx context.Context, archive string) (engine.Installation, error) {
	installation, err := a.engines.Install(ctx, archive)
	if err != nil {
		return installation, err
	}
	return a.selectEngine(ctx, installation)
}

func (a *App) SelectEngine(ctx context.Context, path string) (engine.Installation, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return engine.Installation{}, err
	}
	return a.selectEngine(ctx, engine.Installation{Path: path, Source: "external", Platform: runtime.GOOS + "/" + runtime.GOARCH})
}

func (a *App) selectEngine(ctx context.Context, installation engine.Installation) (engine.Installation, error) {
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := (RcloneClient{Path: installation.Path}).Version(verifyCtx)
	if err != nil || !strings.HasPrefix(version, "rclone v") {
		return installation, errors.New("selected executable is not a usable rclone engine")
	}
	installation.Version = version
	a.engineMu.Lock()
	defer a.engineMu.Unlock()
	if err := a.engines.Select(installation); err != nil {
		return installation, err
	}
	a.rclone.Path = installation.Path
	a.queue.setEngine(a.rclone)
	a.runtime.setEngine(a.rclone)
	a.runtime.Refresh()
	a.queue.Wake()
	return installation, nil
}
