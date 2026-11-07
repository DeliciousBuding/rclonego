//go:build windows

package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type mountProcess struct {
	ProcessID   int    `json:"ProcessId"`
	CommandLine string `json:"CommandLine"`
}

var mountCommandPattern = regexp.MustCompile(`(?i)(?:^|\s)mount\s+(\S+)\s+([A-Za-z]:)`)
var mountRCPortPattern = regexp.MustCompile(`(?i)--rc-addr\s+127\.0\.0\.1:(\d+)`)

func listRcloneMountProcesses(ctx context.Context) ([]mountProcess, error) {
	var processes []mountProcess
	snapshot, err := syscall.CreateToolhelp32Snapshot(2, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snapshot)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), "rclone.exe") {
			continue
		}
		command := processCommandLine(entry.ProcessID)
		if mountCommandPattern.MatchString(command) {
			processes = append(processes, mountProcess{ProcessID: int(entry.ProcessID), CommandLine: command})
		}
	}
	return processes, nil
}

var ntQueryInformationProcess = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQueryInformationProcess")
var getDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

func processCommandLine(pid uint32) string {
	handle, err := syscall.OpenProcess(0x1000, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(handle)
	buffer := make([]byte, 65536)
	var length uint32
	status, _, _ := ntQueryInformationProcess.Call(uintptr(handle), 60, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&length)))
	if status != 0 {
		return ""
	}
	header := (*struct {
		Length, MaximumLength uint16
		Buffer                *uint16
	})(unsafe.Pointer(&buffer[0]))
	if header.Buffer == nil || header.Length == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(header.Buffer, int(header.Length)/2))
}

func inspectMounts(ctx context.Context, configs []MountConfig) ([]MountView, error) {
	processes, err := listRcloneMountProcesses(ctx)
	if err != nil {
		return nil, err
	}
	byDrive := map[string]mountProcess{}
	for _, process := range processes {
		match := mountCommandPattern.FindStringSubmatch(process.CommandLine)
		if len(match) == 3 {
			byDrive[strings.ToUpper(match[2])] = process
		}
	}
	result := make([]MountView, 0, len(configs))
	for _, cfg := range configs {
		view := MountView{MountConfig: cfg, DriveAvailable: pathExists(cfg.Drive + `\`)}
		// An inherited elevated mount may hide its command line. The visible
		// drive is still usable; do not claim that its PID is owned.
		view.Running = view.DriveAvailable
		if process, ok := byDrive[strings.ToUpper(cfg.Drive)]; ok {
			view.Running = true
			view.PID = process.ProcessID
			if match := mountRCPortPattern.FindStringSubmatch(process.CommandLine); len(match) == 2 {
				view.RCPort, _ = strconv.Atoi(match[1])
			}
			view.CommandLine = process.CommandLine
		}
		result = append(result, view)
	}
	return result, nil
}

func pathExists(path string) bool {
	if len(path) == 3 && path[1] == ':' {
		root, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return false
		}
		kind, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(root)))
		return kind > 1
	}
	_, err := os.Stat(path)
	return err == nil
}

func ValidateMountConfig(cfg MountConfig) error {
	if cfg.ID == "" || strings.ContainsAny(cfg.ID, `/\\`) || validateSafeText(cfg.ID) != nil {
		return errors.New("invalid mount id")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("mount name is required")
	}
	if !drivePattern.MatchString(cfg.Drive) {
		return errors.New("invalid drive")
	}
	cfg.Drive = strings.ToUpper(cfg.Drive)
	if !strings.HasSuffix(cfg.Remote, ":") || strings.ContainsAny(cfg.Remote, " \t\r\n\x00") {
		return errors.New("invalid remote")
	}
	if cfg.CacheDir == "" || strings.ContainsAny(cfg.CacheDir, "\r\n\x00") {
		return errors.New("invalid cache directory")
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
	mounts, err := listRcloneMountProcesses(ctx)
	if err != nil {
		return 0, err
	}
	for _, process := range mounts {
		match := mountCommandPattern.FindStringSubmatch(process.CommandLine)
		if len(match) == 3 && strings.EqualFold(match[2], cfg.Drive) {
			return 0, errors.New("mount target is already owned by another process")
		}
	}
	if pathExists(cfg.Drive + `\`) {
		return 0, fmt.Errorf("drive %s is already in use", cfg.Drive)
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o700); err != nil {
		return 0, err
	}
	logDir := filepath.Join(dataDir, "mounts")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return 0, err
	}
	logPath := filepath.Join(logDir, cfg.ID+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	args := []string{
		"mount", cfg.Remote, cfg.Drive,
		"--vfs-cache-mode", "full",
		"--network-mode",
		"--drive-skip-shortcuts",
		"--dir-cache-time", "1000h",
		"--poll-interval", "15s",
		"--cache-dir", cfg.CacheDir,
		"--vfs-cache-max-size", ValueOr(cfg.CacheMax, "15G"),
		"--vfs-cache-max-age", ValueOr(cfg.CacheMaxAge, "24h"),
		"--log-level", "INFO",
	}
	rcPort, err := freePort()
	if err != nil {
		return 0, err
	}
	args = append(args, "--config", client.ConfigPath, "--rc", "--rc-addr", fmt.Sprintf("127.0.0.1:%d", rcPort))
	if cfg.VolumeName != "" {
		args = append(args, "--volname", cfg.VolumeName)
	}
	cmd := exec.Command(client.Path, args...)
	cmd.Env = client.environment()
	hideProcess(cmd)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if pathExists(cfg.Drive + `\`) {
			return pid, nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-done
			return 0, ctx.Err()
		case <-done:
			return 0, errors.New("mount process exited before drive became ready")
		case <-time.After(250 * time.Millisecond):
		}
	}
	_ = cmd.Process.Kill()
	<-done
	return 0, errors.New("mount did not become ready; failed process stopped")
}

func mountDependency() Diagnostic {
	for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		if root == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "WinFsp", "bin", "winfsp-x64.dll")); err == nil {
			return Diagnostic{ID: "mount", Status: "ready", Message: "WinFsp installation detected"}
		}
	}
	return Diagnostic{ID: "mount", Status: "missing", Message: "WinFsp is not installed; transfers remain available", NextAction: "install WinFsp from https://winfsp.dev"}
}

func stopMount(ctx context.Context, cfg MountConfig, ownedPID int, rcToken string) error {
	processes, err := listRcloneMountProcesses(ctx)
	if err != nil {
		return err
	}
	for _, process := range processes {
		match := mountCommandPattern.FindStringSubmatch(process.CommandLine)
		if len(match) != 3 || process.ProcessID != ownedPID || !strings.EqualFold(match[1], cfg.Remote) || !strings.EqualFold(match[2], cfg.Drive) {
			continue
		}
		if portMatch := mountRCPortPattern.FindStringSubmatch(process.CommandLine); len(portMatch) == 2 {
			if port, parseErr := strconv.Atoi(portMatch[1]); parseErr == nil && port > 0 {
				if err := quitRcloneMount(ctx, port, rcToken); err == nil {
					deadline := time.Now().Add(8 * time.Second)
					for time.Now().Before(deadline) {
						if !pathExists(cfg.Drive + `\`) {
							return nil
						}
						time.Sleep(250 * time.Millisecond)
					}
				}
			}
		}
		processHandle, err := os.FindProcess(process.ProcessID)
		if err != nil {
			return err
		}
		if err := processHandle.Kill(); err != nil {
			return fmt.Errorf("stop pid %d: %w", process.ProcessID, err)
		}
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if !pathExists(cfg.Drive + `\`) {
				return nil
			}
			time.Sleep(250 * time.Millisecond)
		}
		return errors.New("mount process stopped but drive is still visible")
	}
	return errors.New("matching rclone mount process was not found")
}

func quitRcloneMount(ctx context.Context, port int, rcToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/core/quit", port), strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("rclonego", rcToken)
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rclone quit status %s", resp.Status)
	}
	return nil
}
