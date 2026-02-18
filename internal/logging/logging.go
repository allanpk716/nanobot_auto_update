package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger creates a new slog.Logger with custom format and file rotation.
// The logger writes to both a rotating log file and stdout simultaneously.
//
// Log format: "2024-01-01 12:00:00.123 - [INFO]: message"
// Log files are rotated at 50MB and kept for 7 days.
func NewLogger(logDir string) *slog.Logger {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// If we can't create the log directory, fall back to stdout only
		fmt.Fprintf(os.Stderr, "Warning: failed to create log directory %s: %v\n", logDir, err)
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			ReplaceAttr: customReplaceAttr,
		}))
	}

	// Configure lumberjack for log rotation
	fileLogger := &lumberjack.Logger{
		Filename:   logDir + "/app.log",
		MaxSize:    50, // MB - triggers rotation
		MaxBackups: 3,  // keep 3 old files
		MaxAge:     7,  // days - retention
		Compress:   false,
		LocalTime:  true,
	}

	// Use MultiWriter to output to both file and stdout
	multiWriter := io.MultiWriter(fileLogger, os.Stdout)

	// Create TextHandler with custom format
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		ReplaceAttr: customReplaceAttr,
	})

	return slog.New(handler)
}

// customReplaceAttr customizes the log format:
// - Time format: "2006-01-02 15:04:05.000" (millisecond precision)
// - Level format: "[INFO]", "[WARN]", "[ERROR]" (bracketed, uppercase)
// - Removes the "level=" and "time=" prefixes, formats as "timestamp - [LEVEL]: msg"
func customReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	// Handle time formatting with millisecond precision
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(time.Time); ok {
			// Format: "2006-01-02 15:04:05.000"
			a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
		}
	}

	// Handle level formatting as bracketed uppercase
	if a.Key == slog.LevelKey {
		if level, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(fmt.Sprintf("[%s]", level.String()))
		}
	}

	return a
}
