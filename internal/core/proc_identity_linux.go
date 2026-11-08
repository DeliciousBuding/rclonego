//go:build linux

package core

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

var processBootID struct {
	sync.Once
	value string
}

func processIdentity(pid int) string {
	if pid <= 0 {
		return ""
	}
	processBootID.Do(func() {
		raw, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
		if err == nil {
			processBootID.value = strings.TrimSpace(string(raw))
		}
	})
	if processBootID.value == "" {
		return ""
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	start := processStartTicks(string(raw))
	if start == "" {
		return ""
	}
	return processBootID.value + ":" + start
}

func processStartTicks(stat string) string {
	// The command name can contain spaces and parentheses. Fields after its
	// last closing parenthesis start at field 3; starttime is field 22.
	end := strings.LastIndex(stat, ")")
	if end < 0 {
		return ""
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) < 20 {
		return ""
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return ""
	}
	return fields[19]
}
