//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// StopNanobot gracefully stops nanobot process, force-killing after timeout.
// timeout is the maximum time to wait for graceful shutdown before force kill.
// Returns error if stop fails completely.
func StopNanobot(ctx context.Context, pid int32, timeout time.Duration, logger *slog.Logger) error {
	if pid <= 0 {
		logger.Debug("No PID provided, nothing to stop")
		return nil // Nothing to stop
	}

	logger.Info("Stopping nanobot", "pid", pid, "timeout", timeout)

	// Create timeout context for the entire stop operation
	stopCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Step 1: Try graceful termination (taskkill without /f)
	// This sends WM_CLOSE message to the process
	logger.Info("Attempting graceful termination", "pid", pid)
	gracefulCmd := exec.CommandContext(stopCtx, "taskkill", "/PID", fmt.Sprintf("%d", pid))
	gracefulCmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	err := gracefulCmd.Run()
	if err == nil {
		// Graceful termination succeeded, wait for process to exit
		logger.Debug("Graceful termination command sent, waiting for process exit", "pid", pid)
		if waitForProcessExit(stopCtx, pid, 2*time.Second, logger) {
			logger.Info("Nanobot stopped gracefully", "pid", pid)
			return nil
		}
		logger.Warn("Graceful termination timed out, proceeding to force kill", "pid", pid)
	} else {
		logger.Warn("Graceful termination failed", "pid", pid, "error", err)
	}

	// Step 2: Force kill (taskkill /f)
	logger.Info("Attempting force kill", "pid", pid)
	forceCmd := exec.CommandContext(stopCtx, "taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	forceCmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	if err := forceCmd.Run(); err != nil {
		logger.Error("Force kill command failed", "pid", pid, "error", err)
		return fmt.Errorf("force kill failed: %w", err)
	}

	// Verify process is gone
	logger.Debug("Verifying process termination", "pid", pid)
	if !waitForProcessExit(stopCtx, pid, 1*time.Second, logger) {
		logger.Error("Process did not terminate after force kill", "pid", pid)
		return fmt.Errorf("process %d did not terminate after force kill", pid)
	}

	logger.Info("Nanobot stopped (force killed)", "pid", pid)
	return nil
}

// waitForProcessExit polls until the process exits or context is done
func waitForProcessExit(ctx context.Context, pid int32, pollInterval time.Duration, logger *slog.Logger) bool {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Wait for process exit timed out", "pid", pid)
			return false
		case <-ticker.C:
			// Check if process still exists
			process, err := os.FindProcess(int(pid))
			if err != nil {
				logger.Debug("Process not found (exited)", "pid", pid)
				return true // Process doesn't exist
			}
			// On Windows, FindProcess always succeeds, so we try to signal
			err = process.Signal(windows.Signal(0))
			if err != nil {
				logger.Debug("Process has exited", "pid", pid)
				return true // Process has exited
			}
		}
	}
}
