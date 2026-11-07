package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"
)

type Preview struct {
	ID          string    `json:"id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Log         string    `json:"log"`
	Truncated   bool      `json:"truncated"`
	Fingerprint string    `json:"-"`
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
	file, err := os.CreateTemp(a.dataDir, ".preview-*")
	if err != nil {
		return Preview{}, err
	}
	path := file.Name()
	file.Close()
	defer os.Remove(path)
	job.DryRun = true
	port, err := freePort()
	if err != nil {
		return Preview{}, err
	}
	args, err := rcloneTransferArgs(job, port, path)
	if err != nil {
		return Preview{}, err
	}
	client := a.EngineClient()
	cmd := exec.CommandContext(ctx, client.Path, args...)
	cmd.Env = client.environment()
	hideProcess(cmd)
	log, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Preview{}, err
	}
	cmd.Stdout, cmd.Stderr = log, log
	err = cmd.Run()
	log.Close()
	if err != nil {
		return Preview{}, errors.New("preview failed; check paths, authorization and deletion limit")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Preview{}, err
	}
	text, err := tailFile(path, 64<<10)
	if err != nil {
		return Preview{}, err
	}
	id, err := RandomID()
	if err != nil {
		return Preview{}, err
	}
	result := Preview{ID: id, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Log: text,
		Truncated: info.Size() > 64<<10, Fingerprint: jobFingerprint(job)}
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
	return nil
}
