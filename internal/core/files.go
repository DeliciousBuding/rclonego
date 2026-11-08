package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

const filePreviewLimit = 8 << 20

type FilePreview struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	Text      string `json:"text,omitempty"`
	Data      []byte `json:"data,omitempty"`
	Truncated bool   `json:"truncated"`
}

type FileActionRequest struct {
	Operation     string `json:"operation"`
	Path          string `json:"path"`
	Name          string `json:"name,omitempty"`
	IsDir         bool   `json:"is_dir"`
	ConfirmDelete bool   `json:"confirm_delete"`
}

type FileVersion struct {
	Size    int64      `json:"size"`
	IsDir   bool       `json:"is_dir"`
	ModTime *time.Time `json:"mod_time,omitempty"`
}

// File changes use the same durable queue and attempt history as transfers.
// Delete removes one file or one empty directory; it never expands to purge.
func (a *App) AddFileAction(request FileActionRequest) (Job, error) {
	source, parent, _, err := browsePath(request.Path)
	if err != nil {
		return Job{}, err
	}
	destination, operation := source, request.Operation
	switch operation {
	case "mkdir":
		destination, err = ChildPath(source, request.Name)
		source = destination
	case "rename":
		if parent == "" {
			return Job{}, errors.New("a filesystem root cannot be renamed")
		}
		destination, err = ChildPath(parent, request.Name)
		operation = "moveto"
	case "delete":
		if !request.ConfirmDelete || parent == "" {
			return Job{}, errors.New("deleting one file or empty folder requires explicit confirmation")
		}
		operation = "deletefile"
		if request.IsDir {
			operation = "rmdir"
		}
	default:
		return Job{}, errors.New("file action must be mkdir, rename or delete")
	}
	if err != nil {
		return Job{}, err
	}
	return a.queue.Add(JobCreateRequest{Name: request.Operation + " · " + FileBaseName(source),
		Operation: operation, Source: source, Destination: destination, AllowDelete: request.ConfirmDelete})
}

func ChildPath(parent, name string) (string, error) {
	if name == "" || strings.TrimSpace(name) != name || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\:`) || strings.ContainsAny(name, "\x00\r\n\t") {
		return "", errors.New("enter one file or folder name without path separators")
	}
	root, _, kind, err := browsePath(parent)
	if err != nil {
		return "", err
	}
	if kind == "local" {
		return filepath.Join(root, name), nil
	}
	separator := "/"
	if strings.HasSuffix(root, ":") || strings.HasSuffix(root, "/") {
		separator = ""
	}
	return root + separator + name, nil
}

func FileBaseName(value string) string {
	if filepath.IsAbs(value) {
		return filepath.Base(value)
	}
	_, suffix, _ := strings.Cut(value, ":")
	return path.Base(suffix)
}

func fileOperation(operation string) bool {
	return operation == "mkdir" || operation == "moveto" || operation == "deletefile" || operation == "rmdir"
}

func fileOperationArgs(job Job, rcPort int) ([]string, error) {
	source, parent, _, err := browsePath(job.Source)
	if err != nil {
		return nil, err
	}
	if job.Operation != "mkdir" && parent == "" {
		return nil, errors.New("file actions cannot modify a filesystem root")
	}
	if len(job.Files)+len(job.Includes)+len(job.Excludes) > 0 {
		return nil, errors.New("file actions do not accept recursive filters")
	}
	args := []string{job.Operation, source}
	switch job.Operation {
	case "moveto":
		dest, destParent, _, err := browsePath(job.Destination)
		if err != nil || destParent != parent || dest == source {
			return nil, errors.New("rename must use a different name in the same directory")
		}
		args = append(args, dest, "--ignore-existing")
	case "deletefile", "rmdir":
		if !job.AllowDelete {
			return nil, errors.New("file deletion requires explicit allow_delete")
		}
	}
	if job.DryRun {
		args = append(args, "--dry-run")
	}
	return append(args, "--retries", "1", "--log-level", "INFO", "--use-json-log",
		"--rc", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", rcPort)), nil
}

// Recheck delayed changes immediately before execution. Existing rename
// destinations are rejected; --ignore-existing also prevents overwriting a
// file created after this check.
func validateFileExecution(ctx context.Context, client RcloneClient, job Job) error {
	if !fileOperation(job.Operation) {
		return nil
	}
	if job.Operation == "moveto" || job.Operation == "mkdir" {
		parent := job.Source
		target := job.Source
		if job.Operation == "moveto" {
			target = job.Destination
		}
		_, parent, _, _ = browsePath(target)
		entries, truncated, err := fileDirectory(ctx, client, parent)
		if err != nil {
			return err
		}
		if truncated {
			return errors.New("directory is too large to confirm a unique name")
		}
		for _, entry := range entries {
			if strings.EqualFold(entry.Name, FileBaseName(target)) {
				return errors.New("a file or folder with that name already exists")
			}
		}
	}
	if job.Operation == "mkdir" {
		return nil
	}
	info, err := fileStat(ctx, client, job.Source)
	if err != nil {
		return err
	}
	if version := job.FileVersion; version != nil {
		if info.IsDir != version.IsDir || (!info.IsDir && info.Size != version.Size) ||
			(!info.IsDir && version.ModTime != nil && info.ModTime != nil && !version.ModTime.Equal(*info.ModTime)) {
			return errors.New("selected file changed while waiting; refresh and submit the operation again")
		}
	}
	if job.Operation == "deletefile" && info.IsDir {
		return errors.New("selected file is now a folder; refresh before deleting")
	}
	if job.Operation == "rmdir" && !info.IsDir {
		return errors.New("selected folder is now a file; refresh before deleting")
	}
	return nil
}

func verifyFileExecution(ctx context.Context, client RcloneClient, job Job) error {
	if job.Operation != "moveto" || job.DryRun {
		return nil
	}
	_, parent, _, _ := browsePath(job.Source)
	entries, truncated, err := fileDirectory(ctx, client, parent)
	if err != nil {
		return err
	}
	if truncated {
		return errors.New("rename verification needs a smaller directory")
	}
	found := false
	for _, entry := range entries {
		if entry.Name == FileBaseName(job.Source) {
			return errors.New("rename did not remove the source; refresh the folder before retrying")
		}
		if entry.Name == FileBaseName(job.Destination) {
			found = true
		}
	}
	if !found {
		return errors.New("rename destination is unavailable; refresh the folder before retrying")
	}
	return nil
}

func fileDirectory(ctx context.Context, client RcloneClient, value string) ([]DirectoryEntry, bool, error) {
	root, _, kind, err := browsePath(value)
	if err != nil {
		return nil, false, err
	}
	if kind == "local" {
		return browseLocal(ctx, root, false)
	}
	return browseRemote(ctx, client, root, false)
}

func fileStat(ctx context.Context, client RcloneClient, value string) (DirectoryEntry, error) {
	root, _, kind, err := browsePath(value)
	if err != nil {
		return DirectoryEntry{}, err
	}
	if kind == "local" {
		info, err := os.Lstat(root)
		if err != nil {
			return DirectoryEntry{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return DirectoryEntry{}, errors.New("file operations require a regular file or folder")
		}
		modified := info.ModTime()
		return DirectoryEntry{Name: info.Name(), Path: root, Size: info.Size(), IsDir: info.IsDir(), ModTime: &modified}, nil
	}
	raw, err := boundedFileCommand(ctx, client, 64<<10, "lsjson", root, "--stat", "--no-mimetype")
	if err != nil {
		return DirectoryEntry{}, err
	}
	var item struct {
		Name    string
		Size    int64
		IsDir   bool
		ModTime *time.Time
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return DirectoryEntry{}, err
	}
	return DirectoryEntry{Name: item.Name, Path: root, Size: item.Size, IsDir: item.IsDir, ModTime: item.ModTime}, nil
}

func boundedFileCommand(ctx context.Context, client RcloneClient, limit int64, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, client.Path, args...)
	cmd.Env = client.environment()
	cmd.WaitDelay = 3 * time.Second
	hideProcess(cmd)
	var output boundedPreviewOutput
	output.limit = int(limit)
	stderr := &boundedPreviewOutput{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = &output, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("file operation failed: %s", strings.TrimSpace(string(stderr.data)))
	}
	if output.truncated {
		return nil, errors.New("file response exceeds its size limit")
	}
	return output.data, nil
}

func (a *App) PreviewFile(ctx context.Context, value string) (FilePreview, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client := a.EngineClient()
	info, err := fileStat(ctx, client, value)
	result := FilePreview{Path: value, Size: info.Size}
	if err != nil {
		return result, err
	}
	if info.IsDir {
		return result, errors.New("select one file to preview")
	}
	readLimit := int64(256 << 10)
	switch strings.ToLower(filepath.Ext(value)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		readLimit = filePreviewLimit
	}
	var raw []byte
	_, _, kind, _ := browsePath(value)
	if kind == "local" {
		file, err := os.Open(value)
		if err != nil {
			return result, err
		}
		defer file.Close()
		raw, err = io.ReadAll(io.LimitReader(file, readLimit+1))
		if err != nil {
			return result, err
		}
	} else {
		raw, err = boundedFileCommand(ctx, client, readLimit+1, "cat", value, "--count", fmt.Sprint(readLimit+1))
		if err != nil {
			return result, err
		}
	}
	result.MIME = http.DetectContentType(raw)
	if strings.HasPrefix(result.MIME, "image/") {
		if int64(len(raw)) > readLimit || info.Size > readLimit {
			return result, errors.New("image preview is limited to 8 MiB; download the file to open it")
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(raw))
		if err != nil || config.Width <= 0 || config.Height <= 0 ||
			int64(config.Width)*int64(config.Height) > 16_000_000 {
			return result, errors.New("image preview supports PNG, JPEG, GIF and WebP up to 16 megapixels")
		}
		result.Kind, result.Data = "image", raw
		return result, nil
	}
	result.Truncated = len(raw) > 256<<10 || info.Size > int64(len(raw))
	if len(raw) > 256<<10 {
		raw = raw[:256<<10]
		for i := 0; i < 3 && !utf8.Valid(raw); i++ {
			raw = raw[:len(raw)-1]
		}
	}
	if bytes.ContainsRune(raw, 0) || !utf8.Valid(raw) {
		return result, errors.New("this file has no text or image preview; download it to open in another app")
	}
	result.Kind, result.Text = "text", strings.TrimPrefix(string(raw), "\ufeff")
	return result, nil
}
