// Package rclonego embeds distribution notices for the standalone executable.
package rclonego

import (
	"embed"
	"io/fs"
)

//go:embed LICENSE NOTICE third_party/licenses/*
var notices embed.FS

// LicenseDocuments returns public license texts without depending on sidecars.
func LicenseDocuments() map[string]string {
	result := map[string]string{}
	_ = fs.WalkDir(notices, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			if raw, err := notices.ReadFile(path); err == nil {
				result[path] = string(raw)
			}
		}
		return nil
	})
	return result
}
