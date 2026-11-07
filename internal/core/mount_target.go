package core

import (
	"path/filepath"
	"runtime"
	"strings"
)

func NormalizeMountTarget(target string) string {
	target = strings.TrimSpace(target)
	if runtime.GOOS == "windows" {
		return strings.ToUpper(target)
	}
	return filepath.Clean(target)
}

func MountTargetsEqual(a, b string) bool {
	return NormalizeMountTarget(a) == NormalizeMountTarget(b)
}
