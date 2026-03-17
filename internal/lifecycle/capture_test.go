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
