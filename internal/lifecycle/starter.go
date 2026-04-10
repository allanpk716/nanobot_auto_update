//go:build windows

package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// StartNanobotWithCapture starts nanobot with log capture.
// Returns the process ID on success.
func StartNanobotWithCapture(
	ctx context.Context,
	command string,
	port uint32,
	startupTimeout time.Duration,
	logger *slog.Logger,
	logBuffer *logbuffer.LogBuffer,
) (int, error) {
	// Auto-append --port parameter if not already present in command
	// This ensures nanobot uses the configured port without requiring manual configuration
	finalCommand := command
	if !containsPortFlag(command) {
		finalCommand = fmt.Sprintf("%s --port %d", command, port)
		logger.Debug("Auto-appending port parameter to command", "original", command, "final", finalCommand)
	}

	// Parse command into executable and arguments
	parts := splitCommand(finalCommand)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty command")
	}

	executable := parts[0]
	args := parts[1:]

	logger.Info("Starting nanobot", "command", finalCommand, "executable", executable, "args", strings.Join(args, " "), "port", port)

	// Create a detached context for the process
	// CRITICAL: We use context.Background() instead of the passed context to avoid
	// killing the process when the parent context is cancelled. The process lifetime
	// should be independent of the startup context.
	// The passed ctx is only used for startup timeout control in the caller.
	detachedCtx := context.Background()

	// Create pipes for stdout and stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		return 0, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Prepare command with detached context
	cmd := exec.CommandContext(detachedCtx, executable, args...)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.SysProcAttr = &windows.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}

	// Set stdout and stderr
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	// Start process
	if err := cmd.Start(); err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		stderrReader.Close()
		stderrWriter.Close()
		logger.Error("Failed to start nanobot", "error", err)
		return 0, fmt.Errorf("failed to start nanobot: %w", err)
	}

	pid := cmd.Process.Pid
	logger.Info("Nanobot process started", "pid", pid)

	// Close writer ends immediately - the subprocess has already inherited them
	// This is critical to prevent pipe deadlock when buffer fills up
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait 2 seconds for process stabilization
	time.Sleep(2 * time.Second)

	// Verify process is still running
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		_ = cmd.Wait()
		stdoutReader.Close()
		stderrReader.Close()
		logger.Error("Process exited immediately after start", "pid", pid)
		return 0, fmt.Errorf("process exited immediately after start (PID %d)", pid)
	}

	name, err := proc.Name()
	if err != nil {
		_ = cmd.Wait()
		stdoutReader.Close()
		stderrReader.Close()
		logger.Error("Failed to verify process name", "pid", pid, "error", err)
		return 0, fmt.Errorf("failed to verify process name (PID %d): %w", pid, err)
	}

	logger.Info("Nanobot process verified", "pid", pid, "process_name", name)

	// Start log capture goroutines
	// Use detachedCtx to ensure log capture continues even if parent context is cancelled
	go captureLogs(detachedCtx, stdoutReader, "stdout", logBuffer, logger)
	go captureLogs(detachedCtx, stderrReader, "stderr", logBuffer, logger)

	// Start monitor goroutine to handle process exit
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warn("Process exited with error", "pid", pid, "error", err)
		} else {
			logger.Info("Process exited normally", "pid", pid)
		}
	}()

	return pid, nil
}

// captureLogs reads from a reader and writes to LogBuffer
func captureLogs(ctx context.Context, reader io.Reader, source string, logBuffer *logbuffer.LogBuffer,
	logger *slog.Logger,
) {
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			closer.Close()
		}
	}()
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					logger.Debug("Log capture stopped", "source", source, "error", err)
				}
				return
			}
			if n > 0 {
				logBuffer.Write(logbuffer.LogEntry{
					Timestamp: time.Now(),
					Source:    source,
					Content:   string(buf[:n]),
				})
			}
		}
	}
}

// splitCommand splits a command string into parts, handling quoted arguments
func splitCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(cmd); i++ {
		char := cmd[i]

		if char == '"' {
			inQuotes = !inQuotes
			current.WriteByte(char)
		} else if char == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	// Remove quotes from each part
	for i, part := range parts {
		if len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' {
			parts[i] = part[1 : len(part)-1]
		}
	}

	return parts
}

// containsPortFlag checks if the command already contains a --port flag.
// This prevents duplicate port parameters when the command already includes one.
func containsPortFlag(command string) bool {
	// Simple check for --port flag presence
	// Handles both "--port 12345" and "--port=12345" formats
	return strings.Contains(command, " --port ") ||
		strings.Contains(command, " --port=") ||
		strings.HasSuffix(command, " --port")
}
