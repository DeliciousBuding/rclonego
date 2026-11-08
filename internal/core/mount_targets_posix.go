//go:build linux || darwin

package core

import "path/filepath"

func availableMountTargets(dataDir string) MountTargets {
	return MountTargets{Available: []string{}, Suggested: filepath.Join(dataDir, "mountpoints", "cloud")}
}
