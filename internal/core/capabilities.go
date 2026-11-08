package core

import (
	"path/filepath"
	"strings"
)

// BackendCapabilities describes the settings whose meaning is backend
// specific. Ordinary transfer buffers and concurrency remain global options.
type BackendCapabilities struct {
	Backend        string `json:"backend"`
	DriveChunkSize bool   `json:"drive_chunk_size"`
}

func CapabilitiesForBackend(backend string) BackendCapabilities {
	return BackendCapabilities{Backend: backend, DriveChunkSize: backend == "drive"}
}

// DestinationCapabilities uses an already loaded remote catalog, avoiding a
// config read or network call during rendering. An unknown remote deliberately
// has no backend-specific controls.
func DestinationCapabilities(destination string, remotes []RemoteInfo) BackendCapabilities {
	if filepath.IsAbs(destination) || filepath.VolumeName(destination) != "" {
		return CapabilitiesForBackend("local")
	}
	name, _, remote := strings.Cut(destination, ":")
	if !remote {
		return CapabilitiesForBackend("local")
	}
	for _, entry := range remotes {
		if entry.Name == name || strings.TrimSuffix(entry.Name, ":") == name {
			return CapabilitiesForBackend(entry.Type)
		}
	}
	return CapabilitiesForBackend("")
}
