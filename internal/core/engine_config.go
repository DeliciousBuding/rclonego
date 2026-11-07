package core

import (
	"os"
	"strings"
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
	a.rclone.ConfigPath, a.rclone.RCToken = configPath, rcToken
	a.queue.client = a.rclone
	a.runtime.rclone = a.rclone
}
