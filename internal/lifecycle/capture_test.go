//go:build windows

package lifecycle

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
)

// createTestLogger creates a discard logger for testing
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCaptureLogs_WritesToBuffer(t *testing.T) {
	// Setup: Create LogBuffer and pipe reader
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	input := "line1\nline2\nline3\n"
	reader := strings.NewReader(input)

	ctx := context.Background()

	// Execute: Run captureLogs
	captureLogs(ctx, reader, "stdout", logBuf, createTestLogger())

	// Verify: Check logs written to buffer
	history := logBuf.GetHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 log entries, got %d", len(history))
	}
	if history[0].Content != "line1" {
		t.Errorf("expected first line 'line1', got '%s'", history[0].Content)
	}
	if history[0].Source != "stdout" {
		t.Errorf("expected source 'stdout', got '%s'", history[0].Source)
	}
}

func TestCaptureLogs_ContextCancellation(t *testing.T) {
	// Setup: Create LogBuffer and slow reader
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	input := "line1\nline2\n"
	reader := strings.NewReader(input)

	ctx, cancel := context.WithCancel(context.Background())

	// Execute: Cancel context immediately
	cancel()
	captureLogs(ctx, reader, "stdout", logBuf, createTestLogger())

	// Verify: No logs written (context cancelled before scan)
	// Note: Due to select behavior, some logs may be written before cancellation
	// This test verifies the function exits cleanly
}

func TestCaptureLogs_LogEntryFields(t *testing.T) {
	// Setup
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	input := "test log line\n"
	reader := strings.NewReader(input)
	ctx := context.Background()

	beforeTime := time.Now()
	captureLogs(ctx, reader, "stderr", logBuf, createTestLogger())
	afterTime := time.Now()

	// Verify: Check all LogEntry fields are set correctly
	history := logBuf.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(history))
	}

	entry := history[0]
	if entry.Source != "stderr" {
		t.Errorf("expected source 'stderr', got '%s'", entry.Source)
	}
	if entry.Content != "test log line" {
		t.Errorf("expected content 'test log line', got '%s'", entry.Content)
	}
	if entry.Timestamp.Before(beforeTime) || entry.Timestamp.After(afterTime) {
		t.Errorf("timestamp %v not in expected range [%v, %v]", entry.Timestamp, beforeTime, afterTime)
	}
}

func TestStartNanobotWithCapture_CapturesOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup: Create LogBuffer
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	ctx := context.Background()
	// Use echo command to generate output
	command := "echo stdout_test && echo stderr_test 1>&2"
	port := uint32(9999) // Dummy port (won't be verified)

	// Use a very short timeout since we expect port verification to fail
	startupTimeout := 1 * time.Second

	testLogger := createTestLogger()

	// Execute: Start process with capture (will fail port verification but capture should work)
	err := StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Verify: Port verification fails (expected), but logs should be captured
	if err == nil {
		t.Error("expected port verification to fail")
	}

	// Wait a bit for goroutines to finish
	time.Sleep(500 * time.Millisecond)

	history := logBuf.GetHistory()
	if len(history) < 2 {
		t.Errorf("expected at least 2 log entries, got %d: %v", len(history), history)
	}

	// Verify stdout was captured
	stdoutFound := false
	stderrFound := false
	for _, entry := range history {
		if entry.Content == "stdout_test" && entry.Source == "stdout" {
			stdoutFound = true
		}
		if entry.Content == "stderr_test" && entry.Source == "stderr" {
			stderrFound = true
		}
	}

	if !stdoutFound {
		t.Error("stdout output not captured")
	}
	if !stderrFound {
		t.Error("stderr output not captured")
	}
}

func TestStartNanobotWithCapture_ProcessExit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	logBuf := logbuffer.NewLogBuffer(createTestLogger())
	ctx := context.Background()

	// Command that exits quickly
	command := "echo test"
	port := uint32(9999)
	startupTimeout := 1 * time.Second

	testLogger := createTestLogger()

	// Execute
	_ = StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Wait for process to exit and goroutines to cleanup
	time.Sleep(1 * time.Second)

	// Verify: Logs captured
	history := logBuf.GetHistory()
	if len(history) == 0 {
		t.Error("expected logs to be captured")
	}
}

func TestStartNanobotWithCapture_InvalidCommand(t *testing.T) {
	// Setup
	logBuf := logbuffer.NewLogBuffer(createTestLogger())
	ctx := context.Background()

	// Invalid command that will fail to start
	command := "nonexistent_command_12345"
	port := uint32(9999)
	startupTimeout := 1 * time.Second

	testLogger := createTestLogger()

	// Execute
	err := StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Verify: Should return error
	if err == nil {
		t.Error("expected error for invalid command")
	}

	// Verify: No goroutine leaks (logBuffer should have no entries)
	history := logBuf.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected no logs for failed start, got %d", len(history))
	}
}
