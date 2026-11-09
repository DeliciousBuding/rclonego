//go:build !windows

package desktop

import "github.com/egoist/mygo"

func configureNativeFrame(*mygo.Window) {}

func updateNativeFrame(*mygo.Window) {}
