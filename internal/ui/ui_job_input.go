package desktop

import (
	"strconv"
	"strings"
	"time"

	"github.com/DeliciousBuding/rclonego/internal/core"
)

type jobInputIssue struct {
	field, message string
}

// This builds the same request for preview and submission. The core remains
// responsible for backend access, sync confirmation and execution validation.
func (s *nativeUI) editedJob() (core.JobCreateRequest, *jobInputIssue) {
	request := core.JobCreateRequest{
		Name: strings.TrimSpace(s.jobName), Operation: s.jobOperation,
		Source: strings.TrimSpace(s.jobSource), Destination: strings.TrimSpace(s.jobDestination),
		Bandwidth: strings.TrimSpace(s.jobBandwidth), ChunkSize: strings.TrimSpace(s.jobChunk),
		BufferSize: strings.TrimSpace(s.jobBuffer), Checksum: s.jobChecksum,
		CreateDirs: s.jobCreateDirs, AllowDelete: s.jobAllowDelete, PreviewID: s.jobPreview.ID,
	}
	for _, required := range []struct{ field, value, message string }{
		{"name", request.Name, "请输入任务名称"},
		{"source", request.Source, "请选择源目录"},
		{"destination", request.Destination, "请选择目标目录"},
	} {
		if required.value == "" {
			return request, &jobInputIssue{required.field, required.message}
		}
	}
	if request.Source == request.Destination {
		return request, &jobInputIssue{"destination", "源目录和目标目录不能相同"}
	}
	transfers, err := strconv.Atoi(s.jobTransfers)
	if err != nil {
		return request, &jobInputIssue{"parameters", "传输并发必须为 1–32，校验并发必须为 1–64"}
	}
	checkers, err := strconv.Atoi(s.jobCheckers)
	if err != nil {
		return request, &jobInputIssue{"parameters", "传输并发必须为 1–32，校验并发必须为 1–64"}
	}
	cpus, err := strconv.Atoi(s.jobCPU)
	if err != nil {
		return request, &jobInputIssue{"parameters", "CPU 并行度必须为 0–64"}
	}
	request.Transfers, request.Checkers, request.CPUParallelism = transfers, checkers, &cpus
	settings := core.TransferSettings{
		Transfers: transfers, Checkers: checkers, CPUParallelism: cpus,
		ChunkSize: request.ChunkSize, BufferSize: request.BufferSize, Bandwidth: request.Bandwidth,
	}
	if settings.Bandwidth == "off" {
		settings.Bandwidth = ""
	}
	if err := settings.Validate(); err != nil {
		message := map[string]string{
			"CPU parallelism must be 0 (automatic) or 1–64":           "CPU 并行度必须为 0–64",
			"transfers must be 1–32 and checkers 1–64":                "传输并发必须为 1–32，校验并发必须为 1–64",
			"Drive chunk size must be a power of two from 256K to 1G": "Google Drive 分块需为 256K–1G 之间的 2 的幂",
			"buffer size must be between 0 and 1G":                    "每路缓冲需为 0–1G，例如 8M",
		}[err.Error()]
		if message == "" {
			message = err.Error()
			if strings.HasPrefix(message, "bandwidth:") {
				message = "限速格式无效，请使用 4M 或 off"
			}
		}
		return request, &jobInputIssue{"parameters", message}
	}
	if s.jobOperation == "sync" {
		maxDelete, err := strconv.Atoi(s.jobMaxDelete)
		if err != nil || maxDelete < 1 || maxDelete > 10000 {
			return request, &jobInputIssue{"max-delete", "单次删除上限必须为 1–10000"}
		}
		request.MaxDelete = maxDelete
	}
	if schedule := strings.TrimSpace(s.jobSchedule); schedule != "" {
		at, err := time.ParseInLocation("2006-01-02 15:04", schedule, time.Local)
		if err != nil {
			return request, &jobInputIssue{"schedule", "计划时间格式为 YYYY-MM-DD HH:MM"}
		}
		request.NotBefore = &at
	}
	for _, line := range strings.Split(s.jobIncludes, "\n") {
		if value := strings.TrimSpace(line); value != "" {
			request.Includes = append(request.Includes, value)
		}
	}
	return request, nil
}
