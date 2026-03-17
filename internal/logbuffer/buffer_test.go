package logbuffer

import (
	"fmt"
	"io"
	"log/slog"
	"runtime"
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

// TestLogBuffer_Subscribe tests Subscribe method returns read-only channel and starts goroutine
func TestLogBuffer_Subscribe(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Get initial goroutine count
	initialGoroutines := countGoroutines()

	// Subscribe should return a read-only channel
	ch := lb.Subscribe()

	// Verify channel type is read-only LogEntry channel
	_, ok := interface{}(ch).(<-chan LogEntry)
	if !ok {
		t.Error("Subscribe should return <-chan LogEntry")
	}

	// Verify goroutine was started
	time.Sleep(10 * time.Millisecond) // Wait for goroutine to start
	currentGoroutines := countGoroutines()
	if currentGoroutines <= initialGoroutines {
		t.Errorf("Expected goroutine count to increase, got initial=%d, current=%d", initialGoroutines, currentGoroutines)
	}
}

// TestLogBuffer_History tests new subscriber receives all buffered history logs
func TestLogBuffer_History(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Write 10 logs to buffer
	for i := 1; i <= 10; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Subscribe and receive 10 history logs
	ch := lb.Subscribe()
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < 10 {
		select {
		case entry := <-ch:
			receivedCount++
			expectedContent := "log-" + toString(receivedCount)
			if entry.Content != expectedContent {
				t.Errorf("Expected content '%s', got '%s'", expectedContent, entry.Content)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for history logs, received %d/10", receivedCount)
		}
	}

	// Verify all 10 history logs received
	if receivedCount != 10 {
		t.Errorf("Expected 10 history logs, received %d", receivedCount)
	}
}

// TestLogBuffer_RealTime tests subscriber receives real-time logs after subscription
func TestLogBuffer_RealTime(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Subscribe first
	ch := lb.Subscribe()

	// Wait for history logs (empty buffer, should complete immediately)
	time.Sleep(50 * time.Millisecond)

	// Write 5 logs after subscription
	for i := 1; i <= 5; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "realtime-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Receive 5 real-time logs
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < 5 {
		select {
		case entry := <-ch:
			receivedCount++
			expectedContent := "realtime-" + toString(receivedCount)
			if entry.Content != expectedContent {
				t.Errorf("Expected content '%s', got '%s'", expectedContent, entry.Content)
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for real-time logs, received %d/5", receivedCount)
		}
	}
}

// TestLogBuffer_Unsubscribe tests Unsubscribe closes channel and stops goroutine
func TestLogBuffer_Unsubscribe(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Subscribe
	ch := lb.Subscribe()
	time.Sleep(50 * time.Millisecond) // Wait for goroutine to start

	// Write a log to verify channel is working
	entry := LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "test",
	}
	err := lb.Write(entry)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should receive the log
	select {
	case <-ch:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Should receive log before Unsubscribe")
	}

	// Unsubscribe
	lb.Unsubscribe(ch)
	time.Sleep(200 * time.Millisecond) // Wait for goroutine to stop completely

	// Verify channel is closed
	_, ok := <-ch
	if ok {
		t.Error("Channel should be closed after Unsubscribe")
	}

	// Write another log after unsubscribe
	entry2 := LogEntry{
		Timestamp: time.Now(),
		Source:    "stdout",
		Content:   "test2",
	}
	err = lb.Write(entry2)
	if err != nil {
		t.Fatalf("Write after unsubscribe failed: %v", err)
	}

	// Should not receive the log (channel closed)
	select {
	case _, received := <-ch:
		if received {
			t.Error("Should not receive log after Unsubscribe")
		}
	default:
		// Good - channel is closed and empty
	}
}

// TestLogBuffer_SlowSubscriber tests slow subscriber (not reading channel) doesn't block Write
func TestLogBuffer_SlowSubscriber(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Subscribe but don't read channel (simulating slow subscriber)
	_ = lb.Subscribe()

	// Write 200 logs (exceeds channel capacity 100)
	start := time.Now()
	for i := 1; i <= 200; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	// Verify all writes complete within 1 second (not blocked)
	if elapsed > 1*time.Second {
		t.Errorf("Write operations took too long: %v (should not block)", elapsed)
	}
}

// TestLogBuffer_ConcurrentSubscribe tests 10 concurrent subscribers all receive logs
func TestLogBuffer_ConcurrentSubscribe(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	numSubscribers := 10
	channels := make([]<-chan LogEntry, numSubscribers)

	// Start 10 subscribers
	for i := 0; i < numSubscribers; i++ {
		channels[i] = lb.Subscribe()
	}

	// Wait for all goroutines to start
	time.Sleep(50 * time.Millisecond)

	// Write 100 logs
	for i := 1; i <= 100; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Verify all subscribers receive 100 logs
	var wg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			receivedCount := 0
			timeout := time.After(5 * time.Second)

			for receivedCount < 100 {
				select {
				case <-channels[idx]:
					receivedCount++
				case <-timeout:
					t.Errorf("Subscriber %d timeout, received %d/100 logs", idx, receivedCount)
					return
				}
			}

			if receivedCount != 100 {
				t.Errorf("Subscriber %d expected 100 logs, received %d", idx, receivedCount)
			}
		}(i)
	}

	wg.Wait()
}

// Helper function to count current goroutines
func countGoroutines() int {
	return runtime.NumGoroutine()
}

// TestLogBuffer_Clear tests basic Clear() functionality
func TestLogBuffer_Clear(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Write 10 logs to buffer
	for i := 1; i <= 10; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Verify buffer has 10 entries
	history := lb.GetHistory()
	if len(history) != 10 {
		t.Fatalf("Expected 10 entries before Clear, got %d", len(history))
	}

	// Clear buffer
	lb.Clear()

	// Verify buffer is empty after Clear
	history = lb.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected 0 entries after Clear, got %d", len(history))
	}
}

// TestLogBuffer_Clear_EmptyBuffer tests Clear() on empty buffer
func TestLogBuffer_Clear_EmptyBuffer(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Verify buffer is initially empty
	history := lb.GetHistory()
	if len(history) != 0 {
		t.Fatalf("Expected 0 entries initially, got %d", len(history))
	}

	// Clear empty buffer (should be no-op)
	lb.Clear()

	// Verify buffer remains empty
	history = lb.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected 0 entries after Clear on empty buffer, got %d", len(history))
	}
}

// TestLogBuffer_Clear_WriteAfterClear tests Write() works after Clear()
func TestLogBuffer_Clear_WriteAfterClear(t *testing.T) {
	logger := createTestLogger()
	lb := NewLogBuffer(logger)

	// Write 5 logs to buffer
	for i := 1; i <= 5; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Clear buffer
	lb.Clear()

	// Write 3 new logs after Clear
	for i := 1; i <= 3; i++ {
		entry := LogEntry{
			Timestamp: time.Now(),
			Source:    "stdout",
			Content:   "new-log-" + toString(i),
		}
		err := lb.Write(entry)
		if err != nil {
			t.Fatalf("Write after Clear failed: %v", err)
		}
	}

	// Verify buffer has 3 new entries
	history := lb.GetHistory()
	if len(history) != 3 {
		t.Fatalf("Expected 3 entries after Clear and new writes, got %d", len(history))
	}

	// Verify content is correct
	for i := 0; i < 3; i++ {
		expectedContent := "new-log-" + toString(i+1)
		if history[i].Content != expectedContent {
			t.Errorf("Expected content '%s', got '%s'", expectedContent, history[i].Content)
		}
	}
}
