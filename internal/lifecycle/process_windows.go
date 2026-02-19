//go:build windows

package lifecycle

import (
	"os"
	"syscall"
	"unsafe"
)

// getProcessExeName gets the executable name for a given PID
func getProcessExeName(pid int) (string, error) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(snapshot)

	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	// Get first process
	err = syscall.Process32First(snapshot, &entry)
	if err != nil {
		return "", err
	}

	// Iterate through processes to find our target PID
	for {
		if entry.ProcessID == uint32(pid) {
			return syscall.UTF16ToString(entry.ExeFile[:]), nil
		}

		err = syscall.Process32Next(snapshot, &entry)
		if err != nil {
			break // No more processes
		}
	}

	return "", syscall.ERROR_NOT_FOUND
}

// isParentNanobot checks if parent process is nanobot
func isParentNanobot() (bool, error) {
	ppid := os.Getppid()
	if ppid <= 1 {
		return false, nil // No parent or init process
	}

	exeName, err := getProcessExeName(ppid)
	if err != nil {
		return false, err
	}

	return exeName == "nanobot.exe", nil
}
