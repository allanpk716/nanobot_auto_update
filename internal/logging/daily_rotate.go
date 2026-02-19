package logging

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// dailyRotateWriter wraps a lumberjack.Logger and automatically rotates
// the log file when the date changes. It implements io.Writer.
type dailyRotateWriter struct {
	mu          sync.Mutex
	currentDate string
	logDir      string
	baseWriter  *lumberjack.Logger
}

// newDailyRotateWriter creates a new daily rotating writer
func newDailyRotateWriter(logDir string) (*dailyRotateWriter, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	// Initialize with current date
	currentDate := time.Now().Format("2006-01-02")
	logFilename := fmt.Sprintf("%s/app-%s.log", logDir, currentDate)

	// Create base lumberjack logger
	baseWriter := &lumberjack.Logger{
		Filename:   logFilename,
		MaxSize:    50, // MB - triggers rotation within the same day
		MaxBackups: 3,  // keep 3 old files per day
		MaxAge:     7,  // days - retention
		Compress:   false,
		LocalTime:  true,
	}

	return &dailyRotateWriter{
		currentDate: currentDate,
		logDir:      logDir,
		baseWriter:  baseWriter,
	}, nil
}

// Write implements io.Writer. It checks if the date has changed before writing,
// and rotates the file if necessary.
func (w *dailyRotateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if date has changed
	today := time.Now().Format("2006-01-02")
	if today != w.currentDate {
		// Rotate to new date file
		if err := w.rotateDate(today); err != nil {
			// If rotation fails, write to stderr and continue with old file
			fmt.Fprintf(os.Stderr, "Warning: failed to rotate log file: %v\n", err)
		}
	}

	// Write to current file
	return w.baseWriter.Write(p)
}

// rotateDate closes the current log file and creates a new one for the new date
func (w *dailyRotateWriter) rotateDate(newDate string) error {
	// Close the old writer
	if err := w.baseWriter.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close old log file: %v\n", err)
	}

	// Create new filename for the new date
	logFilename := fmt.Sprintf("%s/app-%s.log", w.logDir, newDate)

	// Create new lumberjack logger for the new date
	w.baseWriter = &lumberjack.Logger{
		Filename:   logFilename,
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
		LocalTime:  true,
	}

	w.currentDate = newDate
	return nil
}

// Close closes the underlying writer
func (w *dailyRotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.baseWriter.Close()
}
