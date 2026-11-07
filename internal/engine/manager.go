// Package engine installs verified upstream releases without changing PATH.
package engine

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const Recommended = "v1.76.0"

var hashes = map[string]string{
	"windows/amd64": "8b4fd2737ade4e70c057ef4a49deb7cffe94b0fb9d7587c189d149823f21a1dc",
	"windows/arm64": "c406c8a69a5a408e50624aa990fbb0a6266b3a4e83782a94de3cb96f50f86f10",
	"linux/amd64":   "b00f00b3266c3764413afe599d3ec316f6f43b3e0ea8a0bc1d186d2133e46df7",
	"linux/arm64":   "ee169016675e23a69d77240565133ff7dbc9b33d16fa8f7053d4af39f1eaeb78",
	"darwin/amd64":  "4f5835733d52b818340f97b1fc0348a70461534d123363a183ca2b660219595a",
	"darwin/arm64":  "e72dc90b70f4947a7b72b61bde4a4fbba5777b2874fb448951ffdde67ebd0ab7",
}

type Installation struct {
	Version  string `json:"version"`
	Path     string `json:"path"`
	Source   string `json:"source"`
	SHA256   string `json:"sha256,omitempty"`
	Platform string `json:"platform"`
}

type Manager struct {
	Dir string
	mu  sync.Mutex
}

func (m *Manager) Selected() (Installation, error) {
	var selected Installation
	raw, err := os.ReadFile(filepath.Join(m.Dir, "selected.json"))
	if err == nil {
		err = json.Unmarshal(raw, &selected)
	}
	return selected, err
}

func (m *Manager) Select(installation Installation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(installation)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(m.Dir, ".selected-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0o600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, filepath.Join(m.Dir, "selected.json"))
}

// Install accepts only the pinned official archive, including in offline mode.
// The prior selection is untouched until the caller verifies and selects it.
func (m *Manager) Install(ctx context.Context, archive string) (Installation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	platform := runtime.GOOS + "/" + runtime.GOARCH
	expected, ok := hashes[platform]
	if !ok {
		return Installation{}, errors.New("no verified release for this platform")
	}
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return Installation{}, err
	}
	targetOS := runtime.GOOS
	if targetOS == "darwin" {
		targetOS = "osx"
	}
	asset := fmt.Sprintf("rclone-%s-%s-%s.zip", Recommended, targetOS, runtime.GOARCH)
	source := "https://github.com/rclone/rclone/releases/download/" + Recommended + "/" + asset
	if archive == "" {
		file, err := os.CreateTemp(m.Dir, ".download-*")
		if err != nil {
			return Installation{}, err
		}
		archive = file.Name()
		defer os.Remove(archive)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			file.Close()
			return Installation{}, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			file.Close()
			return Installation{}, errors.New("engine download failed; retry or import an official archive")
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			file.Close()
			return Installation{}, fmt.Errorf("engine download returned HTTP %d", resp.StatusCode)
		}
		_, err = io.Copy(file, io.LimitReader(resp.Body, 128<<20))
		resp.Body.Close()
		closeErr := file.Close()
		if err != nil {
			return Installation{}, err
		}
		if closeErr != nil {
			return Installation{}, closeErr
		}
	}
	file, err := os.Open(archive)
	if err != nil {
		return Installation{}, err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	file.Close()
	if err != nil {
		return Installation{}, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return Installation{}, errors.New("archive SHA-256 does not match the pinned official release")
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return Installation{}, err
	}
	defer reader.Close()
	binary := "rclone"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	destination := filepath.Join(m.Dir, Recommended+"-"+targetOS+"-"+runtime.GOARCH)
	path := filepath.Join(destination, binary)
	if _, err := os.Stat(path); err == nil {
		return Installation{Recommended, path, source, expected, platform}, nil
	}
	stage, err := os.MkdirTemp(m.Dir, ".install-*")
	if err != nil {
		return Installation{}, err
	}
	defer os.RemoveAll(stage)
	found := false
	for _, entry := range reader.File {
		if ctx.Err() != nil {
			return Installation{}, ctx.Err()
		}
		clean := filepath.Clean(filepath.FromSlash(entry.Name))
		if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) || entry.Mode()&os.ModeSymlink != 0 {
			return Installation{}, errors.New("unsafe archive member")
		}
		name := filepath.Base(clean)
		if name != binary && name != "COPYING" && name != "README.html" {
			continue
		}
		if entry.UncompressedSize64 > 128<<20 {
			return Installation{}, errors.New("archive member exceeds size limit")
		}
		src, err := entry.Open()
		if err != nil {
			return Installation{}, err
		}
		mode := os.FileMode(0o600)
		if name == binary {
			if found {
				src.Close()
				return Installation{}, errors.New("duplicate engine in archive")
			}
			mode, found = 0o700, true
		}
		out, err := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			src.Close()
			return Installation{}, err
		}
		_, err = io.Copy(out, src)
		src.Close()
		closeErr := out.Close()
		if err != nil {
			return Installation{}, err
		}
		if closeErr != nil {
			return Installation{}, closeErr
		}
	}
	if !found {
		return Installation{}, errors.New("archive contains no engine")
	}
	if err := os.Rename(stage, destination); err != nil {
		return Installation{}, err
	}
	return Installation{Recommended, path, source, expected, platform}, nil
}
