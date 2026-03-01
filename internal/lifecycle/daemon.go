//go:build windows

package lifecycle

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

// MakeDaemon restarts the current process as an independent daemon
// if it detects it's being called from nanobot parent process.
// Returns true if daemon restart was performed, false if already daemon.
func MakeDaemon() (bool, error) {
	// 1. Check if already daemonized (via environment variable)
	if os.Getenv("NANOBOT_UPDATER_DAEMON") == "1" {
		return false, nil // Already running as daemon
	}

	// 2. Check if parent process is nanobot
	isFromNanobot, err := isParentNanobot()
	if err != nil {
		return false, fmt.Errorf("failed to check parent process: %w", err)
	}

	// 3. If not from nanobot, no need to daemonize
	if !isFromNanobot {
		return false, nil
	}

	// 4. Restart self as independent process
	return restartAsDaemon()
}

// MakeDaemonSimple always restarts as daemon without parent check
// This is a simpler alternative when parent detection is not reliable
func MakeDaemonSimple() (bool, error) {
	// Check if already daemonized
	if os.Getenv("NANOBOT_UPDATER_DAEMON") == "1" {
		return false, nil
	}

	return restartAsDaemon()
}

// restartAsDaemon restarts current process as independent daemon
func restartAsDaemon() (bool, error) {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare command line arguments (preserve all flags)
	args := os.Args[1:]

	// Create new independent process
	cmd := exec.Command(exePath, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}

	// Set environment variable to mark as daemon
	cmd.Env = append(os.Environ(), "NANOBOT_UPDATER_DAEMON=1")

	// Redirect stdio to log file for debugging daemon process
	// This ensures we capture any errors during daemon execution
	logFile, err := os.OpenFile("./logs/daemon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to create daemon log file: %w", err)
	}
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start detached process
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Release process handle so it runs independently
	if err := cmd.Process.Release(); err != nil {
		return false, fmt.Errorf("failed to release process handle: %w", err)
	}

	// Exit current process (parent will clean up)
	os.Exit(0)
	return true, nil
}
