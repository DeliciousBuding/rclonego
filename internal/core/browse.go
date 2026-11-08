package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	browsePageLimit = 500
	browseScanLimit = 10000
	browseByteLimit = 8 << 20
	browsePathLimit = 32 << 10
)

// BrowseRequest lists one directory. Pagination reflects the current listing,
// not a persistent snapshot; callers should refresh after changing its contents.
type BrowseRequest struct {
	Path            string `json:"path"`
	Offset          int    `json:"offset"`
	Limit           int    `json:"limit"`
	DirectoriesOnly bool   `json:"directories_only"`
}

type DirectoryEntry struct {
	Name    string     `json:"name"`
	Path    string     `json:"path"`
	IsDir   bool       `json:"is_dir"`
	Size    int64      `json:"size"`
	ModTime *time.Time `json:"mod_time,omitempty"`
}

type BrowseResult struct {
	Path       string           `json:"path"`
	Parent     string           `json:"parent"`
	Kind       string           `json:"kind"`
	Entries    []DirectoryEntry `json:"entries"`
	NextOffset int              `json:"next_offset"`
	HasMore    bool             `json:"has_more"`
	Truncated  bool             `json:"truncated"`
}

func (a *App) Browse(ctx context.Context, request BrowseRequest) (BrowseResult, error) {
	root, parent, kind, err := browsePath(request.Path)
	if err != nil {
		return BrowseResult{}, err
	}
	if request.Offset < 0 || request.Offset >= browseScanLimit {
		return BrowseResult{}, errors.New("directory offset must be between 0 and 9999")
	}
	if request.Limit == 0 {
		request.Limit = 100
	}
	if request.Limit < 1 || request.Limit > browsePageLimit {
		return BrowseResult{}, errors.New("directory limit must be between 1 and 500")
	}
	var entries []DirectoryEntry
	var truncated bool
	if kind == "local" {
		entries, truncated, err = browseLocal(ctx, root, request.DirectoriesOnly)
	} else {
		entries, truncated, err = browseRemote(ctx, a.EngineClient(), root, request.DirectoriesOnly)
	}
	if err != nil {
		return BrowseResult{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		left, right := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if left != right {
			return left < right
		}
		return entries[i].Name < entries[j].Name
	})
	start := min(request.Offset, len(entries))
	end := min(start+request.Limit, len(entries))
	page := append([]DirectoryEntry{}, entries[start:end]...)
	return BrowseResult{
		Path: root, Parent: parent, Kind: kind, Entries: page, NextOffset: end,
		HasMore: end < len(entries), Truncated: truncated,
	}, nil
}

// A remote is always explicit. Never reinterpret a relative local path as a
// configured remote, or accept an inline backend connection string.
func browsePath(value string) (root, parent, kind string, err error) {
	if value == "" || validateSafeText(value) != nil {
		return "", "", "", errors.New("directory path is required and must not contain control characters")
	}
	if len(value) > browsePathLimit {
		return "", "", "", errors.New("directory path exceeds the 32 KiB limit")
	}
	if filepath.IsAbs(value) {
		root = filepath.Clean(value)
		parent = filepath.Dir(root)
		if parent == root {
			parent = ""
		}
		return root, parent, "local", nil
	}
	if filepath.VolumeName(value) != "" {
		return "", "", "", errors.New("local directory must be absolute, including its drive root")
	}
	name, suffix, ok := strings.Cut(value, ":")
	if !ok || !remotePattern.MatchString(name+":") {
		return "", "", "", errors.New("use an absolute local directory or an explicit remote:path")
	}
	for _, component := range strings.Split(suffix, "/") {
		if component == ".." {
			return "", "", "", errors.New("remote directory must not contain parent traversal")
		}
	}
	subdir := suffix
	if subdir != "" {
		subdir = path.Clean(subdir)
		if subdir == "." {
			subdir = ""
		}
	}
	root = name + ":" + subdir
	if subdir != "" && subdir != "/" {
		up := path.Dir(subdir)
		if up == "." {
			up = ""
		}
		parent = name + ":" + up
	}
	return root, parent, "remote", nil
}

func browseLocal(ctx context.Context, root string, dirsOnly bool) ([]DirectoryEntry, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	folder, err := os.Open(root)
	if err != nil {
		return nil, false, fmt.Errorf("open directory: %w", err)
	}
	defer folder.Close()
	entries := make([]DirectoryEntry, 0, 100)
	scanned := 0
	entryBytes := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		batch, err := folder.ReadDir(128)
		for _, item := range batch {
			scanned++
			if scanned > browseScanLimit {
				return entries, true, nil
			}
			if dirsOnly && !item.IsDir() {
				continue
			}
			entry := DirectoryEntry{Name: item.Name(), Path: filepath.Join(root, item.Name()), IsDir: item.IsDir(), Size: -1}
			entryBytes += len(entry.Name) + len(entry.Path) + 64
			if entryBytes > browseByteLimit {
				return entries, true, nil
			}
			if !dirsOnly {
				info, statErr := item.Info()
				if statErr != nil {
					return nil, false, fmt.Errorf("read directory entry: %w", statErr)
				}
				entry.Size = info.Size()
				modified := info.ModTime()
				entry.ModTime = &modified
			}
			entries = append(entries, entry)
		}
		if errors.Is(err, io.EOF) {
			return entries, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read directory: %w", err)
		}
	}
}

func browseRemote(ctx context.Context, client RcloneClient, root string, dirsOnly bool) ([]DirectoryEntry, bool, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := []string{"lsjson", root, "--no-modtime", "--no-mimetype", "--max-depth", "1"}
	if dirsOnly {
		args = append(args, "--dirs-only")
	}
	cmd := exec.CommandContext(commandCtx, client.Path, args...)
	cmd.Env = client.environment()
	hideProcess(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	// Error output is deliberately bounded as well as the directory listing.
	cmd.Stderr = &boundedPreviewOutput{limit: 16 << 10}
	if err := cmd.Start(); err != nil {
		return nil, false, fmt.Errorf("start directory listing: %w", err)
	}
	stream := &io.LimitedReader{R: stdout, N: browseByteLimit}
	decoder := json.NewDecoder(stream)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		cancel()
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, errors.New("directory listing failed; check the path and account authorization")
	}
	entries := make([]DirectoryEntry, 0, 100)
	truncated := false
	entryBytes := 0
	for decoder.More() {
		if len(entries) >= browseScanLimit {
			truncated = true
			break
		}
		var item struct {
			Name  string `json:"Name"`
			IsDir bool   `json:"IsDir"`
			Size  int64  `json:"Size"`
		}
		if err = decoder.Decode(&item); err != nil {
			if stream.N == 0 {
				truncated = true
			}
			break
		}
		if item.Name == "" || item.Name == "." || item.Name == ".." || strings.Contains(item.Name, "/") || validateSafeText(item.Name) != nil {
			err = errors.New("directory listing contains an unusable child path")
			break
		}
		separator := ""
		if !strings.HasSuffix(root, ":") && !strings.HasSuffix(root, "/") {
			separator = "/"
		}
		entry := DirectoryEntry{Name: item.Name, Path: root + separator + item.Name, IsDir: item.IsDir, Size: item.Size}
		entryBytes += len(entry.Name) + len(entry.Path) + 64
		if entryBytes > browseByteLimit {
			truncated = true
			break
		}
		entries = append(entries, entry)
	}
	if !truncated && err == nil {
		token, err = decoder.Token()
		if err == nil && token != json.Delim(']') {
			err = errors.New("invalid directory listing")
		}
	}
	if truncated || err != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if truncated {
		return entries, true, nil
	}
	if err != nil || waitErr != nil {
		if commandCtx.Err() != nil {
			return nil, false, commandCtx.Err()
		}
		return nil, false, errors.New("directory listing failed; check the path and account authorization")
	}
	return entries, false, nil
}
