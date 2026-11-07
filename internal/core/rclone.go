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
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type RcloneClient struct {
	Path       string
	ConfigPath string
	RCToken    string
}

func (r RcloneClient) command(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.Path, args...)
	cmd.Env = r.environment()
	hideProcess(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("rclone command failed: %s", message)
	}
	return stdout.Bytes(), nil
}

func (r RcloneClient) Version(ctx context.Context) (string, error) {
	out, err := r.command(ctx, "version")
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line), nil
}

func (r RcloneClient) Remotes(ctx context.Context) ([]RemoteInfo, error) {
	out, err := r.command(ctx, "listremotes", "--json")
	if err != nil {
		return nil, err
	}
	var remotes []RemoteInfo
	if err := json.Unmarshal(out, &remotes); err != nil {
		return nil, fmt.Errorf("decode remotes: %w", err)
	}
	return remotes, nil
}

func (r RcloneClient) Stats(ctx context.Context, port int) (*RCStats, error) {
	body, err := json.Marshal(map[string]any{})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/core/stats", port), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.RCToken != "" {
		req.SetBasicAuth("rclonego", r.RCToken)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stats status %s", resp.Status)
	}
	var stats RCStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r RcloneClient) SetBandwidth(ctx context.Context, port int, rate string) error {
	body, _ := json.Marshal(map[string]string{"rate": rate})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/core/bwlimit", port), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.RCToken != "" {
		req.SetBasicAuth("rclonego", r.RCToken)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("rclone bandwidth status %s", resp.Status)
	}
	return nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func validateSafeText(value string) error {
	if strings.ContainsAny(value, "\x00\r\n") {
		return errors.New("contains control characters")
	}
	return nil
}

var bandwidthPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?[KMGTP]?$`)
var remotePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+:$`)
var drivePattern = regexp.MustCompile(`^[A-Za-z]:$`)

func EnsureColonRemote(value string) string {
	if strings.HasSuffix(value, ":") {
		return value
	}
	return value + ":"
}

func rcloneTransferArgs(job Job, rcPort int, logPath string) ([]string, error) {
	if err := validateSafeText(job.Source); err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	if err := validateSafeText(job.Destination); err != nil {
		return nil, fmt.Errorf("destination: %w", err)
	}
	if strings.TrimSpace(job.Source) == "" {
		return nil, errors.New("source is required")
	}
	if job.Operation != "oauth-reconnect" && job.Operation != "oauth-create" &&
		strings.TrimSpace(job.Destination) == "" {
		return nil, errors.New("destination is required")
	}
	switch job.Operation {
	case "copy", "copyto", "sync":
		if job.Operation == "copyto" && job.Immutable {
			return nil, errors.New("rclone single-file copy does not enforce immutable; use directory copy with files")
		}
		if job.Operation == "sync" && !job.AllowDelete {
			return nil, errors.New("sync requires explicit allow_delete")
		}
		args := []string{job.Operation, job.Source, job.Destination}
		for _, file := range job.Files {
			clean := filepath.Clean(filepath.FromSlash(file))
			if validateSafeText(file) != nil || file == "" || filepath.IsAbs(file) || filepath.VolumeName(file) != "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return nil, errors.New("files must contain relative paths within the source directory")
			}
		}
		if len(job.Files) > 0 && logPath != "" {
			args = append(args, "--files-from-raw", logPath+".files")
		}
		for _, include := range job.Includes {
			if err := validateSafeText(include); err != nil || include == "" {
				return nil, fmt.Errorf("invalid include %q", include)
			}
			args = append(args, "--include", include)
		}
		if job.Checksum {
			args = append(args, "--checksum")
		}
		if job.Immutable {
			args = append(args, "--immutable")
		}
		for _, exclude := range job.Excludes {
			if err := validateSafeText(exclude); err != nil || exclude == "" {
				return nil, errors.New("invalid exclude")
			}
			args = append(args, "--exclude", exclude)
		}
		settings := TransferSettings{
			Transfers: job.Transfers, Checkers: job.Checkers,
			ChunkSize: ValueOr(job.ChunkSize, "16M"), BufferSize: ValueOr(job.BufferSize, "8M"), Bandwidth: job.Bandwidth,
		}
		if err := settings.Validate(); err != nil {
			return nil, err
		}
		args = append(args, "--drive-chunk-size", settings.ChunkSize, "--buffer-size", settings.BufferSize)
		if job.CreateDirs && job.Operation != "copyto" {
			args = append(args, "--create-empty-src-dirs")
		}
		if job.Bandwidth != "" {
			if !bandwidthPattern.MatchString(job.Bandwidth) {
				return nil, errors.New("invalid bandwidth limit")
			}
			args = append(args, "--bwlimit", job.Bandwidth)
		}
		if job.Operation == "sync" {
			args = append(args, "--max-delete", "10")
		}
		args = append(args,
			"--transfers", strconv.Itoa(job.Transfers),
			"--checkers", strconv.Itoa(job.Checkers),
			"--retries", "5", "--low-level-retries", "10",
			"--stats", "5s", "--stats-one-line",
			"--log-level", "INFO",
			"--use-json-log",
			"--rc", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", rcPort),
		)
		return args, nil
	case "check":
		if job.VerificationMode != "" && job.VerificationMode != "backend" && job.VerificationMode != "content" && job.VerificationMode != "size" {
			return nil, errors.New("verification_mode must be backend, content or size")
		}
		args := []string{"check", job.Source, job.Destination, "--one-way",
			"--checkers", strconv.Itoa(job.Checkers), "--stats", "5s", "--stats-one-line",
			"--log-level", "INFO", "--use-json-log",
			"--rc", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", rcPort)}
		if job.VerificationMode == "content" {
			args = append(args, "--download")
		}
		if job.VerificationMode == "size" {
			args = append(args, "--size-only")
		}
		for _, include := range job.Includes {
			if err := validateSafeText(include); err != nil || include == "" {
				return nil, fmt.Errorf("invalid include %q", include)
			}
			args = append(args, "--include", include)
		}
		for _, exclude := range job.Excludes {
			if err := validateSafeText(exclude); err != nil || exclude == "" {
				return nil, errors.New("invalid exclude")
			}
			args = append(args, "--exclude", exclude)
		}
		return args, nil
	case "oauth-reconnect":
		if !remotePattern.MatchString(EnsureColonRemote(job.Source)) {
			return nil, errors.New("invalid remote name")
		}
		return []string{"config", "reconnect", EnsureColonRemote(job.Source)}, nil
	case "oauth-create":
		return nil, errors.New("multi-step account setup is not available in this preview; create a config with rclone and select it using -config")
	default:
		return nil, errors.New("unsupported operation")
	}
}

func tailFile(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	start := info.Size() - limit
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, 0); err != nil {
		return "", err
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
