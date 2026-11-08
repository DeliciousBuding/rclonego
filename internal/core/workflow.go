package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

type WorkflowRequest struct {
	DefinitionID     string            `json:"definition_id,omitempty"`
	VerificationMode string            `json:"verification_mode,omitempty"`
	Name             string            `json:"name"`
	Source           string            `json:"source"`
	Destination      string            `json:"destination"`
	ProofPath        string            `json:"proof_path,omitempty"`
	IdempotencyKey   string            `json:"idempotency_key"`
	Settings         *TransferSettings `json:"settings,omitempty"`
	NotBefore        *time.Time        `json:"not_before,omitempty"`
	Entries          []string          `json:"entries,omitempty"`
	TotalBytes       int64             `json:"total_bytes,omitempty"`
	InitialBytes     int64             `json:"initial_bytes,omitempty"`
}

type exclusionProof struct {
	Library     string `json:"library"`
	Directories []struct {
		Source string `json:"source"`
		Keep   string `json:"keep"`
		Files  []struct {
			Relative    string `json:"relative"`
			Size        int64  `json:"size"`
			SourceMtime int64  `json:"source_mtime_ns"`
			KeepMtime   int64  `json:"keep_mtime_ns"`
			SHA256      string `json:"sha256"`
		} `json:"files"`
	} `json:"directories"`
}

func proofExclusions(file, root string) ([]string, error) {
	if file == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var proof exclusionProof
	if err = json.Unmarshal(raw, &proof); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(proof.Library), filepath.Clean(absolute)) {
		return nil, errors.New("exclusion proof belongs to another source")
	}
	var excludes []string
	for _, dir := range proof.Directories {
		if dir.Source == dir.Keep || strings.ContainsAny(dir.Source, "*?[]{}\r\n\x00") {
			return nil, errors.New("invalid excluded directory")
		}
		for _, name := range []string{dir.Source, dir.Keep} {
			if _, e := confinedPath(absolute, name); e != nil {
				return nil, e
			}
		}
		if len(dir.Files) == 0 {
			return nil, errors.New("exclusion proof is empty")
		}
		seen := 0
		source := filepath.Join(absolute, filepath.FromSlash(dir.Source))
		records := map[string]bool{}
		for _, entry := range dir.Files {
			if len(entry.SHA256) != 64 {
				return nil, errors.New("proof has no verified SHA-256")
			}
			a, e := confinedPath(source, entry.Relative)
			if e != nil {
				return nil, e
			}
			b, e := confinedPath(filepath.Join(absolute, filepath.FromSlash(dir.Keep)), entry.Relative)
			if e != nil {
				return nil, e
			}
			for i, p := range []string{a, b} {
				st, e := os.Stat(p)
				if e != nil {
					return nil, e
				}
				stamp := entry.SourceMtime
				if i == 1 {
					stamp = entry.KeepMtime
				}
				if !st.Mode().IsRegular() || st.Size() != entry.Size || st.ModTime().UnixNano() != stamp {
					return nil, fmt.Errorf("exclusion proof expired for %s", p)
				}
			}
			records[filepath.Clean(filepath.FromSlash(entry.Relative))] = true
		}
		err = filepath.WalkDir(source, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(source, p)
			if !records[rel] {
				return errors.New("excluded directory has changed")
			}
			seen++
			return nil
		})
		if err != nil || seen != len(records) {
			return nil, errors.New("exclusion proof file set changed")
		}
		excludes = append(excludes, "/"+strings.Trim(dir.Source, "/")+"/**")
	}
	return excludes, nil
}

func confinedPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", errors.New("proof requires a relative path")
	}
	target := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("exclusion proof escapes source")
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("exclusion proof follows a link outside source")
	}
	return target, nil
}

func (q *Queue) AddWorkflow(request WorkflowRequest) ([]Job, error) {
	q.addMu.Lock()
	defer q.addMu.Unlock()
	request.Name = strings.TrimSpace(request.Name)
	request.Source = strings.TrimSpace(request.Source)
	request.Destination = strings.TrimSpace(request.Destination)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.Name == "" || request.Destination == "" || request.Source == "" || request.IdempotencyKey == "" {
		return nil, errors.New("workflow requires name, source, destination and idempotency_key")
	}
	if len(request.IdempotencyKey) > 80 || validateSafeText(request.IdempotencyKey) != nil {
		return nil, errors.New("workflow idempotency key must be at most 80 characters")
	}
	if err := validateWorkflowEntries(request.Entries); err != nil {
		return nil, err
	}
	var existing []Job
	for _, job := range q.store.Snapshot().Jobs {
		if job.WorkflowID == request.IdempotencyKey {
			existing = append(existing, job)
		}
	}
	if len(existing) > 0 {
		sort.Slice(existing, func(i, j int) bool { return existing[i].WorkflowStep < existing[j].WorkflowStep })
		last := existing[len(existing)-1]
		root, _ := filepath.Abs(request.Source)
		if last.Source != root || last.Destination != request.Destination {
			return nil, errors.New("workflow idempotency key already belongs to another source or destination")
		}
		return existing, nil
	}
	settings := q.store.Snapshot().Defaults
	if request.Settings != nil {
		settings = *request.Settings
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(request.Source)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("workflow source must be a local directory")
	}
	excludes, err := proofExclusions(request.ProofPath, root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if len(request.Entries) > 0 {
		byName := make(map[string]os.DirEntry, len(entries))
		for _, entry := range entries {
			byName[entry.Name()] = entry
		}
		selected := make([]os.DirEntry, 0, len(request.Entries))
		seen := map[string]bool{}
		for _, name := range request.Entries {
			name = strings.TrimSpace(name)
			if name == "" || filepath.Base(name) != name || seen[name] {
				return nil, errors.New("workflow entries must be unique top-level names")
			}
			entry, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("workflow entry %q does not exist", name)
			}
			seen[name] = true
			selected = append(selected, entry)
		}
		entries = selected
	}
	var jobs []Job
	for _, entry := range entries {
		operation := "copy"
		source := filepath.Join(root, entry.Name())
		destination := strings.TrimRight(request.Destination, "/") + "/" + entry.Name()
		var files []string
		if !entry.IsDir() {
			source, destination = root, request.Destination
			files = []string{entry.Name()}
		}
		childExcludes := []string{}
		prefix := "/" + entry.Name() + "/"
		for _, excluded := range excludes {
			if strings.HasPrefix(excluded, prefix) {
				childExcludes = append(childExcludes, "/"+strings.TrimPrefix(excluded, prefix))
			}
		}
		job, err := q.buildJob(JobCreateRequest{
			Name: request.Name + " · " + entry.Name(), Operation: operation, WorkflowID: request.IdempotencyKey,
			Source: source, Destination: destination, Files: files,
			IdempotencyKey: request.IdempotencyKey + "/" + fmt.Sprint(len(jobs)+1), Immutable: true, CreateDirs: true, Excludes: childExcludes,
			ProofPath: request.ProofPath, ProofRoot: root,
			Transfers: settings.Transfers, Checkers: settings.Checkers,
			ChunkSize: settings.ChunkSize, BufferSize: settings.BufferSize, Bandwidth: settings.Bandwidth,
			CPUParallelism:     &settings.CPUParallelism,
			NotBefore:          request.NotBefore,
			WorkflowTotalBytes: request.TotalBytes, WorkflowInitialBytes: request.InitialBytes,
		})
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	var checkIncludes []string
	if len(request.Entries) > 0 {
		for _, entry := range entries {
			pattern := "/" + escapeWorkflowFilter(entry.Name())
			if entry.IsDir() {
				pattern += "/**"
			}
			checkIncludes = append(checkIncludes, pattern)
		}
	}
	check, err := q.buildJob(JobCreateRequest{
		Name: request.Name + " · 最终校验", Operation: "check", Source: root, Destination: request.Destination,
		VerificationMode: request.VerificationMode,
		WorkflowID:       request.IdempotencyKey, IdempotencyKey: request.IdempotencyKey + "/verify", Excludes: excludes,
		Includes:  checkIncludes,
		ProofPath: request.ProofPath, ProofRoot: root,
		Transfers: settings.Transfers, Checkers: settings.Checkers,
		ChunkSize: settings.ChunkSize, BufferSize: settings.BufferSize, Bandwidth: settings.Bandwidth,
		CPUParallelism:     &settings.CPUParallelism,
		NotBefore:          request.NotBefore,
		WorkflowTotalBytes: request.TotalBytes, WorkflowInitialBytes: request.InitialBytes,
	})
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, check)
	if err := q.store.Update(func(state *State) error {
		position := lastPosition(state.Jobs)
		for i := range jobs {
			jobs[i].WorkflowStep, jobs[i].WorkflowSize = i+1, len(jobs)
			jobs[i].Position = position + i + 1
		}
		state.Jobs = append(state.Jobs, jobs...)
		state.Runs = append(state.Runs, Run{ID: request.IdempotencyKey, WorkflowID: request.DefinitionID, Name: request.Name, CreatedAt: time.Now().UTC()})
		return nil
	}); err != nil {
		return nil, err
	}
	q.Wake()
	return jobs, nil
}

func validateWorkflowEntries(entries []string) error {
	seen := make(map[string]bool, len(entries))
	for _, name := range entries {
		if name == "" || strings.TrimSpace(name) != name || name == "." || name == ".." ||
			filepath.Base(name) != name || strings.ContainsAny(name, `/\`) || validateSafeText(name) != nil || seen[name] {
			return errors.New("workflow entries must be unique top-level names")
		}
		seen[name] = true
	}
	return nil
}

// rclone filters are globs, while workflow entry names are literal filenames.
func escapeWorkflowFilter(name string) string {
	return strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`, "{", `\{`, "}", `\}`).Replace(name)
}

func WorkflowBlocked(jobs []Job, job Job) bool {
	if job.WorkflowID == "" {
		return false
	}
	if job.WorkflowStep <= 0 {
		return true // Unknown legacy dependency order must never skip a check.
	}
	completed := map[int]bool{}
	for _, previous := range jobs {
		if previous.WorkflowID == job.WorkflowID && previous.WorkflowStep < job.WorkflowStep && previous.Status == "completed" {
			completed[previous.WorkflowStep] = true
		}
	}
	return len(completed) != job.WorkflowStep-1
}

func (q *Queue) validateProof(job Job) error {
	if job.ProofPath == "" {
		return nil
	}
	excludes, err := proofExclusions(job.ProofPath, job.ProofRoot)
	if err != nil {
		return err
	}
	if len(job.Files) > 0 {
		excludes = nil
	} else if job.Operation != "check" {
		relative, err := filepath.Rel(job.ProofRoot, job.Source)
		if err != nil {
			return err
		}
		prefix := "/" + filepath.ToSlash(relative) + "/"
		var child []string
		for _, rule := range excludes {
			if strings.HasPrefix(rule, prefix) {
				child = append(child, "/"+strings.TrimPrefix(rule, prefix))
			}
		}
		excludes = child
	}
	if !slices.Equal(excludes, job.Excludes) {
		return errors.New("exclusion proof no longer matches the saved task")
	}
	return nil
}

// Local directory workflows share the queue with upload/download jobs.
func (a *App) addWorkflow(ctx context.Context, request WorkflowRequest) ([]Job, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return a.queue.AddWorkflow(request)
}
