package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyRotateWriter_Write(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create daily rotate writer
	writer, err := newDailyRotateWriter(tempDir)
	if err != nil {
		t.Fatalf("Failed to create daily rotate writer: %v", err)
	}
	defer writer.Close()

	// Write some data
	testData := []byte("test log message\n")
	n, err := writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Verify file was created with today's date
	today := time.Now().Format("2006-01-02")
	expectedFilename := filepath.Join(tempDir, "app-"+today+".log")
	if _, err := os.Stat(expectedFilename); os.IsNotExist(err) {
		t.Errorf("Expected log file %s was not created", expectedFilename)
	}

	// Verify content
	content, err := os.ReadFile(expectedFilename)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "test log message") {
		t.Errorf("Log file does not contain expected message, got: %s", string(content))
	}
}

func TestDailyRotateWriter_Rotation(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create daily rotate writer
	writer, err := newDailyRotateWriter(tempDir)
	if err != nil {
		t.Fatalf("Failed to create daily rotate writer: %v", err)
	}
	defer writer.Close()

	// Write initial data with current date
	initialData := []byte("initial message\n")
	_, err = writer.Write(initialData)
	if err != nil {
		t.Fatalf("Failed to write initial data: %v", err)
	}

	// Get current date
	currentDate := writer.currentDate

	// Simulate date change by manually setting it to a different date
	writer.mu.Lock()
	writer.currentDate = "2000-01-01" // Set to past date to trigger rotation
	writer.mu.Unlock()

	// Write new data - should trigger rotation
	newData := []byte("new message after rotation\n")
	_, err = writer.Write(newData)
	if err != nil {
		t.Fatalf("Failed to write new data: %v", err)
	}

	// Verify that currentDate was updated (rotation happened)
	writer.mu.Lock()
	newDate := writer.currentDate
	writer.mu.Unlock()

	if newDate == "2000-01-01" {
		t.Error("Date was not updated after rotation")
	}

	if newDate != time.Now().Format("2006-01-02") {
		t.Errorf("Expected date %s, got %s", time.Now().Format("2006-01-02"), newDate)
	}

	_ = currentDate // Avoid unused variable error
}
