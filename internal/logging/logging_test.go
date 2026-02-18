package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	logger.Info("test message", "key", "value")

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
		{"key-value pair", "key=value"},
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
