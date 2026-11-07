//go:build windows

package core

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

func processIdentity(pid int) string {
	handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(handle)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return ""
	}
	return strconv.FormatUint(filetimeTicks(created), 10)
}

const createNoWindow = 0x08000000
const createNewProcessGroup = 0x00000200

func hideProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow | createNewProcessGroup,
	}
}

// A windowsgui executable stays silent for GUI launches, but CLI commands
// attach to the caller's console. Existing redirected streams are preserved.
func AttachCLIConsole() {
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	out, errOut := os.Stdout, os.Stderr
	_, _, _ = attach.Call(^uintptr(0))
	if _, err := out.Stat(); err != nil {
		if handle, e := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE); e == nil {
			os.Stdout = os.NewFile(uintptr(handle), "stdout")
		}
	}
	if _, err := errOut.Stat(); err != nil {
		if handle, e := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE); e == nil {
			os.Stderr = os.NewFile(uintptr(handle), "stderr")
		}
	}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		// An access-denied process still exists; never bypass its ownership.
		return err == syscall.ERROR_ACCESS_DENIED
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if syscall.GetExitCodeProcess(handle, &code) != nil {
		return false
	}
	return code == 259
}

func AcquireInstance(path string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return func() { syscall.CloseHandle(handle) }, nil
}

func FocusExistingWindow() bool {
	dll := syscall.NewLazyDLL("user32.dll")
	title, _ := syscall.UTF16PtrFromString("RcloneGo")
	handle, _, _ := dll.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(title)))
	if handle == 0 {
		return false
	}
	dll.NewProc("ShowWindow").Call(handle, 9)
	dll.NewProc("SetForegroundWindow").Call(handle)
	return true
}
