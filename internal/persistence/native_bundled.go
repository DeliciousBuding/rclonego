//go:build rclonego_embedded

package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var nativeOnce sync.Once
var nativeError error

// The build tool embeds the platform library so a standalone executable works
// alone. Packaged builds carry MyGo's signed copy beside the executable or in
// the macOS bundle; prefer that copy and extract only when it is absent.
func prepareNative() error {
	nativeOnce.Do(func() {
		if packagedNativeLibrary() {
			return
		}
		hash := sha256.Sum256(bundledLibrary)
		digest := hex.EncodeToString(hash[:])
		cache, err := os.UserCacheDir()
		if err != nil {
			nativeError = err
			return
		}
		dir := filepath.Join(cache, "rclonego", "native", digest)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			nativeError = err
			return
		}
		path := filepath.Join(dir, bundledLibraryName)
		if existing, err := os.ReadFile(path); err == nil && sha256.Sum256(existing) == hash {
			nativeError = os.Setenv("MYGO_NATIVE_SQLITECORE", path)
			return
		}
		file, err := os.CreateTemp(dir, ".library-*")
		if err != nil {
			nativeError = err
			return
		}
		defer os.Remove(file.Name())
		if err = file.Chmod(0o600); err == nil {
			_, err = file.Write(bundledLibrary)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(file.Name(), path)
		}
		if err != nil {
			nativeError = errors.New("could not prepare embedded SQLite library")
			return
		}
		nativeError = os.Setenv("MYGO_NATIVE_SQLITECORE", path)
	})
	return nativeError
}

// packagedNativeLibrary reports whether MyGo's own library lookup can find a
// copy shipped with the application instead of the executable's embedded copy.
func packagedNativeLibrary() bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	dir := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(dir, bundledLibraryName),
		filepath.Join(dir, "..", "Resources", bundledLibraryName),
		filepath.Join(dir, "..", "Frameworks", bundledLibraryName),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}
