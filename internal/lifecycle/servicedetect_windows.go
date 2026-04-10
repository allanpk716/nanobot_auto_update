//go:build windows

package lifecycle

import "golang.org/x/sys/windows/svc"

// IsServiceMode detects whether the current process is running as a Windows service.
// Returns true if running under Service Control Manager (SCM).
// This should be called early in main() before loading configuration (D-06).
func IsServiceMode() (bool, error) {
	return svc.IsWindowsService()
}
