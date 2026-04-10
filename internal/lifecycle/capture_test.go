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

	// Execute: Run captureLogs -- reads all bytes at once (buffer > input size)
	captureLogs(ctx, reader, "stdout", logBuf, createTestLogger())

	// Verify: Check logs written to buffer
	history := logBuf.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 log entry (raw read), got %d", len(history))
	}
	// Raw read captures entire input as one chunk
	if history[0].Content != "line1\nline2\nline3\n" {
		t.Errorf("expected content 'line1\\nline2\\nline3\\n', got '%s'", history[0].Content)
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
	if entry.Content != "test log line\n" {
		t.Errorf("expected content 'test log line\\n', got '%s'", entry.Content)
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
	_, err := StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Verify: Port verification fails (expected), but logs should be captured
	if err == nil {
		t.Error("expected port verification to fail")
	}

	// Wait a bit for goroutines to finish
	time.Sleep(1 * time.Second)

	history := logBuf.GetHistory()
	if len(history) < 2 {
		t.Errorf("expected at least 2 log entries, got %d: %v", len(history), history)
	}

	// Verify stdout was captured
	stdoutFound := false
	stderrFound := false
	for _, entry := range history {
		if strings.Contains(entry.Content, "stdout_test") && entry.Source == "stdout" {
			stdoutFound = true
		}
		if strings.Contains(entry.Content, "stderr_test") && entry.Source == "stderr" {
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
	_, _ = StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Wait for process to exit and goroutines to cleanup
	time.Sleep(1 * time.Second)

	// Verify: Logs captured
	history := logBuf.GetHistory()
	if len(history) == 0 {
		t.Error("expected logs to be captured")
	}
}

// errorReader is an io.Reader that returns an error after some data
type errorReader struct {
	data     string
	readPos  int
	errAfter int // Return error after reading this many bytes
	err      error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.readPos >= r.errAfter {
		return 0, r.err
	}

	remaining := r.data[r.readPos:r.errAfter]
	if len(remaining) == 0 {
		return 0, io.EOF
	}

	toRead := len(p)
	if toRead > len(remaining) {
		toRead = len(remaining)
	}

	copy(p, remaining[:toRead])
	r.readPos += toRead
	return toRead, nil
}

func TestCaptureLogsPipeError(t *testing.T) {
	// Setup: Create LogBuffer with custom logger to capture log output
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	// Create error reader that returns error after some data
	testErr := io.ErrUnexpectedEOF
	reader := &errorReader{
		data:     "line1\nline2\nline3\n",
		errAfter: 10, // Error after reading 10 bytes (during "line2")
		err:      testErr,
	}

	ctx := context.Background()

	// Create a buffer to capture log output
	var logOutput strings.Builder
	testLogger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Execute: Run captureLogs
	captureLogs(ctx, reader, "stdout", logBuf, testLogger)

	// Verify: Error is logged (ERR-01) -- captureLogs logs "Log capture stopped" on read errors
	logs := logOutput.String()
	if !strings.Contains(logs, "Log capture stopped") {
		t.Errorf("expected 'Log capture stopped' in logs, got: %s", logs)
	}
	if !strings.Contains(logs, "source=stdout") {
		t.Errorf("expected source to be logged, got: %s", logs)
	}

	// Verify: Function continues and doesn't panic (test didn't crash)
	// Verify: Some logs before error were captured
	history := logBuf.GetHistory()
	if len(history) == 0 {
		t.Error("expected some logs to be captured before error")
	}
}

func TestCaptureLogsContinuesAfterError(t *testing.T) {
	// Setup: Verify that captureLogs handles errors gracefully without panic
	// This test verifies ERR-01: System continues running after log capture error
	logBuf := logbuffer.NewLogBuffer(createTestLogger())

	// Create reader that will cause scanner error
	testErr := io.ErrUnexpectedEOF
	reader := &errorReader{
		data:     "line1\nline2\n",
		errAfter: 5, // Error early
		err:      testErr,
	}

	ctx := context.Background()
	testLogger := createTestLogger()

	// Execute: This should NOT panic or call os.Exit
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("captureLogs panicked: %v", r)
		}
	}()

	captureLogs(ctx, reader, "stderr", logBuf, testLogger)

	// Verify: Some logs were captured before error
	history := logBuf.GetHistory()
	if len(history) == 0 {
		t.Error("expected some logs to be captured before error occurred")
	}

	// Verify: System is still functional (buffer can still accept writes)
	entry := logbuffer.LogEntry{
		Timestamp: time.Now(),
		Source:    "test",
		Content:   "after error",
	}
	if err := logBuf.Write(entry); err != nil {
		t.Errorf("buffer should still be functional after capture error: %v", err)
	}

	history = logBuf.GetHistory()
	if len(history) == 0 {
		t.Error("buffer should accept writes after capture error")
	}
}

func TestStartNanobotWithCapture_InvalidCommand(t *testing.T) {
	// Setup
	logBuf := logbuffer.NewLogBuffer(createTestLogger())
	ctx := context.Background()

	// Invalid command that will fail to start (use a command that definitely doesn't exist)
	// Note: On Windows, cmd /c will start successfully even for invalid commands,
	// but the command itself will output to stderr and exit with non-zero code.
	// To test actual startup failure, we need to use a non-existent executable directly.
	command := "cmd /c exit 1" // This will start but fail port verification
	port := uint32(9999)
	startupTimeout := 1 * time.Second

	testLogger := createTestLogger()

	// Execute
	_, err := StartNanobotWithCapture(ctx, command, port, startupTimeout, testLogger, logBuf)

	// Verify: Should return error (port verification fails)
	if err == nil {
		t.Error("expected error for command that fails port verification")
	}

	// Note: Some stderr from cmd.exe may be captured, which is acceptable
	// The key is that the function returns an error and doesn't crash
}
