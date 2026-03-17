//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// StartNanobot starts nanobot with the specified command in the background with hidden window.
// Returns error if startup fails or port not listening within timeout.
func StartNanobot(ctx context.Context, command string, port uint32, startupTimeout time.Duration, logger *slog.Logger) error {
	logger.Info("Starting nanobot", "command", command, "port", port, "startup_timeout", startupTimeout)

	// Start nanobot as background process using Windows shell
	// cmd /c supports pipes, redirections, and complex commands
	cmd := exec.CommandContext(ctx, "cmd", "/c", command)
	// Set PYTHONIOENCODING=utf-8 to fix Unicode encoding issues on Windows
	// (nanobot uses emoji in output which fails with GBK encoding)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	logger.Debug("Executing command via Windows shell")
	// Detach from parent - don't wait for completion
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start nanobot process", "command", command, "error", err)
		return fmt.Errorf("failed to start nanobot: %w", err)
	}

	logger.Info("Nanobot process started", "pid", cmd.Process.Pid)

	// Release the process so it continues independently
	if err := cmd.Process.Release(); err != nil {
		logger.Warn("Failed to detach nanobot process (non-fatal)", "error", err)
		return fmt.Errorf("failed to detach nanobot process: %w", err)
	}

	logger.Debug("Process detached, waiting for port to become available")

	// Verify startup by checking port is listening
	if err := waitForPortListening(ctx, port, startupTimeout, logger); err != nil {
		logger.Error("Nanobot startup verification failed", "port", port, "error", err)
		return fmt.Errorf("nanobot startup verification failed: %w", err)
	}

	logger.Info("Nanobot startup verified, port is listening", "port", port)
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

// StartNanobotWithCapture starts nanobot with stdout/stderr capture to LogBuffer.
// CAPT-01, CAPT-02: Captures stdout and stderr output streams
// CAPT-03: Concurrent pipe reading using separate goroutines
// CAPT-04: Auto-starts capture on process start
// CAPT-05: Auto-stops capture on process exit via context cancellation
func StartNanobotWithCapture(
	ctx context.Context,
	command string,
	port uint32,
	startupTimeout time.Duration,
	logger *slog.Logger,
	logBuffer *logbuffer.LogBuffer,
) error {
	logger.Info("Starting nanobot with log capture", "command", command, "port", port)

	// Create cancelable context for log capture goroutines (CAPT-05)
	captureCtx, cancelCapture := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Prepare command
	cmd := exec.CommandContext(ctx, "cmd", "/c", command)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	// Create stdout pipe (avoid StdoutPipe() race condition - RESEARCH.md)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		cancelCapture()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Create stderr pipe
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		cancelCapture()
		stdoutReader.Close()
		stdoutWriter.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Set cmd.Stdout and cmd.Stderr (CAPT-01, CAPT-02)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	// Start stdout capture goroutine (CAPT-03)
	wg.Add(1)
	go func() {
		defer wg.Done()
		captureLogs(captureCtx, stdoutReader, "stdout", logBuffer, logger)
		stdoutReader.Close()
	}()

	// Start stderr capture goroutine (CAPT-03)
	wg.Add(1)
	go func() {
		defer wg.Done()
		captureLogs(captureCtx, stderrReader, "stderr", logBuffer, logger)
		stderrReader.Close()
	}()

	// Start process (CAPT-04)
	if err := cmd.Start(); err != nil {
		cancelCapture() // Cancel capture goroutines
		wg.Wait()       // Wait for goroutines to exit
		stdoutReader.Close()
		stdoutWriter.Close()
		stderrReader.Close()
		stderrWriter.Close()
		logger.Error("Failed to start nanobot process", "error", err)
		return fmt.Errorf("failed to start nanobot: %w", err)
	}

	logger.Info("Nanobot process started", "pid", cmd.Process.Pid)

	// Start monitor goroutine: stop capture on process exit (CAPT-05)
	go func() {
		err := cmd.Wait() // Wait for process exit
		if err != nil {
			logger.Warn("Nanobot process exited with error", "pid", cmd.Process.Pid, "error", err)
		} else {
			logger.Info("Nanobot process exited", "pid", cmd.Process.Pid)
		}

		// Close writer ends to trigger EOF in readers
		stdoutWriter.Close()
		stderrWriter.Close()

		// Cancel context to stop capture goroutines
		cancelCapture()
		wg.Wait() // Wait for capture goroutines to exit

		logger.Debug("Log capture goroutines stopped")
	}()

	// Verify startup by checking port is listening
	if err := waitForPortListening(ctx, port, startupTimeout, logger); err != nil {
		logger.Error("Nanobot startup verification failed", "port", port, "error", err)
		return fmt.Errorf("nanobot startup verification failed: %w", err)
	}

	logger.Info("Nanobot startup verified, port is listening", "port", port)
	return nil
}
