//go:build windows

package core

import "syscall"

var getLogicalDrives = syscall.NewLazyDLL("kernel32.dll").NewProc("GetLogicalDrives")

func availableMountTargets(_ string) MountTargets {
	mask, _, _ := getLogicalDrives.Call()
	var result MountTargets
	if mask == 0 {
		return result
	}
	for letter := 'K'; letter <= 'Z'; letter++ {
		if mask&(1<<uint(letter-'A')) == 0 {
			result.Available = append(result.Available, string(letter)+":")
		}
	}
	for letter := 'D'; letter < 'K'; letter++ {
		if mask&(1<<uint(letter-'A')) == 0 {
			result.Available = append(result.Available, string(letter)+":")
		}
	}
	return result
}
