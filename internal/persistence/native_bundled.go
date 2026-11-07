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

// The build tool embeds the platform library. Extract it to a private,
// content-addressed cache before SQLite opens; portable users need only the exe.
func prepareNative() error {
	nativeOnce.Do(func() {
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
			nativeError = os.Setenv("MYGO_SQLITE_LIBRARY", path)
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
		nativeError = os.Setenv("MYGO_SQLITE_LIBRARY", path)
	})
	return nativeError
}
