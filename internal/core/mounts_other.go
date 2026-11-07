//go:build !windows

package core

import (
	"context"
	"errors"
)

func inspectMounts(context.Context, []MountConfig) ([]MountView, error) {
	return nil, errors.New("mount management is implemented for Windows")
}

func startMount(context.Context, RcloneClient, string, MountConfig) (int, error) {
	return 0, errors.New("mount management is implemented for Windows")
}

func stopMount(context.Context, MountConfig, int, string) error {
	return errors.New("mount management is implemented for Windows")
}

func ValidateMountConfig(MountConfig) error {
	return errors.New("mount management is implemented for Windows")
}
