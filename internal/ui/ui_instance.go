package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// Use MyGo's activation forwarding while the core lock remains responsible for
// exclusive state ownership. Independent profiles have independent UI instances.
func ClaimInstance(dataDirectory string) bool {
	mygo.App.SetName(instanceName(dataDirectory))
	claimed := mygo.App.RequestSingleInstanceLock()
	mygo.App.SetName("RcloneGo")
	return claimed
}

func instanceName(directory string) string {
	if absolute, err := filepath.Abs(directory); err == nil {
		directory = absolute
	}
	if resolved, err := filepath.EvalSymlinks(directory); err == nil {
		directory = resolved
	}
	if runtime.GOOS == "windows" {
		directory = strings.ToLower(directory)
	}
	hash := sha256.Sum256([]byte(filepath.Clean(directory)))
	return "RcloneGo-" + hex.EncodeToString(hash[:12])
}
