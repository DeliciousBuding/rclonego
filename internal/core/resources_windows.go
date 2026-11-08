//go:build windows

package core

import (
	"context"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var getProcessMemoryInfo = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type processMemoryCounters struct {
	Size, PageFaultCount                               uint32
	PeakWorkingSetSize, WorkingSetSize                 uintptr
	QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage       uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage uintptr
	PagefileUsage, PeakPagefileUsage, PrivateUsage     uintptr
}

// Toolhelp and process counters do not launch a helper or require elevation.
// CPU is normalized to the whole machine, like Task Manager.
func sampleResources(ctx context.Context, appPID int, previous map[int]cpuReading) (ResourceSnapshot, map[int]cpuReading) {
	result := ResourceSnapshot{UpdatedAt: time.Now(), LogicalCPUs: runtime.NumCPU(), Goroutines: runtime.NumGoroutine()}
	readings := map[int]cpuReading{}
	handle, err := syscall.CreateToolhelp32Snapshot(2, 0)
	if err != nil {
		result.Error = err.Error()
		return result, readings
	}
	defer syscall.CloseHandle(handle)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	for err = syscall.Process32First(handle, &entry); err == nil; err = syscall.Process32Next(handle, &entry) {
		if ctx.Err() != nil {
			break
		}
		name := syscall.UTF16ToString(entry.ExeFile[:])
		pid := int(entry.ProcessID)
		external := pid != appPID && int(entry.ParentProcessID) != appPID
		if external && !isRcloneProcess(name) {
			continue
		}
		row := ProcessResource{PID: pid, Name: name, Threads: int(entry.Threads), External: external}
		process, openErr := syscall.OpenProcess(0x1000|0x10, false, entry.ProcessID)
		if openErr != nil {
			process, openErr = syscall.OpenProcess(0x1000, false, entry.ProcessID)
		}
		if openErr != nil {
			row.Error = "需要该进程的读取权限"
			result.Processes = append(result.Processes, row)
			continue
		}
		var created, exited, kernel, user syscall.Filetime
		if syscall.GetProcessTimes(process, &created, &exited, &kernel, &user) == nil {
			reading := cpuReading{Created: filetimeTicks(created), Ticks: filetimeTicks(kernel) + filetimeTicks(user), At: result.UpdatedAt}
			old, ok := previous[pid]
			if ok && old.Created == reading.Created && reading.Ticks >= old.Ticks {
				row.CPUPercent = cpuUsage(reading.Ticks-old.Ticks, reading.At.Sub(old.At), result.LogicalCPUs)
				row.CPUAvailable = true
			}
			readings[pid] = reading
		}
		memory := processMemoryCounters{Size: uint32(unsafe.Sizeof(processMemoryCounters{}))}
		ok, _, _ := getProcessMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(&memory)), uintptr(memory.Size))
		if ok != 0 {
			row.MemoryBytes = int64(memory.PrivateUsage)
			row.WorkingSetBytes = int64(memory.WorkingSetSize)
			row.MemoryAvailable = true
		}
		syscall.CloseHandle(process)
		if !row.MemoryAvailable {
			row.Error = "内存计数不可读"
		}
		result.Processes = append(result.Processes, row)
	}
	return result, readings
}

func filetimeTicks(value syscall.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}
