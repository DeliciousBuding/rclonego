// Package updates verifies release artifacts in MyGo's signed update format.
// The application itself uses the official MyGo updater.
package updates

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxManifestSize = 1 << 20

type signedFile struct {
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
}

type manifest struct {
	Version string `json:"version"`
	signedFile
	Deltas []struct {
		From string `json:"from"`
		signedFile
	} `json:"deltas,omitempty"`
}

// VerifyDirectory checks every MyGo update manifest against the embedded
// public key and the artifacts beside it. It never downloads release files.
func VerifyDirectory(directory, publicKey, version string, required bool) (int, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return 0, errors.New("invalid MyGo update public key")
	}
	paths, err := filepath.Glob(filepath.Join(directory, "update-*.json"))
	if err != nil {
		return 0, err
	}
	if required && len(paths) == 0 {
		return 0, errors.New("signed updates are required but no MyGo manifest was produced")
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
		closeErr := file.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return 0, err
		}
		if len(data) > maxManifestSize {
			return 0, fmt.Errorf("%s exceeds MyGo's manifest limit", filepath.Base(path))
		}
		var release manifest
		if err := json.Unmarshal(data, &release); err != nil {
			return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if release.Version == "" || version != "" && release.Version != version {
			return 0, fmt.Errorf("%s has an unexpected release version", filepath.Base(path))
		}
		if err := verifyFile(directory, ed25519.PublicKey(key), release.signedFile); err != nil {
			return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		for _, delta := range release.Deltas {
			if delta.From == "" {
				return 0, fmt.Errorf("%s: delta has no source version", filepath.Base(path))
			}
			if err := verifyFile(directory, ed25519.PublicKey(key), delta.signedFile); err != nil {
				return 0, fmt.Errorf("%s: delta: %w", filepath.Base(path), err)
			}
		}
	}
	return len(paths), nil
}

func verifyFile(directory string, key ed25519.PublicKey, entry signedFile) error {
	location, err := url.Parse(entry.URL)
	if err != nil || location.Scheme != "https" || location.Host == "" || location.User != nil ||
		location.RawQuery != "" || location.Fragment != "" || entry.Size <= 0 || entry.Size > 1<<30 {
		return errors.New("invalid signed artifact URL or size")
	}
	name := filepath.Base(location.Path)
	if name == "." || name == "" || strings.ContainsAny(name, `\/`) {
		return errors.New("invalid signed artifact name")
	}
	path := filepath.Join(directory, name)
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("missing signed artifact %s", name)
	}
	if !info.Mode().IsRegular() || info.Size() != entry.Size {
		return fmt.Errorf("signed artifact size/type mismatch: %s", name)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	digest := sha256.New()
	_, readErr := io.Copy(digest, file)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(entry.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(key, digest.Sum(nil), signature) {
		return fmt.Errorf("artifact is not signed with the configured MyGo key: %s", name)
	}
	return nil
}
