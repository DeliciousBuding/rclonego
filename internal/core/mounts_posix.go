//go:build linux || darwin

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func mounted(ctx context.Context, target string) bool {
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile("/proc/self/mountinfo")
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 4 {
				path := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`).Replace(fields[4])
				if path == target {
					return true
				}
			}
		}
		return false
	}
	raw, err := exec.CommandContext(ctx, "/sbin/mount").Output()
	return err == nil && strings.Contains(string(raw), " on "+target+" (")
}

func inspectMounts(ctx context.Context, configs []MountConfig) ([]MountView, error) {
	result := make([]MountView, 0, len(configs))
	for _, cfg := range configs {
		ready := mounted(ctx, cfg.Drive)
		result = append(result, MountView{MountConfig: cfg, Running: ready, DriveAvailable: ready})
	}
	return result, nil
}

func ValidateMountConfig(cfg MountConfig) error {
	if cfg.ID == "" || strings.ContainsAny(cfg.ID, `/\\`) {
		return errors.New("invalid mount id")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("mount name is required")
	}
	if !filepath.IsAbs(cfg.Drive) || filepath.Clean(cfg.Drive) == string(filepath.Separator) || validateSafeText(cfg.Drive) != nil {
		return errors.New("mount target must be an absolute directory below the filesystem root")
	}
	if cfg.Remote == "" || validateSafeText(cfg.Remote) != nil {
		return errors.New("invalid remote")
	}
	if !filepath.IsAbs(cfg.CacheDir) || validateSafeText(cfg.CacheDir) != nil {
		return errors.New("cache directory must be absolute")
	}
	if cfg.CacheMax != "" && !bandwidthPattern.MatchString(cfg.CacheMax) {
		return errors.New("invalid cache size")
	}
	if cfg.CacheMaxAge != "" {
		duration, err := time.ParseDuration(cfg.CacheMaxAge)
		if err != nil || duration <= 0 {
			return errors.New("invalid cache age")
		}
	}
	return nil
}

func startMount(ctx context.Context, client RcloneClient, dataDir string, cfg MountConfig) (int, error) {
	if err := ValidateMountConfig(cfg); err != nil {
		return 0, err
	}
	if diagnostic := mountDependency(); diagnostic.Status != "ready" {
		return 0, errors.New(diagnostic.Message)
	}
	if mounted(ctx, cfg.Drive) {
		return 0, errors.New("mount target is already in use")
	}
	if err := os.MkdirAll(cfg.Drive, 0o700); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(cfg.Drive)
	if err != nil || len(entries) != 0 {
		return 0, errors.New("mount target must be an empty readable directory")
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
		return 0, err
	}
	logDir := filepath.Join(dataDir, "mounts")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return 0, err
	}
	log, err := os.OpenFile(filepath.Join(logDir, cfg.ID+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer log.Close()
	port, err := freePort()
	if err != nil {
		return 0, err
	}
	args := []string{"mount", cfg.Remote, cfg.Drive, "--config", client.ConfigPath,
		"--vfs-cache-mode", "full", "--cache-dir", cfg.CacheDir, "--vfs-cache-max-size", ValueOr(cfg.CacheMax, "15G"),
		"--vfs-cache-max-age", ValueOr(cfg.CacheMaxAge, "24h"), "--rc", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", port)}
	cmd := exec.Command(client.Path, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr = client.environment(), log, log
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return 0, errors.New("mount process exited before target became ready")
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-done
			return 0, ctx.Err()
		case <-deadline.C:
			_ = cmd.Process.Kill()
			<-done
			return 0, errors.New("mount did not become ready; inspect mount dependency and engine build")
		case <-ticker.C:
			if mounted(ctx, cfg.Drive) {
				return cmd.Process.Pid, nil
			}
		}
	}
}

func stopMount(ctx context.Context, cfg MountConfig, pid int, _ string) error {
	// App verifies the recorded process start identity before calling this.
	if pid <= 0 {
		return errors.New("mount owner unavailable")
	}
	cmd, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil || !strings.Contains(string(cmd), "mount") || !strings.Contains(string(cmd), cfg.Remote) || !strings.Contains(string(cmd), cfg.Drive) {
		return errors.New("recorded process no longer matches this mount")
	}
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.CommandContext(ctx, "/sbin/umount", cfg.Drive)
	} else {
		path, err := exec.LookPath("fusermount3")
		if err != nil {
			path, err = exec.LookPath("fusermount")
		}
		if err != nil {
			return errors.New("fusermount is unavailable")
		}
		command = exec.CommandContext(ctx, path, "-u", cfg.Drive)
	}
	if err := command.Run(); err != nil {
		return errors.New("unmount failed; mount remains owned by this instance")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !mounted(ctx, cfg.Drive) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("target is still mounted")
}

func mountDependency() Diagnostic {
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/fuse"); err == nil {
			if _, err := exec.LookPath("fusermount3"); err == nil {
				return Diagnostic{ID: "mount", Status: "ready", Message: "FUSE device and fusermount3 available"}
			}
			if _, err := exec.LookPath("fusermount"); err == nil {
				return Diagnostic{ID: "mount", Status: "ready", Message: "FUSE device and fusermount available"}
			}
		}
		return Diagnostic{ID: "mount", Status: "missing", Message: "FUSE device or fusermount is missing", NextAction: "install distribution FUSE support and grant access to /dev/fuse"}
	}
	for _, path := range []string{"/Library/Filesystems/macfuse.fs", "/Library/Filesystems/osxfuse.fs"} {
		if _, err := os.Stat(path); err == nil {
			return Diagnostic{ID: "mount", Status: "ready", Message: "FUSE installation detected; engine mount support must also be available"}
		}
	}
	return Diagnostic{ID: "mount", Status: "missing", Message: "macFUSE is not installed; normal transfers remain available", NextAction: "install official macFUSE and use a compatible rclone build"}
}
