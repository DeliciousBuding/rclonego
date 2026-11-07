// Package session manages private local instance discovery.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	API string `json:"api"`
	PID int    `json:"pid"`
}

func DataDir() (string, error) {
	base, err := os.UserConfigDir()
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		base, err = local, nil
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "RcloneGo"), nil
}

func Token(dataDir, name string) (string, error) {
	path := filepath.Join(dataDir, name)
	if raw, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(raw))
		if len(token) != 64 {
			return "", errors.New("invalid local authentication token")
		}
		return token, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.WriteString(token + "\n")
	closeErr := file.Close()
	return token, errors.Join(writeErr, closeErr)
}

func Publish(dataDir, address string) error {
	raw, err := json.Marshal(Info{API: address, PID: os.Getpid()})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "instance.json"), raw, 0o600)
}

func Discover(dataDir string) (Info, string, error) {
	var result Info
	raw, err := os.ReadFile(filepath.Join(dataDir, "instance.json"))
	if err != nil {
		return result, "", err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, "", err
	}
	raw, err = os.ReadFile(filepath.Join(dataDir, "api-token"))
	return result, strings.TrimSpace(string(raw)), err
}
