package core

import (
	"path/filepath"
	"strings"
)

type MountTargets struct {
	Available  []string   `json:"available"`
	Suggested  string     `json:"suggested"`
	Dependency Diagnostic `json:"dependency"`
}

func (a *App) MountTargets() MountTargets {
	targets := availableMountTargets(a.dataDir)
	targets.Dependency = mountDependency()
	reserved := map[string]bool{}
	for _, mount := range a.Snapshot().Mounts {
		reserved[NormalizeMountTarget(mount.Drive)] = true
	}
	available := targets.Available[:0]
	for _, target := range targets.Available {
		if !reserved[NormalizeMountTarget(target)] {
			available = append(available, target)
		}
	}
	targets.Available = available
	if len(available) > 0 {
		targets.Suggested = available[0]
	}
	return targets
}

func (a *App) DefaultMountCache() string { return filepath.Join(a.dataDir, "cache") }

func (a *App) OwnsMount(id string) bool {
	state := a.Snapshot()
	pid := state.OwnedMounts[id]
	return pid > 0 && state.MountIdentities[id] != "" && state.MountIdentities[id] == processIdentity(pid)
}

// A task addressed through a mounted drive still shares the remote's files.
// Resolve its route for queue overlap checks without adopting the mount.
func mountedTransferPath(value string, mounts []MountView) string {
	if !filepath.IsAbs(value) {
		return value
	}
	for _, mount := range mounts {
		if !mount.DriveAvailable {
			continue
		}
		root := mount.Drive
		if len(root) == 2 && strings.HasSuffix(root, ":") {
			root += string(filepath.Separator)
		}
		relative, err := filepath.Rel(root, value)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if relative == "." {
			return mount.Remote
		}
		target := mount.Remote
		if !strings.HasSuffix(target, ":") && !strings.HasSuffix(target, "/") {
			target += "/"
		}
		return target + filepath.ToSlash(relative)
	}
	return value
}
