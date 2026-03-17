package logbuffer

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// LogEntry represents a single log entry with timestamp, source, and content
// BUFF-05: System retains timestamp, source (stdout/stderr), and content for each log
type LogEntry struct {
	Timestamp time.Time // Millisecond precision
	Source    string    // "stdout" or "stderr"
	Content   string
}

// LogBuffer is a thread-safe circular buffer for storing log entries
// BUFF-01: System maintains independent circular buffer for each nanobot instance
// BUFF-02: System limits buffer size to 5000 log lines
// BUFF-03: System uses thread-safe circular buffer implementation
type LogBuffer struct {
	mu          sync.RWMutex
	entries     [5000]LogEntry                   // Fixed capacity of 5000 entries (BUFF-02)
	head        int                              // Next write position (0-4999)
	size        int                              // Current entry count (0-5000)
	subscribers map[chan LogEntry]context.CancelFunc // Subscriber channels with cancel funcs
	logger      *slog.Logger
}

// NewLogBuffer creates a new log buffer with fixed capacity of 5000 entries
func NewLogBuffer(logger *slog.Logger) *LogBuffer {
	return &LogBuffer{
		entries:     [5000]LogEntry{}, // Pre-allocate 5000 entry capacity
		subscribers: make(map[chan LogEntry]context.CancelFunc),
		logger:      logger.With("component", "logbuffer"),
	}
}

// Write writes a log entry to the circular buffer
// BUFF-03: Thread-safe implementation using mutex
// BUFF-04: Automatic FIFO overwrite when buffer is full
func (lb *LogBuffer) Write(entry LogEntry) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Write to circular buffer (FIFO overwrite handled automatically)
	lb.entries[lb.head] = entry
	lb.head = (lb.head + 1) % 5000

	// Increment size until buffer is full
	if lb.size < 5000 {
		lb.size++
	}

	// Non-blocking send to all subscribers (CONTEXT.md constraint)
	// Slow subscribers: drop log if channel full (don't block)
	for ch := range lb.subscribers {
		select {
		case ch <- entry:
			// Send successful
		default:
			// Channel full, drop this log for this subscriber (don't block)
			lb.logger.Warn("Subscriber channel full, dropping log",
				"channel_capacity", cap(ch))
		}
	}

	return nil
}

// GetHistory returns all log entries in chronological order
// BUFF-03: Thread-safe read using RWMutex
func (lb *LogBuffer) GetHistory() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if lb.size == 0 {
		return []LogEntry{}
	}

	// Extract entries from circular buffer in chronological order
	result := make([]LogEntry, lb.size)

	if lb.size < 5000 {
		// Buffer not full: entries[0:size] contains valid data
		copy(result, lb.entries[:lb.size])
	} else {
		// Buffer full: entries[head:5000] + entries[0:head] contains valid data
		// head points to the oldest entry when buffer is full
		copy(result, lb.entries[lb.head:])
		copy(result[5000-lb.head:], lb.entries[:lb.head])
	}

	return result
}

// Clear resets the buffer to empty state
// INST-05: Support instance restart behavior - old logs discarded before restart
// Note: Subscribers continue receiving new logs after Clear() (subscribers map unchanged)
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Reset buffer state
	lb.head = 0
	lb.size = 0

	// Zero out entries array
	lb.entries = [5000]LogEntry{}

	lb.logger.Debug("Buffer cleared")
}
