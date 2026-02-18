//go:build windows

package lifecycle

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/net"
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

// IsNanobotRunning checks if nanobot is running on the specified port.
// Returns (isRunning, pid, error).
func IsNanobotRunning(port uint32) (bool, int32, error) {
	pid, err := FindPIDByPort(port)
	if err != nil {
		return false, 0, err
	}
	return pid > 0, pid, nil
}
