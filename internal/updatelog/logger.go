package updatelog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UpdateLogger provides thread-safe in-memory storage and JSONL file persistence for update log records.
// File operations use a separate mutex (fileMu) to avoid blocking GetAll().
type UpdateLogger struct {
	logs     []UpdateLog
	mu       sync.RWMutex
	logger   *slog.Logger
	filePath string     // JSONL file path (e.g. "./logs/updates.jsonl")
	file     *os.File   // Open file handle (nil = memory-only mode or not yet opened)
	fileMu   sync.Mutex // Mutex for file operations (separate from mu to avoid blocking GetAll)
}

// NewUpdateLogger creates a new UpdateLogger with an empty logs slice and the given file path.
// If filePath is empty, the logger operates in memory-only mode (no file persistence).
func NewUpdateLogger(logger *slog.Logger, filePath string) *UpdateLogger {
	return &UpdateLogger{
		logs:     []UpdateLog{},
		logger:   logger.With("component", "update-logger"),
		filePath: filePath,
	}
}

// openFile lazily creates the directory and opens the JSONL file for append.
func (ul *UpdateLogger) openFile() error {
	dir := filepath.Dir(ul.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	f, err := os.OpenFile(ul.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open JSONL file: %w", err)
	}
	ul.file = f
	return nil
}

// writeToFile writes a single UpdateLog as a JSON line to the JSONL file.
// Caller must NOT hold fileMu; this method acquires it.
func (ul *UpdateLogger) writeToFile(log UpdateLog) error {
	ul.fileMu.Lock()
	defer ul.fileMu.Unlock()

	if ul.file == nil {
		if err := ul.openFile(); err != nil {
			return err
		}
	}

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal update log: %w", err)
	}
	data = append(data, '\n')

	if _, err := ul.file.Write(data); err != nil {
		ul.file.Close()
		ul.file = nil
		return fmt.Errorf("failed to write to file: %w", err)
	}

	if err := ul.file.Sync(); err != nil {
		ul.logger.Warn("fsync failed", "error", err)
	}
	return nil
}

// Record appends an UpdateLog to the internal slice and writes it to the JSONL file.
// This method is thread-safe and never blocks the caller.
// If file write fails, the log is still kept in memory (non-blocking degradation).
func (ul *UpdateLogger) Record(log UpdateLog) error {
	ul.mu.Lock()
	ul.logs = append(ul.logs, log)
	ul.mu.Unlock()

	if ul.filePath != "" {
		if err := ul.writeToFile(log); err != nil {
			ul.logger.Error("Failed to write update log to file",
				"error", err, "update_id", log.ID)
			// D-03: degrade to memory-only, do not return error
		}
	}

	ul.logger.Debug("Recorded update log",
		"update_id", log.ID,
		"status", log.Status,
		"duration_ms", log.Duration,
		"instance_count", len(log.Instances))
	return nil
}

// GetAll returns a copy of all recorded logs.
// Thread-safe read using RWMutex. Not blocked by file operations.
func (ul *UpdateLogger) GetAll() []UpdateLog {
	ul.mu.RLock()
	defer ul.mu.RUnlock()
	// Return a copy to prevent external modification
	result := make([]UpdateLog, len(ul.logs))
	copy(result, ul.logs)
	return result
}

// CleanupOldLogs removes records older than 7 days from the JSONL file.
// Uses temp file + atomic rename pattern for safe cleanup.
// This method closes the file handle before rename (Windows compatibility).
func (ul *UpdateLogger) CleanupOldLogs() error {
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)

	ul.fileMu.Lock()
	defer ul.fileMu.Unlock()

	// Close file handle (Windows requires this for rename)
	if ul.file != nil {
		ul.file.Close()
		ul.file = nil
	}

	// File doesn't exist, nothing to clean
	if _, err := os.Stat(ul.filePath); os.IsNotExist(err) {
		return nil
	}

	// Open source file for reading
	src, err := os.Open(ul.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for cleanup: %w", err)
	}
	defer src.Close()

	// Create temp file in same directory (same filesystem for atomic rename)
	dir := filepath.Dir(ul.filePath)
	tmp, err := os.CreateTemp(dir, "updates-*.jsonl.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Stream read + write kept records
	scanner := bufio.NewScanner(src)
	kept := 0
	removed := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var log UpdateLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue // skip invalid lines
		}
		if log.StartTime.After(cutoff) {
			tmp.WriteString(line + "\n")
			kept++
		} else {
			removed++
		}
	}
	src.Close()
	tmp.Close()

	if removed == 0 {
		os.Remove(tmpPath)
		return nil
	}

	// Atomic rename: temp -> target
	if err := os.Rename(tmpPath, ul.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	ul.logger.Info("Cleaned up old log records",
		"kept", kept, "removed", removed)
	return nil
}

// Close closes the file handle if it is open.
// It is safe to call Close() even if the file was never opened.
func (ul *UpdateLogger) Close() error {
	ul.fileMu.Lock()
	defer ul.fileMu.Unlock()

	if ul.file != nil {
		err := ul.file.Close()
		ul.file = nil
		return err
	}
	return nil
}
