//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// StopNanobot gracefully stops nanobot process, force-killing after timeout.
// timeout is the maximum time to wait for graceful shutdown before force kill.
// Returns error if stop fails completely.
func StopNanobot(ctx context.Context, pid int32, timeout time.Duration) error {
	if pid <= 0 {
		return nil // Nothing to stop
	}

	// Create timeout context for the entire stop operation
	stopCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Step 1: Try graceful termination (taskkill without /f)
	// This sends WM_CLOSE message to the process
	gracefulCmd := exec.CommandContext(stopCtx, "taskkill", "/PID", fmt.Sprintf("%d", pid))
	gracefulCmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	err := gracefulCmd.Run()
	if err == nil {
		// Graceful termination succeeded, wait for process to exit
		if waitForProcessExit(stopCtx, pid, 2*time.Second) {
			return nil
		}
	}

	// Step 2: Force kill (taskkill /f)
	forceCmd := exec.CommandContext(stopCtx, "taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	forceCmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	if err := forceCmd.Run(); err != nil {
		return fmt.Errorf("force kill failed: %w", err)
	}

	// Verify process is gone
	if !waitForProcessExit(stopCtx, pid, 1*time.Second) {
		return fmt.Errorf("process %d did not terminate after force kill", pid)
	}

	return nil
}

// waitForProcessExit polls until the process exits or context is done
func waitForProcessExit(ctx context.Context, pid int32, pollInterval time.Duration) bool {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// Check if process still exists
			process, err := os.FindProcess(int(pid))
			if err != nil {
				return true // Process doesn't exist
			}
			// On Windows, FindProcess always succeeds, so we try to signal
			err = process.Signal(windows.Signal(0))
			if err != nil {
				return true // Process has exited
			}
		}
	}
}
