package core

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const taskLogLimit = int64(8 << 20)

type LogRetentionPolicy struct {
	MaxBytes int64         `json:"max_bytes"`
	MaxFiles int           `json:"max_files"`
	MaxAge   time.Duration `json:"-"`
}

var DefaultLogRetention = LogRetentionPolicy{MaxBytes: 256 << 20, MaxFiles: 200, MaxAge: 30 * 24 * time.Hour}

// taskLog bounds live output too, so a noisy process cannot fill the data disk.
// Completed logs are separately aged out; reports and attempts remain in SQLite.
type taskLog struct {
	mu   sync.Mutex
	file *os.File
	size int64
}

func openTaskLog(path string) (*taskLog, error) {
	// Windows append-only handles cannot truncate. This writer owns the file
	// position and serializes every write, so use a writable handle and seek.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		file.Close()
		return nil, err
	}
	return &taskLog{file: file, size: info.Size()}, nil
}

func (log *taskLog) Write(p []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	original := len(p)
	if int64(len(p)) > taskLogLimit/2 {
		p = p[len(p)-int(taskLogLimit/2):]
	}
	if log.size+int64(len(p)) > taskLogLimit {
		if err := log.file.Truncate(0); err != nil {
			return 0, err
		}
		if _, err := log.file.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		n, err := log.file.WriteString("[Earlier output omitted after the task log size limit]\n")
		if err != nil {
			return 0, err
		}
		log.size = int64(n)
	}
	n, err := log.file.Write(p)
	log.size += int64(n)
	if err != nil {
		return 0, err
	}
	return original, nil
}

func (log *taskLog) Close() error {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.file.Close()
}

func retainedJob(status string) bool {
	return status == "handoff" || status == "pending" || status == "running" || status == "paused" || status == "interrupted"
}

func (q *Queue) CleanupLogs() error {
	protected := map[string]bool{}
	for _, job := range q.store.Snapshot().Jobs {
		if retainedJob(job.Status) && job.LogPath != "" {
			if path, err := filepath.Abs(job.LogPath); err == nil {
				protected[path] = true
				protected[path+".files"] = true
			}
		}
	}
	q.mu.Lock()
	if q.unsavedFinal != nil && q.unsavedFinal.LogPath != "" {
		if path, err := filepath.Abs(q.unsavedFinal.LogPath); err == nil {
			protected[path] = true
			protected[path+".files"] = true
		}
	}
	q.mu.Unlock()
	return pruneTaskLogs(filepath.Join(q.dataDir, "logs"), protected, DefaultLogRetention, time.Now())
}

func (q *Queue) cleanupLogsIfDue(force bool) {
	q.mu.Lock()
	due := force || q.cleanupAt.IsZero() || time.Since(q.cleanupAt) >= time.Hour
	if due {
		q.cleanupAt = time.Now()
	}
	q.mu.Unlock()
	if !due {
		return
	}
	err := q.CleanupLogs()
	q.mu.Lock()
	q.cleanupError = err
	q.mu.Unlock()
}

func pruneTaskLogs(directory string, protected map[string]bool, policy LogRetentionPolicy, now time.Time) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("task log directory must be a real directory")
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	type entry struct {
		path string
		info os.FileInfo
	}
	var files []entry
	var bytes int64
	for _, item := range entries {
		if item.Type()&os.ModeSymlink != 0 || item.IsDir() || (!strings.HasSuffix(item.Name(), ".log") && !strings.HasSuffix(item.Name(), ".log.files")) {
			continue
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		files = append(files, entry{path: filepath.Join(directory, item.Name()), info: info})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].info.ModTime().Equal(files[j].info.ModTime()) {
			return files[i].path < files[j].path
		}
		return files[i].info.ModTime().Before(files[j].info.ModTime())
	})
	count := len(files)
	var failures []error
	for _, item := range files {
		// All candidates come directly from ReadDir; never follow links or
		// remove anything outside this instance's dedicated log directory.
		if protected[item.path] {
			continue
		}
		aged := policy.MaxAge > 0 && now.Sub(item.info.ModTime()) > policy.MaxAge
		overSize := policy.MaxBytes > 0 && bytes > policy.MaxBytes
		overCount := policy.MaxFiles > 0 && count > policy.MaxFiles
		if !aged && !overSize && !overCount {
			continue
		}
		if err := os.Remove(item.path); err != nil {
			failures = append(failures, err)
			continue
		}
		bytes -= item.info.Size()
		count--
	}
	return errors.Join(failures...)
}
