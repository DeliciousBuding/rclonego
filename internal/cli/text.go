package cli

import (
	"fmt"
	"strconv"
)

func progressValue(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return min(1, max(0, float64(done)/float64(total)))
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func formatBytes(n int64) string {
	if n < 0 {
		return "—"
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	size := float64(n)
	for _, unit := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		size /= 1024
		if size < 1024 {
			return fmt.Sprintf("%.2f %s", size, unit)
		}
	}
	return strconv.FormatInt(n, 10) + " B"
}

func statusLabel(status string) string {
	switch status {
	case "paused":
		return "已暂停"
	case "pending":
		return "等待"
	case "running":
		return "运行中"
	case "completed":
		return "完成"
	case "interrupted":
		return "中断"
	case "failed":
		return "失败"
	case "cancelled":
		return "已取消"
	}
	return status
}

func operationLabel(operation string) string {
	switch operation {
	case "copy", "copyto":
		return "复制"
	case "sync":
		return "单向镜像"
	case "check":
		return "校验"
	}
	return operation
}
