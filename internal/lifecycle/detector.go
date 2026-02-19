//go:build windows

package lifecycle

import (
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// FindPIDByPort returns the PID of the process listening on the specified port.
// Returns 0 if no process is listening on that port.
func FindPIDByPort(port uint32) (int32, error) {
	connections, err := net.Connections("tcp")
	if err != nil {
		return 0, fmt.Errorf("failed to get network connections: %w", err)
	}

	for _, conn := range connections {
		// Check if connection is listening on the specified port
		if conn.Status == "LISTEN" && conn.Laddr.Port == port {
			return conn.Pid, nil
		}
	}

	return 0, nil // No process found, not an error
}

// FindPIDByProcessName returns the PID of the process with the specified name.
// Returns 0 if no process with that name is found.
func FindPIDByProcessName(processName string) (int32, error) {
	processes, err := process.Processes()
	if err != nil {
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
			return p.Pid, nil
		}
	}

	return 0, nil // No process found, not an error
}

// IsNanobotRunning checks if nanobot is running on the specified port.
// Falls back to process name detection if port check finds nothing.
// Returns (isRunning, pid, error).
func IsNanobotRunning(port uint32) (bool, int32, error) {
	// Primary: Check by port
	pid, err := FindPIDByPort(port)
	if err != nil {
		return false, 0, err
	}
	if pid > 0 {
		return true, pid, nil
	}

	// Fallback: Check by process name (handles case where nanobot is running but not listening on port)
	pid, err = FindPIDByProcessName("nanobot.exe")
	if err != nil {
		return false, 0, err
	}

	return pid > 0, pid, nil
}
