package logbuffer

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// TestNewLogBuffer tests LogBuffer initialization
func TestNewLogBuffer(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	if lb == nil {
		t.Fatal("NewLogBuffer returned nil")
	}

	// Verify initial size is 0
	history := lb.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected initial size 0, got %d", len(history))
	}
}

// TestLogBuffer_Write tests single log entry write and read
func TestLogBuffer_Write(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	entry := LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "test log",
	}

	err := lb.Write(entry)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	history := lb.GetHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(history))
	}

	if history[0].Content != "test log" {
		t.Errorf("Expected content 'test log', got '%s'", history[0].Content)
	}

	if history[0].Source != "stdout" {
		t.Errorf("Expected source 'stdout', got '%s'", history[0].Source)
	}
}

// TestLogBuffer_FIFO tests FIFO overwrite when buffer is full
func TestLogBuffer_FIFO(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Write 5001 entries
	for i := 1; i <= 5001; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed at entry %d: %v", i, err)
		}
	}

	history := lb.GetHistory()

	// Verify buffer size is 5000
	if len(history) != 5000 {
		t.Fatalf("Expected 5000 entries, got %d", len(history))
	}

	// First entry should be "log-2" (log-1 was overwritten)
	if history[0].Content != "log-2" {
		t.Errorf("Expected first entry 'log-2', got '%s'", history[0].Content)
	}

	// Last entry should be "log-5001"
	if history[4999].Content != "log-5001" {
		t.Errorf("Expected last entry 'log-5001', got '%s'", history[4999].Content)
	}
}

// TestLogBuffer_Concurrent tests concurrent writes with race detection
func TestLogBuffer_Concurrent(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	var wg sync.WaitGroup
	numGoroutines := 10
	entriesPerGoroutine := 100

	// Start 10 goroutines writing concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := LogEntry{
					Timestamp: time.Now(),
					Source:    "stdout",
					Content:   "concurrent-log",
				}
				err := lb.Write(entry)
				if err != nil {
					t.Errorf("Write failed in goroutine %d: %v", goroutineID, err)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify total entries (may be less due to FIFO overwrite)
	history := lb.GetHistory()
	if len(history) != 1000 {
		t.Errorf("Expected 1000 entries, got %d", len(history))
	}
}

// TestLogEntry_Fields tests LogEntry field types
func TestLogEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now,
		Source:    "stderr",
		Content:   "test content",
	}

	// Verify Timestamp field
	if !entry.Timestamp.Equal(now) {
		t.Errorf("Expected Timestamp %v, got %v", now, entry.Timestamp)
	}

	// Verify Source field
	if entry.Source != "stderr" {
		t.Errorf("Expected Source 'stderr', got '%s'", entry.Source)
	}

	// Verify Content field
	if entry.Content != "test content" {
		t.Errorf("Expected Content 'test content', got '%s'", entry.Content)
	}
}

// Helper function to create test logger
func createTestLogger() *slog.Logger {
	// Create a simple discard logger for testing
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Helper function to convert int to string
func toString(i int) string {
	return fmt.Sprintf("%d", i)
}
