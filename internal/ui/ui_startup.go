package desktop

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DeliciousBuding/rclonego/internal/session"
)

// MyGo's system startup entry launches the executable without profile flags.
// Only the default profile can register it without opening a different profile
// at the next login.
func (s *nativeUI) canConfigureLogin() bool {
	standard, err := session.DefaultDataDir()
	if err != nil {
		return false
	}
	return sameDataDirectory(s.app.DataDirectory(), standard)
}

func sameDataDirectory(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
