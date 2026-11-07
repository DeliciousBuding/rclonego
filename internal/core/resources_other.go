//go:build !windows

package core

import (
	"context"
	"time"
)

func sampleResources(context.Context, int, map[int]cpuReading) (ResourceSnapshot, map[int]cpuReading) {
	return ResourceSnapshot{UpdatedAt: time.Now(), Error: "process counters are only available on Windows"}, nil
}
