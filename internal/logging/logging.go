package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
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

	// Build base output: "timestamp - [LEVEL]: message"
	output := fmt.Sprintf("%s - %s: %s", timestamp, level, record.Message)

	// Append handler-level attributes
	for _, attr := range h.attrs {
		output += fmt.Sprintf(" %s=%v", attr.Key, attr.Value)
	}

	// Append record-level attributes
	record.Attrs(func(attr slog.Attr) bool {
		output += fmt.Sprintf(" %s=%v", attr.Key, attr.Value)
		return true
	})

	// Write with newline
	_, err := fmt.Fprintln(h.w, output)
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
// Log files are organized by date (e.g., app-2024-01-01.log) and automatically
// rotate to a new file at midnight. Each daily file is rotated at 50MB.
// Old log files are kept for 7 days.
func NewLogger(logDir string) *slog.Logger {
	// Create daily rotating writer
	dailyWriter, err := newDailyRotateWriter(logDir)
	if err != nil {
		// If we can't create the daily writer, fall back to stdout only
		fmt.Fprintf(os.Stderr, "Warning: failed to create daily rotate writer: %v\n", err)
		return slog.New(&simpleHandler{w: os.Stdout})
	}

	// Use MultiWriter to output to both file and stdout
	multiWriter := io.MultiWriter(dailyWriter, os.Stdout)

	// Create custom handler with simple format
	handler := &simpleHandler{w: multiWriter}

	return slog.New(handler)
}
