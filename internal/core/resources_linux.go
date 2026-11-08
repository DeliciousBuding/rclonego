//go:build linux

package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func sampleResources(ctx context.Context, appPID int, previous map[int]cpuReading) (ResourceSnapshot, map[int]cpuReading) {
	result := ResourceSnapshot{UpdatedAt: time.Now(), LogicalCPUs: runtime.NumCPU(), Goroutines: runtime.NumGoroutine()}
	readings := map[int]cpuReading{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		result.Error = "process counters unavailable"
		return result, readings
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		text := string(raw)
		start, end := strings.IndexByte(text, '('), strings.LastIndexByte(text, ')')
		if start < 0 || end < start {
			continue
		}
		fields := strings.Fields(text[end+1:])
		if len(fields) < 22 {
			continue
		}
		parent, _ := strconv.Atoi(fields[1])
		name := text[start+1 : end]
		external := pid != appPID && parent != appPID
		if external && !isRcloneProcess(name) {
			continue
		}
		number := func(index int) uint64 { value, _ := strconv.ParseUint(fields[index], 10, 64); return value }
		row := ProcessResource{PID: pid, Name: name, Threads: int(number(17)), External: external}
		// Linux exposes CPU accounting in USER_HZ (100 ticks/s) on our targets.
		reading := cpuReading{Created: number(19), Ticks: (number(11) + number(12)) * 100000, At: result.UpdatedAt}
		if old, ok := previous[pid]; ok && old.Created == reading.Created && reading.Ticks >= old.Ticks {
			row.CPUAvailable, row.CPUPercent = true, cpuUsage(reading.Ticks-old.Ticks, reading.At.Sub(old.At), result.LogicalCPUs)
		}
		readings[pid] = reading
		if memory, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "statm")); err == nil {
			values := strings.Fields(string(memory))
			if len(values) >= 3 {
				resident, _ := strconv.ParseInt(values[1], 10, 64)
				shared, _ := strconv.ParseInt(values[2], 10, 64)
				row.WorkingSetBytes = resident * int64(os.Getpagesize())
				row.MemoryBytes = max(0, resident-shared) * int64(os.Getpagesize())
				row.MemoryAvailable = true
			}
		}
		result.Processes = append(result.Processes, row)
	}
	return result, readings
}
