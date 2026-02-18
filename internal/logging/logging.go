package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// simpleHandler implements slog.Handler with a simple format output:
// "2006-01-02 15:04:05.000 - [LEVEL]: message"
type simpleHandler struct {
	w     io.Writer
	attrs []slog.Attr
}

// Enabled always returns true - we log all levels
func (h *simpleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle writes the log record in simple format
func (h *simpleHandler) Handle(ctx context.Context, record slog.Record) error {
	// Format timestamp: "2006-01-02 15:04:05.000"
	timestamp := record.Time.Format("2006-01-02 15:04:05.000")

	// Format level: "[INFO]", "[WARN]", "[ERROR]", "[DEBUG]"
	level := fmt.Sprintf("[%s]", record.Level.String())

	// Build the output: "timestamp - [LEVEL]: message\n"
	_, err := fmt.Fprintf(h.w, "%s - %s: %s\n", timestamp, level, record.Message)
	return err
}

// WithAttrs returns a new handler with additional attributes
func (h *simpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &simpleHandler{
		w:     h.w,
		attrs: append(h.attrs, attrs...),
	}
}

// WithGroup returns self - groups not needed for simple format
func (h *simpleHandler) WithGroup(name string) slog.Handler {
	return h
}

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
		return slog.New(&simpleHandler{w: os.Stdout})
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

	// Create custom handler with simple format
	handler := &simpleHandler{w: multiWriter}

	return slog.New(handler)
}
