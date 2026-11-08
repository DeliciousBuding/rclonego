//go:build !darwin

package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

func platformLanguagePreference(dataDirectory string) string {
	raw, err := os.ReadFile(filepath.Join(dataDirectory, "locale"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
