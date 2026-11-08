package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	mountCacheResponseLimit = 1 << 20
	mountCacheQueueLimit    = 200
)

// VFSDiskCache contains the documented vfs/stats diskCache fields. Cache
// storage is evictable and must not be presented as a persistent offline copy.
type VFSDiskCache struct {
	BytesUsed         int64 `json:"bytesUsed"`
	ErroredFiles      int64 `json:"erroredFiles"`
	Files             int64 `json:"files"`
	OutOfSpace        bool  `json:"outOfSpace"`
	UploadsInProgress int64 `json:"uploadsInProgress"`
	UploadsQueued     int64 `json:"uploadsQueued"`
}

type VFSMetadataCache struct {
	Directories int64 `json:"dirs"`
	Files       int64 `json:"files"`
}

// VFSUpload uses the names and units of rclone's official vfs/queue output.
// ExpirySeconds may be negative; it is eligibility time, not upload ETA.
type VFSUpload struct {
	Name          string  `json:"name"`
	ID            uint64  `json:"id"`
	Size          int64   `json:"size"`
	ExpirySeconds float64 `json:"expiry"`
	Tries         int64   `json:"tries"`
	DelaySeconds  float64 `json:"delay"`
	Uploading     bool    `json:"uploading"`
}

type MountCacheView struct {
	MountID        string           `json:"mount_id"`
	Available      bool             `json:"available"`
	Status         string           `json:"status"`
	SampledAt      time.Time        `json:"sampled_at"`
	DiskCache      *VFSDiskCache    `json:"disk_cache,omitempty"`
	MetadataCache  VFSMetadataCache `json:"metadata_cache"`
	ReferenceCount int64            `json:"vfs_reference_count"`
	Queue          []VFSUpload      `json:"queue"`
	QueueTruncated bool             `json:"queue_truncated"`
	QueueError     string           `json:"queue_error,omitempty"`
	Error          string           `json:"error,omitempty"`
}

// MountCache only contacts an RC endpoint discovered on a mount process this
// instance owns. It never forwards a caller-supplied port, path or RC command.
func (a *App) MountCache(ctx context.Context, id string) (MountCacheView, error) {
	if err := ctx.Err(); err != nil {
		return MountCacheView{}, err
	}
	a.mountMu.Lock()
	defer a.mountMu.Unlock()
	state := a.Snapshot()
	view := MountCacheView{MountID: id, Status: "unavailable", SampledAt: time.Now().UTC(), Queue: []VFSUpload{}}
	var config *MountConfig
	for i := range state.Mounts {
		if state.Mounts[i].ID == id {
			config = &state.Mounts[i]
			break
		}
	}
	if config == nil {
		return view, errors.New("mount not found")
	}
	pid := state.OwnedMounts[id]
	if pid <= 0 {
		view.Status, view.Error = "not_owned", "cache state is available only for this instance's owned mounts"
		return view, nil
	}
	identity := state.MountIdentities[id]
	if identity == "" || identity != processIdentity(pid) {
		view.Status, view.Error = "identity_changed", "the recorded mount process ended or its identity changed"
		return view, nil
	}
	port, err := mountProcessRCPort(ctx, *config, pid)
	if err != nil {
		view.Error = err.Error()
		return view, nil
	}
	if identity != processIdentity(pid) {
		view.Status, view.Error = "identity_changed", "the recorded mount process ended or its identity changed"
		return view, nil
	}
	return sampleMountCache(ctx, a.EngineClient(), port, *config), nil
}

func sampleMountCache(ctx context.Context, client RcloneClient, port int, config MountConfig) MountCacheView {
	view := MountCacheView{MountID: config.ID, Status: "unavailable", SampledAt: time.Now().UTC(), Queue: []VFSUpload{}}
	var stats struct {
		DiskCache     *VFSDiskCache     `json:"diskCache"`
		MetadataCache *VFSMetadataCache `json:"metadataCache"`
		InUse         *int64            `json:"inUse"`
	}
	if err := readMountRC(ctx, client, port, "vfs/stats", config.Remote, &stats); err != nil {
		view.Error = err.Error()
		return view
	}
	if stats.InUse == nil || stats.MetadataCache == nil || *stats.InUse < 0 || stats.MetadataCache.Directories < 0 || stats.MetadataCache.Files < 0 {
		view.Error = "VFS returned invalid cache counters"
		return view
	}
	if disk := stats.DiskCache; disk != nil {
		if disk.BytesUsed < 0 || disk.Files < 0 || disk.ErroredFiles < 0 || disk.UploadsInProgress < 0 || disk.UploadsQueued < 0 {
			view.Error = "VFS returned invalid disk cache counters"
			return view
		}
	}
	view.Available, view.Status = true, "ready"
	view.DiskCache, view.MetadataCache, view.ReferenceCount = stats.DiskCache, *stats.MetadataCache, *stats.InUse
	if stats.DiskCache == nil {
		view.Status = "cache_disabled"
		return view
	}
	var queue struct {
		Queue *[]VFSUpload `json:"queue"`
	}
	if err := readMountRC(ctx, client, port, "vfs/queue", config.Remote, &queue); err != nil {
		view.QueueError = err.Error()
		view.QueueTruncated = stats.DiskCache.UploadsInProgress+stats.DiskCache.UploadsQueued > mountCacheQueueLimit
		return view
	}
	if queue.Queue == nil {
		view.QueueError = "VFS upload queue was not present in the response"
		view.QueueTruncated = stats.DiskCache.UploadsInProgress+stats.DiskCache.UploadsQueued > 0
		return view
	}
	items := *queue.Queue
	if len(items) > mountCacheQueueLimit {
		items = items[:mountCacheQueueLimit]
		view.QueueTruncated = true
	}
	view.Queue = append(view.Queue, items...)
	return view
}

func readMountRC(ctx context.Context, client RcloneClient, port int, method, fs string, output any) error {
	if port < 1 || port > 65535 || (method != "vfs/stats" && method != "vfs/queue") {
		return errors.New("unsupported VFS status request")
	}
	body, err := json.Marshal(map[string]string{"fs": fs})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://127.0.0.1:"+strconv.Itoa(port)+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if client.RCToken != "" {
		req.SetBasicAuth("rclonego", client.RCToken)
	}
	transport := &http.Transport{
		Proxy:             nil,
		DialContext:       (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("VFS redirects are not allowed")
		},
	}
	response, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("owned mount's VFS status endpoint is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("VFS status request returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, mountCacheResponseLimit+1))
	if err != nil {
		return errors.New("read VFS status response failed")
	}
	if len(raw) > mountCacheResponseLimit {
		return errors.New("VFS status response exceeds the size limit")
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' {
		return errors.New("VFS returned an invalid status response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return errors.New("VFS returned an invalid status response")
	}
	return nil
}

// OfflineCopyRequest creates a persistent local copy. Destination is a local
// directory; when File is true, Source's basename is copied inside it.
type OfflineCopyRequest struct {
	Name           string            `json:"name"`
	Source         string            `json:"source"`
	Destination    string            `json:"destination"`
	File           bool              `json:"file"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Settings       *TransferSettings `json:"settings,omitempty"`
}

func (a *App) AddOfflineCopy(request OfflineCopyRequest) (Job, error) {
	source, _, kind, err := browsePath(strings.TrimSpace(request.Source))
	if err != nil || kind != "remote" {
		return Job{}, errors.New("offline copy requires an explicit configured remote:path source")
	}
	destination, err := offlineCopyDestination(request.Destination)
	if err != nil {
		return Job{}, err
	}
	state := a.Snapshot()
	for _, mount := range state.Mounts {
		if mount.CacheDir == "" {
			continue
		}
		cache, err := offlineCopyDestination(mount.CacheDir)
		if err == nil && withinOfflinePath(cache, destination) {
			return Job{}, errors.New("choose an offline directory outside rclone's VFS cache")
		}
	}
	operation := "copy"
	if request.File {
		_, suffix, _ := strings.Cut(source, ":")
		base := path.Base(suffix)
		if base == "." || base == "/" || base == "" || strings.ContainsAny(base, `/\`) {
			return Job{}, errors.New("offline file source must name a file")
		}
		if runtime.GOOS == "windows" && (strings.ContainsAny(base, `<>:"|?*`) || strings.HasSuffix(base, ".") || strings.HasSuffix(base, " ")) {
			return Job{}, errors.New("source filename cannot be represented as a local Windows filename")
		}
		operation = "copyto"
		destination = filepath.Join(destination, base)
	}
	settings := state.Defaults
	if request.Settings != nil {
		settings = *request.Settings
		if err := settings.Validate(); err != nil {
			return Job{}, err
		}
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "Offline copy"
	}
	if request.IdempotencyKey != "" {
		previous, err := a.store.JobByKey(request.IdempotencyKey)
		if err != nil {
			return Job{}, err
		}
		if previous.ID != "" && (previous.Source != source || previous.Destination != destination || previous.Operation != operation) {
			return Job{}, errors.New("idempotency key belongs to a different offline copy")
		}
	}
	return a.AddJob(JobCreateRequest{
		Name: name, Source: source, Destination: destination, Operation: operation,
		IdempotencyKey: request.IdempotencyKey, Transfers: settings.Transfers, Checkers: settings.Checkers,
		ChunkSize: settings.ChunkSize, BufferSize: settings.BufferSize, CPUParallelism: &settings.CPUParallelism,
		Bandwidth: settings.Bandwidth,
	})
}

// Resolve existing ancestors so a symlink or Windows junction cannot redirect
// a requested offline folder into a mounted VFS cache. No directory is created.
func offlineCopyDestination(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !filepath.IsAbs(value) || validateSafeText(value) != nil {
		return "", errors.New("offline destination must be an absolute local directory")
	}
	value = filepath.Clean(value)
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		return "", errors.New("offline destination must be a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return "", errors.New("offline destination is not accessible")
	}
	ancestor := value
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(ancestor)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", errors.New("offline destination cannot be resolved")
		}
		if info, linkErr := os.Lstat(ancestor); linkErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("offline destination contains an unresolved symbolic link")
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("offline destination has no accessible parent")
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
}

func withinOfflinePath(root, destination string) bool {
	if runtime.GOOS == "windows" {
		root, destination = strings.ToLower(root), strings.ToLower(destination)
	}
	relative, err := filepath.Rel(root, destination)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))))
}
