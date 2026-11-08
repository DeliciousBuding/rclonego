//go:build linux || darwin

package core

import (
	"path/filepath"
	"testing"
)

func syntheticMountTarget(root string) string {
	return filepath.Join(root, "mount with spaces")
}

func TestOwnedMountPortReadsExactArgumentsWithSpacedPaths(t *testing.T) {
	config := MountConfig{Remote: "fixture:directory with spaces", Drive: filepath.Join(t.TempDir(), "mount with spaces")}
	args := []string{"/synthetic/rclone", "mount", config.Remote, config.Drive, "--cache-dir", "/synthetic/cache", "--rc", "--rc-addr", "127.0.0.1:12345"}
	port, err := mountArgsRCPort(args, config)
	if err != nil || port != 12345 {
		t.Fatalf("spaced paths lost their mount identity: %d / %v", port, err)
	}
	for _, changed := range [][]string{
		{"/synthetic/rclone", "rcd", config.Remote, config.Drive, "--rc-addr", "127.0.0.1:12345"},
		{"/synthetic/rclone", "mount", "another:", config.Drive, "--rc-addr", "127.0.0.1:12345"},
		{"/synthetic/rclone", "mount", config.Remote, config.Drive, "--rc-addr", "0.0.0.0:12345"},
		{"/synthetic/rclone", "mount", config.Remote, config.Drive, "--rc-addr", "127.0.0.1:65536"},
	} {
		if _, err := mountArgsRCPort(changed, config); err == nil {
			t.Fatalf("invalid mount command was accepted: %q", changed)
		}
	}
}
