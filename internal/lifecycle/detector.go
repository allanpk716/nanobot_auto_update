//go:build windows

package lifecycle

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// FindPIDByPort returns the PID of the process listening on the specified port.
// Returns 0 if no process is listening on that port.
func FindPIDByPort(port uint32, logger *slog.Logger) (int32, error) {
	logger.Debug("Checking port for nanobot process", "port", port)

	connections, err := net.Connections("tcp")
	if err != nil {
		logger.Error("Failed to get network connections", "error", err)
		return 0, fmt.Errorf("failed to get network connections: %w", err)
	}

	for _, conn := range connections {
		// Check if connection is listening on the specified port
		if conn.Status == "LISTEN" && conn.Laddr.Port == port {
			logger.Info("Found nanobot by port", "pid", conn.Pid, "port", port)
			return conn.Pid, nil
		}
	}

	logger.Debug("No process found listening on port", "port", port)
	return 0, nil // No process found, not an error
}

// FindPIDByProcessName returns the PID of the process with the specified name.
// Returns 0 if no process with that name is found.
// Deprecated: Not suitable for multi-instance scenarios where multiple instances share the same binary.
// Use PID-based tracking via InstanceLifecycle.IsRunning() instead.
func FindPIDByProcessName(processName string, logger *slog.Logger) (int32, error) {
	logger.Debug("Searching for process by name", "process_name", processName)

	processes, err := process.Processes()
	if err != nil {
		logger.Error("Failed to list processes", "error", err)
		return 0, fmt.Errorf("failed to list processes: %w", err)
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			// Skip processes we can't read
			continue
		}
		// Case-insensitive match for process name
		if strings.EqualFold(name, processName) {
			logger.Info("Found nanobot by process name", "pid", p.Pid, "process_name", name)
			return p.Pid, nil
		}
	}

	logger.Debug("No process found with name", "process_name", processName)
	return 0, nil // No process found, not an error
}

// IsNanobotRunning checks if nanobot is running on the specified port.
// Uses port-based detection only, which is precise for multi-instance scenarios.
// Returns (isRunning, pid, detectionMethod, error).
// detectionMethod is "port" or "" if not running.
func IsNanobotRunning(port uint32) (bool, int32, string, error) {
	logger := slog.Default() // Use default logger for detector

	logger.Info("Detecting nanobot process", "port", port)

	// Check by port (precise - identifies specific instance by its listening port)
	pid, err := FindPIDByPort(port, logger)
	if err != nil {
		return false, 0, "", err
	}
	if pid > 0 {
		return true, pid, "port", nil
	}

	logger.Info("Nanobot not running on port", "port", port)
	return false, 0, "", nil
}

// FindProcessByPID checks if a process with the given PID exists and is running.
// Returns the process object if found, nil otherwise.
// Note: proc.Status() is NOT implemented on Windows (gopsutil issue #1708), so we use
// a simpler check: if process.NewProcess(pid) succeeds, the process exists.
func FindProcessByPID(pid int32, logger *slog.Logger) (*process.Process, error) {
	logger.Debug("Checking if process exists", "pid", pid)

	proc, err := process.NewProcess(pid)
	if err != nil {
		logger.Debug("Process not found", "pid", pid, "error", err)
		return nil, err
	}

	// process.NewProcess succeeds only if the OS can open a handle to the process.
	// On Windows, this means the process exists. Additional liveness check via
	// IsRunning() which works on Windows (unlike Status() which is not implemented).
	running, err := proc.IsRunning()
	if err != nil {
		logger.Debug("Failed to check if process is running", "pid", pid, "error", err)
		return nil, err
	}
	if !running {
		logger.Debug("Process exists but not running", "pid", pid)
		return nil, fmt.Errorf("process %d exists but is not running", pid)
	}

	return proc, nil
}
