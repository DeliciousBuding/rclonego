package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Preview struct {
	ID               string         `json:"id"`
	ExpiresAt        time.Time      `json:"expires_at"`
	Operation        string         `json:"operation"`
	Source           string         `json:"source"`
	Destination      string         `json:"destination"`
	Entries          []PreviewEntry `json:"entries"`
	Summary          PreviewSummary `json:"summary"`
	EntriesTruncated bool           `json:"entries_truncated"`
	ReportIncomplete bool           `json:"report_incomplete"`
	Scope            string         `json:"scope"`
	Log              string         `json:"log"`
	Truncated        bool           `json:"truncated"`
	Fingerprint      string         `json:"-"`
}

type PreviewEntry struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

type PreviewSummary struct {
	Added           int64 `json:"added"`
	Changed         int64 `json:"changed"`
	Deleted         int64 `json:"deleted"`
	Unchanged       int64 `json:"unchanged"`
	DestinationOnly int64 `json:"destination_only"`
	Errors          int64 `json:"errors"`
}

func jobFingerprint(job Job) string {
	job.ID, job.Name, job.IdempotencyKey = "", "", ""
	job.Status, job.Position, job.CreatedAt, job.LastUpdatedAt = "", 0, time.Time{}, time.Time{}
	job.DryRun = false
	raw, _ := json.Marshal(job)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func (a *App) PreviewJob(ctx context.Context, request JobCreateRequest) (Preview, error) {
	// Preview cannot mutate either side. Explicit delete permission is needed
	// only to construct sync's arguments; the process always has --dry-run.
	request.AllowDelete = request.Operation == "sync" || request.AllowDelete
	job, err := a.queue.buildJob(request)
	if err != nil {
		return Preview{}, err
	}
	if job.Operation != "copy" && job.Operation != "copyto" && job.Operation != "sync" {
		return Preview{}, errors.New("preview supports copy, copyto and sync")
	}
	directory, err := os.MkdirTemp(a.dataDir, ".preview-*")
	if err != nil {
		return Preview{}, err
	}
	inputPath := filepath.Join(directory, "input")
	defer func() {
		_ = os.Remove(inputPath + ".files")
		_ = os.Remove(directory)
	}()
	if len(job.Files) > 0 {
		if err := os.WriteFile(inputPath+".files", []byte(strings.Join(job.Files, "\n")+"\n"), 0o600); err != nil {
			return Preview{}, err
		}
	}
	job.DryRun = true
	port, err := freePort()
	if err != nil {
		return Preview{}, err
	}
	args, err := rcloneTransferArgs(job, port, inputPath)
	if err != nil {
		return Preview{}, err
	}
	// Combined is rclone's machine-oriented comparison report. Keep it
	// separate from JSON logs; log messages are never interpreted as changes.
	args = append(args, "--combined", "-")
	client := a.EngineClient()
	cmd := exec.CommandContext(ctx, client.Path, args...)
	cmd.Env = client.environment()
	if job.CPUParallelism > 0 {
		cmd.Env = append(cmd.Env, "GOMAXPROCS="+strconv.Itoa(job.CPUParallelism))
	}
	hideProcess(cmd)
	report := &previewReportOutput{operation: job.Operation}
	log := &boundedPreviewOutput{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = report, log
	err = cmd.Run()
	report.finish()
	if err != nil {
		if ctx.Err() != nil {
			return Preview{}, ctx.Err()
		}
		return Preview{}, errors.New("preview failed; check paths, authorization and deletion limit")
	}
	id, err := RandomID()
	if err != nil {
		return Preview{}, err
	}
	result := Preview{
		ID: id, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Operation: job.Operation,
		Source: job.Source, Destination: job.Destination,
		Entries: append([]PreviewEntry{}, report.entries...), Summary: report.summary,
		EntriesTruncated: report.truncated, ReportIncomplete: report.incomplete,
		Scope: "rclone comparison records at preview time; changes are predicted, directory creation is not listed",
		Log:   string(log.data), Truncated: log.truncated, Fingerprint: jobFingerprint(job),
	}
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	if a.previews == nil {
		a.previews = map[string]Preview{}
	}
	for key, preview := range a.previews {
		if time.Now().After(preview.ExpiresAt) {
			delete(a.previews, key)
		}
	}
	if len(a.previews) >= 64 {
		return Preview{}, errors.New("too many outstanding previews")
	}
	a.previews[id] = result
	return result, nil
}

func (a *App) confirmPreview(request JobCreateRequest) error {
	if request.Operation != "sync" || request.DryRun {
		return nil
	}
	job, err := a.queue.buildJob(request)
	if err != nil {
		return err
	}
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	preview, exists := a.previews[request.PreviewID]
	if !exists || time.Now().After(preview.ExpiresAt) || preview.Fingerprint != jobFingerprint(job) {
		return errors.New("sync requires an unexpired matching dry-run preview_id and explicit allow_delete")
	}
	if preview.ReportIncomplete || preview.Summary.Errors > 0 {
		return errors.New("sync preview has incomplete comparison evidence; fix the paths or authorization and preview again")
	}
	return nil
}

// boundedPreviewOutput keeps a tail of process output while still accepting
// every write, so a noisy engine cannot grow a preview's memory without bound.
type boundedPreviewOutput struct {
	data      []byte
	limit     int
	truncated bool
}

func (w *boundedPreviewOutput) Write(p []byte) (int, error) {
	n := len(p)
	if w.limit <= 0 {
		w.truncated = w.truncated || n > 0
		return n, nil
	}
	if len(w.data)+n > w.limit {
		w.truncated = true
	}
	if n >= w.limit {
		w.data = append(w.data[:0], p[n-w.limit:]...)
	} else {
		if len(w.data)+n > w.limit {
			keep := w.limit - n
			copy(w.data, w.data[len(w.data)-keep:])
			w.data = w.data[:keep]
		}
		w.data = append(w.data, p...)
	}
	return n, nil
}

type previewReportOutput struct {
	operation  string
	pending    []byte
	discarding bool
	entries    []PreviewEntry
	entryBytes int
	summary    PreviewSummary
	truncated  bool
	incomplete bool
}

func (w *previewReportOutput) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		index := bytes.IndexByte(p, '\n')
		end := len(p)
		if index >= 0 {
			end = index
		}
		if !w.discarding {
			if len(w.pending)+end > 32<<10 {
				w.discarding, w.incomplete = true, true
				w.pending = nil
			} else {
				w.pending = append(w.pending, p[:end]...)
			}
		}
		if index < 0 {
			break
		}
		if !w.discarding {
			w.line(strings.TrimSuffix(string(w.pending), "\r"))
		}
		w.pending, w.discarding = w.pending[:0], false
		p = p[index+1:]
	}
	return n, nil
}

func (w *previewReportOutput) finish() {
	if !w.discarding && len(w.pending) > 0 {
		w.line(strings.TrimSuffix(string(w.pending), "\r"))
	}
	w.pending = nil
}

func (w *previewReportOutput) line(line string) {
	if line == "" {
		return
	}
	if len(line) < 3 || line[1] != ' ' || validateSafeText(line[2:]) != nil {
		w.incomplete = true
		return
	}
	action := ""
	switch line[0] {
	case '+':
		w.summary.Added++
		action = "added"
	case '*':
		w.summary.Changed++
		action = "changed"
	case '-':
		if w.operation == "sync" {
			w.summary.Deleted++
			action = "deleted"
		} else {
			w.summary.DestinationOnly++
			action = "destination_only"
		}
	case '=':
		w.summary.Unchanged++
		return
	case '!':
		w.summary.Errors++
		action = "error"
	default:
		w.incomplete = true
		return
	}
	if len(w.entries) >= 500 || w.entryBytes+len(line) > 128<<10 {
		w.truncated = true
		return
	}
	w.entries = append(w.entries, PreviewEntry{Path: line[2:], Action: action})
	w.entryBytes += len(line)
}
