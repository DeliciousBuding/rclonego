// Package automation supplies bounded change observation. Transfers still run
// through the application's existing queue.
package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

const (
	MaxEntries     = 100000
	MaxDirectories = 4096
)

// Snapshot is a metadata baseline, matching rclone's ordinary size/time copy
// comparison. Symbolic links are not followed.
type Snapshot struct {
	Fingerprint string
	Directories []string
	Entries     int
}

func Scan(ctx context.Context, root string) (Snapshot, error) {
	result := Snapshot{}
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path != root && os.IsNotExist(err) {
				return nil // A rename during scanning is reconciled next time.
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		result.Entries++
		if result.Entries > MaxEntries {
			return fmt.Errorf("automatic folder scan exceeds %d entries", MaxEntries)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result.Directories = append(result.Directories, path)
			if len(result.Directories) > MaxDirectories {
				return fmt.Errorf("automatic folder scan exceeds %d directories", MaxDirectories)
			}
			fmt.Fprintf(hash, "d:%q\n", filepath.ToSlash(relative))
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			fmt.Fprintf(hash, "f:%q:%d:%d:%d\n", filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano(), info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	result.Fingerprint = hex.EncodeToString(hash.Sum(nil))
	return result, nil
}

// Monitor observes each directory because fsnotify does not recurse by itself.
// A periodic Scan remains necessary when a platform drops notifications.
type Monitor struct {
	watcher *fsnotify.Watcher
	done    chan struct{}
	dirty   atomic.Bool
	mu      sync.Mutex
	paths   map[string]bool
	warning error
}

func NewMonitor() (*Monitor, error) {
	watcher, err := fsnotify.NewBufferedWatcher(256)
	if err != nil {
		return nil, err
	}
	monitor := &Monitor{watcher: watcher, done: make(chan struct{}), paths: map[string]bool{}}
	go monitor.observe()
	return monitor, nil
}

func (m *Monitor) observe() {
	defer close(m.done)
	for {
		select {
		case _, open := <-m.watcher.Events:
			if !open {
				return
			}
			m.dirty.Store(true)
		case err, open := <-m.watcher.Errors:
			if !open {
				return
			}
			m.dirty.Store(true)
			m.mu.Lock()
			m.warning = err
			m.mu.Unlock()
		}
	}
}

func (m *Monitor) Changed() bool { return m.dirty.Swap(false) }

func (m *Monitor) SyncDirectories(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(paths) > MaxDirectories {
		return errors.New("too many watched directories")
	}
	wanted := make(map[string]bool, len(paths))
	for _, path := range paths {
		wanted[path] = true
	}
	for path := range m.paths {
		if !wanted[path] {
			_ = m.watcher.Remove(path) // It may already have been removed by the OS.
			delete(m.paths, path)
		}
	}
	var firstError error
	for path := range wanted {
		if m.paths[path] {
			continue
		}
		if err := m.watcher.Add(path); err != nil {
			if firstError == nil {
				firstError = err
			}
			continue
		}
		m.paths[path] = true
	}
	m.warning = firstError
	return firstError
}

func (m *Monitor) Warning() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.warning
}

func (m *Monitor) Close() {
	_ = m.watcher.Close()
	<-m.done
}
