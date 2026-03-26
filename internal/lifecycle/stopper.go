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

// StopAllNanobots kills all nanobot.exe processes on the system.
// This is used during auto-start to ensure a clean slate before starting instances.
// Returns the number of processes killed and any error that occurred.
func StopAllNanobots(ctx context.Context, timeout time.Duration, logger *slog.Logger) (int, error) {
	logger.Info("正在停止所有 nanobot.exe 进程")

	// Find all nanobot.exe processes
	processes, err := findNanobotProcesses(logger)
	if err != nil {
		logger.Error("查找 nanobot 进程失败", "error", err)
		return 0, fmt.Errorf("failed to find nanobot processes: %w", err)
	}

	if len(processes) == 0 {
		logger.Info("没有找到运行中的 nanobot 进程")
		return 0, nil
	}

	logger.Info("找到 nanobot 进程", "count", len(processes), "pids", processes)

	// Kill all processes
	killedCount := 0
	for _, pid := range processes {
		if err := StopNanobot(ctx, pid, timeout, logger); err != nil {
			logger.Warn("停止进程失败，继续尝试其他进程", "pid", pid, "error", err)
			// Continue killing other processes even if one fails
		} else {
			killedCount++
		}
	}

	logger.Info("停止进程完成", "killed", killedCount, "total", len(processes))
	return killedCount, nil
}

// findNanobotProcesses finds all nanobot.exe processes using tasklist
func findNanobotProcesses(logger *slog.Logger) ([]int32, error) {
	// Use tasklist to find all nanobot.exe processes
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq nanobot.exe", "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tasklist command failed: %w", err)
	}

	// Parse CSV output
	// Format: "nanobot.exe","1234","Console","1","4,752 K"
	var pids []int32
	lines := splitLines(string(output))
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse CSV line
		fields := parseCSVLine(line)
		if len(fields) < 2 {
			continue
		}

		// Check if it's actually nanobot.exe
		if fields[0] != "nanobot.exe" {
			continue
		}

		// Parse PID
		var pid int32
		if _, err := fmt.Sscanf(fields[1], "%d", &pid); err != nil {
			logger.Debug("解析 PID 失败", "line", line, "error", err)
			continue
		}

		if pid > 0 {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// parseCSVLine parses a CSV line with quoted fields
func parseCSVLine(line string) []string {
	var fields []string
	inQuote := false
	start := 0

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			inQuote = !inQuote
		} else if ch == ',' && !inQuote {
			field := line[start:i]
			// Remove surrounding quotes
			if len(field) >= 2 && field[0] == '"' && field[len(field)-1] == '"' {
				field = field[1 : len(field)-1]
			}
			fields = append(fields, field)
			start = i + 1
		}
	}

	// Add last field
	if start < len(line) {
		field := line[start:]
		if len(field) >= 2 && field[0] == '"' && field[len(field)-1] == '"' {
			field = field[1 : len(field)-1]
		}
		fields = append(fields, field)
	}

	return fields
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
