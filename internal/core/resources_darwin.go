//go:build darwin

package core

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func sampleResources(ctx context.Context, appPID int, previous map[int]cpuReading) (ResourceSnapshot, map[int]cpuReading) {
	result := ResourceSnapshot{UpdatedAt: time.Now(), LogicalCPUs: runtime.NumCPU(), Goroutines: runtime.NumGoroutine()}
	raw, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,ppid=,pcpu=,rss=,comm=").Output()
	if err != nil {
		result.Error = "process counters unavailable"
		return result, nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		values := strings.Fields(line)
		if len(values) < 5 {
			continue
		}
		pid, _ := strconv.Atoi(values[0])
		parent, _ := strconv.Atoi(values[1])
		if pid != appPID && parent != appPID {
			continue
		}
		cpu, _ := strconv.ParseFloat(values[2], 64)
		rss, _ := strconv.ParseInt(values[3], 10, 64)
		result.Processes = append(result.Processes, ProcessResource{PID: pid, Name: strings.Join(values[4:], " "),
			CPUAvailable: true, CPUPercent: cpu / float64(result.LogicalCPUs), WorkingSetBytes: rss * 1024,
			Error: "private memory and thread counters unavailable; resident memory available as working_set_bytes"})
	}
	return result, nil
}
