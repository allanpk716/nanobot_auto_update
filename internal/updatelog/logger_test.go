package updatelog

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewUpdateLogger(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	if ul == nil {
		t.Fatal("Expected non-nil UpdateLogger")
	}

	logs := ul.GetAll()
	if logs == nil {
		t.Fatal("Expected non-nil logs slice")
	}
	if len(logs) != 0 {
		t.Errorf("Expected empty logs slice, got %d logs", len(logs))
	}
}

func TestUpdateLogger_Record(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-uuid-1",
		StartTime:   now,
		EndTime:     now.Add(5 * time.Second),
		Duration:    5000,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	err := ul.Record(log)
	if err != nil {
		t.Errorf("Expected nil error from Record(), got %v", err)
	}

	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}
	if logs[0].ID != "test-uuid-1" {
		t.Errorf("Expected ID 'test-uuid-1', got '%s'", logs[0].ID)
	}
	if logs[0].Status != StatusSuccess {
		t.Errorf("Expected status %s, got %s", StatusSuccess, logs[0].Status)
	}

	// Record a second log
	log2 := UpdateLog{
		ID:          "test-uuid-2",
		StartTime:   now,
		EndTime:     now.Add(3 * time.Second),
		Duration:    3000,
		Status:      StatusFailed,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}
	ul.Record(log2)

	logs = ul.GetAll()
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs, got %d", len(logs))
	}
	if logs[1].ID != "test-uuid-2" {
		t.Errorf("Expected ID 'test-uuid-2', got '%s'", logs[1].ID)
	}
}

func TestUpdateLogger_ConcurrentRecord(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	var wg sync.WaitGroup
	count := 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			log := UpdateLog{
				ID:          "concurrent-uuid",
				StartTime:   time.Now().UTC(),
				EndTime:     time.Now().UTC(),
				Duration:    int64(idx),
				Status:      StatusSuccess,
				Instances:   []InstanceUpdateDetail{},
				TriggeredBy: "api-trigger",
			}
			ul.Record(log)
		}(i)
	}
	wg.Wait()

	logs := ul.GetAll()
	if len(logs) != count {
		t.Errorf("Expected %d logs after concurrent writes, got %d", count, len(logs))
	}
}

func TestUpdateLogger_GetAll_ReturnsCopy(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-uuid",
		StartTime:   now,
		EndTime:     now,
		Duration:    0,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}
	ul.Record(log)

	logs := ul.GetAll()
	// Modify the returned slice
	logs[0].ID = "modified-id"

	// Original should be unchanged
	original := ul.GetAll()
	if original[0].ID == "modified-id" {
		t.Error("GetAll() should return a copy, but modification affected original")
	}
	if original[0].ID != "test-uuid" {
		t.Errorf("Expected original ID 'test-uuid', got '%s'", original[0].ID)
	}
}

func TestWriteToFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-file-uuid",
		StartTime:   now,
		EndTime:     now.Add(5 * time.Second),
		Duration:    5000,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	err := ul.Record(log)
	if err != nil {
		t.Fatalf("Record() failed: %v", err)
	}

	// Read the JSONL file and verify content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in JSONL file, got %d", len(lines))
	}

	// Verify the line contains expected fields
	if !strings.Contains(lines[0], `"test-file-uuid"`) {
		t.Errorf("Expected line to contain 'test-file-uuid', got: %s", lines[0])
	}
	if !strings.Contains(lines[0], `"success"`) {
		t.Errorf("Expected line to contain 'success', got: %s", lines[0])
	}
}

func TestConcurrentFileWrite(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	var wg sync.WaitGroup
	count := 50

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			log := UpdateLog{
				ID:          fmt.Sprintf("concurrent-file-uuid-%d", idx),
				StartTime:   time.Now().UTC(),
				EndTime:     time.Now().UTC(),
				Duration:    int64(idx),
				Status:      StatusSuccess,
				Instances:   []InstanceUpdateDetail{},
				TriggeredBy: "api-trigger",
			}
			ul.Record(log)
		}(i)
	}
	wg.Wait()

	// Read the JSONL file and verify line count
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open JSONL file: %v", err)
	}
	defer f.Close()

	lineCount := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			lineCount++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Error scanning JSONL file: %v", err)
	}

	if lineCount != count {
		t.Errorf("Expected %d lines in JSONL file, got %d", count, lineCount)
	}
}

func TestAutoCreateFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	// Use a non-existent subdirectory
	filePath := filepath.Join(tmpDir, "subdir", "nested", "updates.jsonl")
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-autocreate-uuid",
		StartTime:   now,
		EndTime:     now,
		Duration:    100,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	err := ul.Record(log)
	if err != nil {
		t.Fatalf("Record() failed: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected JSONL file to be auto-created, but it does not exist")
	}

	// Verify the directory was created
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Expected directory to be auto-created, but it does not exist")
	}

	// Verify content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}
	if !strings.Contains(string(data), "test-autocreate-uuid") {
		t.Errorf("Expected file to contain 'test-autocreate-uuid', got: %s", string(data))
	}
}

func TestFileWriteErrorDegradation(t *testing.T) {
	logger := slog.Default()
	// Use an impossible path that will fail to open
	ul := NewUpdateLogger(logger, "/nonexistent/impossible/path/updates.jsonl")
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-degradation-uuid",
		StartTime:   now,
		EndTime:     now,
		Duration:    100,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	// Record should return nil (non-blocking per D-03)
	err := ul.Record(log)
	if err != nil {
		t.Errorf("Expected nil error from Record() even when file write fails, got %v", err)
	}

	// Log should still be in memory
	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log in memory, got %d", len(logs))
	}
	if logs[0].ID != "test-degradation-uuid" {
		t.Errorf("Expected log ID 'test-degradation-uuid', got '%s'", logs[0].ID)
	}
}
