//go:build !windows

package lifecycle

// IsServiceMode detects whether the current process is running as a Windows service.
// Always returns false on non-Windows platforms (service mode is Windows-only).
func IsServiceMode() (bool, error) {
	return false, nil
}
