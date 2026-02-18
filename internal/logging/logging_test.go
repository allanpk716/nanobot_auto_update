package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLoggerFormat verifies the exact log output format:
// "YYYY-MM-DD HH:MM:SS.mmm - [LEVEL]: message"
func TestLoggerFormat(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	handler := &simpleHandler{w: &buf}
	logger := slog.New(handler)

	// Test all four log levels
	tests := []struct {
		name    string
		logFunc func(string, ...any)
		level   string
	}{
		{"Debug", logger.Debug, "DEBUG"},
		{"Info", logger.Info, "INFO"},
		{"Warn", logger.Warn, "WARN"},
		{"Error", logger.Error, "ERROR"},
	}

	// Regex for exact format: "2006-01-02 15:04:05.000 - [LEVEL]: message"
	formatRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} - \[(DEBUG|INFO|WARN|ERROR)\]: .+$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			// Log a test message
			tt.logFunc("test message")

			output := strings.TrimSpace(buf.String())

			// Verify output matches exact format
			if !formatRegex.MatchString(output) {
				t.Errorf("Output does not match format \"YYYY-MM-DD HH:MM:SS.mmm - [LEVEL]: message\"\nGot: %q", output)
			}

			// Verify level is correct
			if !strings.Contains(output, "["+tt.level+"]") {
				t.Errorf("Output does not contain level [%s]\nGot: %s", tt.level, output)
			}

			// Verify NO key=value prefixes
			if strings.Contains(output, "time=") {
				t.Errorf("Output contains \"time=\" prefix, should not have key=value format\nGot: %s", output)
			}
			if strings.Contains(output, "level=") {
				t.Errorf("Output contains \"level=\" prefix, should not have key=value format\nGot: %s", output)
			}
			if strings.Contains(output, "msg=") {
				t.Errorf("Output contains \"msg=\" prefix, should not have key=value format\nGot: %s", output)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	// Create a temporary directory for logs
	tempDir, err := os.MkdirTemp("", "logging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test

	// Create logger
	logger := NewLogger(tempDir)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	// Log a test message
	logger.Info("test message")

	// Verify the log file was created
	logFile := filepath.Join(tempDir, "app.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	output := string(content)

	// Verify the output contains expected format components
	tests := []struct {
		name     string
		contains string
	}{
		{"timestamp date", "202"},
		{"timestamp time", ":"},
		{"level marker", "[INFO]"},
		{"message", "test message"},
		{"separator", " - "},
		{"message separator", ": "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("Output does not contain %q\nGot: %s", tt.contains, output)
			}
		})
	}
}

func TestNewLoggerCreatesDirectory(t *testing.T) {
	// Create a temp directory and a subdirectory path that doesn't exist
	tempDir, err := os.MkdirTemp("", "logging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use a subdirectory that doesn't exist yet
	logDir := filepath.Join(tempDir, "logs", "nested")

	// Create logger - should create the directory
	logger := NewLogger(logDir)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	// Verify the directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created: %s", logDir)
	}
}
