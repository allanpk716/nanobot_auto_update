package updatelog

import (
	"log/slog"
	"sync"
)

// UpdateLogger provides thread-safe in-memory storage for update log records.
// Phase 30: In-memory storage only. Phase 31 will add file persistence.
type UpdateLogger struct {
	logs   []UpdateLog
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewUpdateLogger creates a new UpdateLogger with an empty logs slice.
func NewUpdateLogger(logger *slog.Logger) *UpdateLogger {
	return &UpdateLogger{
		logs:   []UpdateLog{},
		logger: logger.With("component", "update-logger"),
	}
}

// Record appends an UpdateLog to the internal slice.
// This method is thread-safe and never blocks the caller.
// Phase 30: In-memory storage only. Phase 31 will add file persistence.
func (ul *UpdateLogger) Record(log UpdateLog) error {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.logs = append(ul.logs, log)
	ul.logger.Debug("Recorded update log",
		"update_id", log.ID,
		"status", log.Status,
		"duration_ms", log.Duration,
		"instance_count", len(log.Instances))
	return nil
}

// GetAll returns a copy of all recorded logs.
// Thread-safe read using RWMutex.
func (ul *UpdateLogger) GetAll() []UpdateLog {
	ul.mu.RLock()
	defer ul.mu.RUnlock()
	// Return a copy to prevent external modification
	result := make([]UpdateLog, len(ul.logs))
	copy(result, ul.logs)
	return result
}
