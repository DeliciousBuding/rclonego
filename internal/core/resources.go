package core

import (
	"strconv"
	"time"
)

type ProcessResource struct {
	PID             int     `json:"pid"`
	Name            string  `json:"name"`
	CPUPercent      float64 `json:"cpu_percent"`
	CPUAvailable    bool    `json:"cpu_available"`
	MemoryBytes     int64   `json:"memory_bytes"`
	WorkingSetBytes int64   `json:"working_set_bytes"`
	MemoryAvailable bool    `json:"memory_available"`
	Threads         int     `json:"threads"`
	Error           string  `json:"error,omitempty"`
}

type ResourceSnapshot struct {
	UpdatedAt   time.Time         `json:"updated_at"`
	Processes   []ProcessResource `json:"processes"`
	Error       string            `json:"error,omitempty"`
	LogicalCPUs int               `json:"logical_cpus"`
	Goroutines  int               `json:"app_goroutines"`
}

func itoa(value int) string { return strconv.Itoa(value) }

type cpuReading struct {
	Created uint64
	Ticks   uint64
	At      time.Time
}

func cpuUsage(ticks uint64, elapsed time.Duration, cores int) float64 {
	if elapsed <= 0 || cores < 1 {
		return 0
	}
	value := float64(ticks) / 1e7 / elapsed.Seconds() / float64(cores) * 100
	if value > 100 {
		return 100
	}
	return value
}
