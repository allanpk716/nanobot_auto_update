//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
)

// StartNanobot starts nanobot gateway in the background with hidden window.
// Returns error if startup fails or process not running within timeout.
func StartNanobot(ctx context.Context, startupTimeout time.Duration, logger *slog.Logger) error {
	logger.Info("Starting nanobot gateway", "startup_timeout", startupTimeout)

	// Start nanobot gateway as background process
	cmd := exec.Command("nanobot", "gateway")
	// Set PYTHONIOENCODING=utf-8 to fix Unicode encoding issues on Windows
	// (nanobot uses emoji in output which fails with GBK encoding)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	logger.Debug("Executing nanobot gateway command")
	// Detach from parent - don't wait for completion
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start nanobot process", "error", err)
		return fmt.Errorf("failed to start nanobot: %w", err)
	}

	logger.Info("Nanobot process started", "pid", cmd.Process.Pid)

	// Release the process so it continues independently
	if err := cmd.Process.Release(); err != nil {
		logger.Warn("Failed to detach nanobot process (non-fatal)", "error", err)
		return fmt.Errorf("failed to detach nanobot process: %w", err)
	}

	logger.Debug("Process detached, waiting for process to start running")

	// Verify startup by checking process is running
	if err := waitForProcessRunning(ctx, "nanobot.exe", startupTimeout, logger); err != nil {
		logger.Error("Nanobot startup verification failed", "error", err)
		return fmt.Errorf("nanobot startup verification failed: %w", err)
	}

	logger.Info("Nanobot startup verified, process is running")
	return nil
}

// waitForProcessRunning polls until the process is running or timeout
func waitForProcessRunning(ctx context.Context, processName string, timeout time.Duration, logger *slog.Logger) error {
	deadline := time.Now().Add(timeout)
	attempts := 0

	logger.Debug("Waiting for process to start running", "process", processName, "timeout", timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			logger.Warn("Context cancelled while waiting for process", "process", processName)
			return ctx.Err()
		default:
			attempts++
			pid, err := FindPIDByProcessName(processName, logger)
			if err == nil && pid > 0 {
				logger.Info("Process started successfully", "process", processName, "pid", pid, "attempts", attempts)
				return nil
			}
			if attempts%4 == 0 {
				// Log every 2 seconds (4 attempts * 500ms)
				logger.Debug("Process not yet running, retrying", "process", processName, "attempt", attempts)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Error("Process not running after timeout", "process", processName, "attempts", attempts)
	return fmt.Errorf("process %s not running after %v", processName, timeout)
}

// waitForPortListening polls until the port is listening or timeout
func waitForPortListening(ctx context.Context, port uint32, timeout time.Duration, logger *slog.Logger) error {
	deadline := time.Now().Add(timeout)
	address := fmt.Sprintf("127.0.0.1:%d", port)

	logger.Debug("Waiting for port to become available", "address", address, "timeout", timeout)

	attempts := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			logger.Warn("Context cancelled while waiting for port", "port", port)
			return ctx.Err()
		default:
			attempts++
			// Try to connect to verify port is listening
			conn, err := net.DialTimeout("tcp", address, 1*time.Second)
			if err == nil {
				conn.Close()
				logger.Debug("Port is now listening", "port", port, "attempts", attempts)
				return nil // Port is listening
			}
			if attempts%4 == 0 {
				// Log every 2 seconds (4 attempts * 500ms)
				logger.Debug("Port not yet available, retrying", "port", port, "attempt", attempts)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Error("Port not listening after timeout", "port", port, "attempts", attempts)
	return fmt.Errorf("port %d not listening after %v", port, timeout)
}
