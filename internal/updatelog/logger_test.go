package updatelog

import (
	"bufio"
	"encoding/json"
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

func TestCleanupOldLogs(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")

	// Pre-populate JSONL file with 3 records of different ages
	now := time.Now().UTC()
	records := []UpdateLog{
		{
			ID:          "old-record-8days",
			StartTime:   now.Add(-8 * 24 * time.Hour),
			EndTime:     now.Add(-8*24*time.Hour + 5*time.Second),
			Duration:    5000,
			Status:      StatusSuccess,
			Instances:   []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID:          "recent-record-6days",
			StartTime:   now.Add(-6 * 24 * time.Hour),
			EndTime:     now.Add(-6*24*time.Hour + 3*time.Second),
			Duration:    3000,
			Status:      StatusSuccess,
			Instances:   []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID:          "today-record",
			StartTime:   now,
			EndTime:     now.Add(2 * time.Second),
			Duration:    2000,
			Status:      StatusSuccess,
			Instances:   []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
	}

	// Write records directly to file
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	for _, rec := range records {
		data, _ := json.Marshal(rec)
		f.Write(append(data, '\n'))
	}
	f.Close()

	// Create UpdateLogger pointing to this file
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	// Run cleanup
	err = ul.CleanupOldLogs()
	if err != nil {
		t.Fatalf("CleanupOldLogs() failed: %v", err)
	}

	// Read the file and verify only 2 lines remain
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file after cleanup: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines after cleanup, got %d", len(lines))
	}

	// Verify the old record was removed
	for _, line := range lines {
		if strings.Contains(line, "old-record-8days") {
			t.Error("Old record should have been removed")
		}
	}

	// Verify recent records remain
	found6days := false
	foundToday := false
	for _, line := range lines {
		if strings.Contains(line, "recent-record-6days") {
			found6days = true
		}
		if strings.Contains(line, "today-record") {
			foundToday = true
		}
	}
	if !found6days {
		t.Error("Expected to find 6-day-old record")
	}
	if !foundToday {
		t.Error("Expected to find today's record")
	}
}

func TestCleanupNoBlock(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-noblock-uuid",
		StartTime:   now.Add(-8 * 24 * time.Hour), // Old record to trigger cleanup work
		EndTime:     now,
		Duration:    1000,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}
	ul.Record(log)

	// Verify GetAll() is not blocked by CleanupOldLogs()
	done := make(chan struct{})
	go func() {
		ul.CleanupOldLogs()
		close(done)
	}()

	// GetAll should return within 200ms even while cleanup is running
	getDone := make(chan []UpdateLog, 1)
	go func() {
		getDone <- ul.GetAll()
	}()

	select {
	case result := <-getDone:
		if result == nil {
			t.Error("GetAll() should not return nil")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("GetAll() was blocked by CleanupOldLogs() - locks not properly separated")
	}

	// Wait for cleanup to finish
	<-done
}

func TestCleanupNoFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	// Point to a file that does not exist
	filePath := filepath.Join(tmpDir, "nonexistent.jsonl")
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	err := ul.CleanupOldLogs()
	if err != nil {
		t.Errorf("Expected nil error when file does not exist, got %v", err)
	}

	// Verify file was NOT created
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Cleanup should not create a file when none exists")
	}
}

func TestClose(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")
	ul := NewUpdateLogger(logger, filePath)

	now := time.Now().UTC()
	log := UpdateLog{
		ID:          "test-close-uuid",
		StartTime:   now,
		EndTime:     now,
		Duration:    100,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	// Record opens the file
	ul.Record(log)

	// Close the file handle
	err := ul.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Verify subsequent Record() still works (should re-open the file)
	log2 := UpdateLog{
		ID:          "test-close-uuid-2",
		StartTime:   now,
		EndTime:     now,
		Duration:    200,
		Status:      StatusSuccess,
		Instances:   []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}
	err = ul.Record(log2)
	if err != nil {
		t.Errorf("Record() after Close() failed: %v", err)
	}

	// Verify both records in memory
	logs := ul.GetAll()
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs after Close() + Record(), got %d", len(logs))
	}

	// Close again to clean up
	ul.Close()
}

func TestCloseWithoutOpen(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))

	// Close without ever recording (file never opened)
	err := ul.Close()
	if err != nil {
		t.Errorf("Close() without open should return nil, got %v", err)
	}
}

// --- GetPage Tests ---

// newPageTestLogs creates n UpdateLog entries with sequential IDs and incrementing StartTime.
// Entry i has ID "page-uuid-(i+1)" and StartTime = baseTime + i*minute.
func newPageTestLogs(n int, baseTime time.Time) []UpdateLog {
	logs := make([]UpdateLog, n)
	for i := 0; i < n; i++ {
		logs[i] = UpdateLog{
			ID:          fmt.Sprintf("page-uuid-%d", i+1),
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      StatusSuccess,
			Instances:   []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		}
	}
	return logs
}

func seedLogs(t *testing.T, ul *UpdateLogger, logs []UpdateLog) {
	t.Helper()
	for _, l := range logs {
		if err := ul.Record(l); err != nil {
			t.Fatalf("Failed to seed log %s: %v", l.ID, err)
		}
	}
}

func TestGetPage_BasicPagination(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(5, baseTime))

	// Test 1: limit=2, offset=0 returns 2 most recent logs and total=5
	result, total := ul.GetPage(2, 0)
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if len(result) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(result))
	}
	// Newest first: page-uuid-5 is the newest (highest StartTime)
	if result[0].ID != "page-uuid-5" {
		t.Errorf("Expected result[0].ID='page-uuid-5', got '%s'", result[0].ID)
	}
	if result[1].ID != "page-uuid-4" {
		t.Errorf("Expected result[1].ID='page-uuid-4', got '%s'", result[1].ID)
	}
}

func TestGetPage_SecondPage(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(5, baseTime))

	// Test 2: limit=2, offset=2 returns 2 older logs (skipping 2 newest) and total=5
	result, total := ul.GetPage(2, 2)
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if len(result) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(result))
	}
	// After skipping 2 newest (page-uuid-5, page-uuid-4), next 2 are page-uuid-3, page-uuid-2
	if result[0].ID != "page-uuid-3" {
		t.Errorf("Expected result[0].ID='page-uuid-3', got '%s'", result[0].ID)
	}
	if result[1].ID != "page-uuid-2" {
		t.Errorf("Expected result[1].ID='page-uuid-2', got '%s'", result[1].ID)
	}
}

func TestGetPage_OffsetExceedsTotal(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(5, baseTime))

	// Test 3: offset >= total returns empty non-nil slice and correct total
	result, total := ul.GetPage(2, 10)
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if result == nil {
		t.Fatal("Expected non-nil result slice")
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 results, got %d", len(result))
	}
}

func TestGetPage_LimitExceedsRemaining(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(5, baseTime))

	// Test 4: limit > remaining logs returns only available count
	result, total := ul.GetPage(10, 3)
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if len(result) != 2 {
		t.Fatalf("Expected 2 results (5 total - 3 offset), got %d", len(result))
	}
	if result[0].ID != "page-uuid-2" {
		t.Errorf("Expected result[0].ID='page-uuid-2', got '%s'", result[0].ID)
	}
	if result[1].ID != "page-uuid-1" {
		t.Errorf("Expected result[1].ID='page-uuid-1', got '%s'", result[1].ID)
	}
}

func TestGetPage_EmptyLogs(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	// Test 5: empty logs returns empty slice and total=0
	result, total := ul.GetPage(10, 0)
	if total != 0 {
		t.Errorf("Expected total=0, got %d", total)
	}
	if result == nil {
		t.Fatal("Expected non-nil result slice")
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 results, got %d", len(result))
	}
}

func TestGetPage_ZeroLimit(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(5, baseTime))

	// Test 6: limit=0 returns empty slice
	result, total := ul.GetPage(0, 0)
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 results for limit=0, got %d", len(result))
	}
}

func TestGetPage_DefensiveCopy(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(3, baseTime))

	// Test 7: modifying returned slice does not affect internal state
	result, _ := ul.GetPage(2, 0)
	if len(result) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(result))
	}
	result[0].ID = "hacked-id"

	// Fetch again and verify original is unchanged
	fresh, _ := ul.GetPage(2, 0)
	if fresh[0].ID == "hacked-id" {
		t.Error("GetPage() should return a defensive copy, but modification affected internal state")
	}
	if fresh[0].ID != "page-uuid-3" {
		t.Errorf("Expected fresh result[0].ID='page-uuid-3', got '%s'", fresh[0].ID)
	}
}

func TestGetPage_ConcurrentWithRecord(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	ul := NewUpdateLogger(logger, filepath.Join(tmpDir, "updates.jsonl"))
	defer ul.Close()

	baseTime := time.Now().UTC()
	seedLogs(t, ul, newPageTestLogs(10, baseTime))

	// Test 8: concurrent GetPage + Record should not race or panic
	var wg sync.WaitGroup
	wg.Add(2)

	// Continuous Record goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			ul.Record(UpdateLog{
				ID:          fmt.Sprintf("concurrent-page-%d", i),
				StartTime:   time.Now().UTC(),
				EndTime:     time.Now().UTC(),
				Duration:    100,
				Status:      StatusSuccess,
				Instances:   []InstanceUpdateDetail{},
				TriggeredBy: "api-trigger",
			})
		}
	}()

	// Continuous GetPage goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			result, total := ul.GetPage(5, 0)
			if result == nil {
				t.Error("GetPage returned nil result during concurrent access")
			}
			if total < 10 {
				t.Errorf("Expected total >= 10 during concurrent access, got %d", total)
			}
		}
	}()

	wg.Wait()
}

// --- LoadFromFile Tests ---

func TestLoadFromFile_ValidFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")

	// Create a valid JSONL file with 3 records
	now := time.Now().UTC()
	records := []UpdateLog{
		{
			ID: "load-uuid-1", StartTime: now, EndTime: now.Add(5 * time.Second),
			Duration: 5000, Status: StatusSuccess, Instances: []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID: "load-uuid-2", StartTime: now.Add(1 * time.Minute), EndTime: now.Add(1*time.Minute + 3*time.Second),
			Duration: 3000, Status: StatusFailed, Instances: []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID: "load-uuid-3", StartTime: now.Add(2 * time.Minute), EndTime: now.Add(2*time.Minute + 2*time.Second),
			Duration: 2000, Status: StatusPartialSuccess, Instances: []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	for _, rec := range records {
		data, _ := json.Marshal(rec)
		f.Write(append(data, '\n'))
	}
	f.Close()

	// Test 1: LoadFromFile loads all 3 records
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	err = ul.LoadFromFile()
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	logs := ul.GetAll()
	if len(logs) != 3 {
		t.Fatalf("Expected 3 loaded logs, got %d", len(logs))
	}

	// Verify IDs are loaded correctly
	ids := make(map[string]bool)
	for _, l := range logs {
		ids[l.ID] = true
	}
	for _, rec := range records {
		if !ids[rec.ID] {
			t.Errorf("Expected to find ID '%s' in loaded logs", rec.ID)
		}
	}
}

func TestLoadFromFile_NonExistentFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.jsonl")

	// Test 2: non-existent file returns nil error (no-op)
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	err := ul.LoadFromFile()
	if err != nil {
		t.Errorf("Expected nil error for non-existent file, got %v", err)
	}

	logs := ul.GetAll()
	if len(logs) != 0 {
		t.Errorf("Expected 0 logs, got %d", len(logs))
	}
}

func TestLoadFromFile_MemoryOnlyMode(t *testing.T) {
	logger := slog.Default()

	// Test 3: memory-only mode (empty filePath) returns nil error
	ul := NewUpdateLogger(logger, "")
	defer ul.Close()

	err := ul.LoadFromFile()
	if err != nil {
		t.Errorf("Expected nil error for memory-only mode, got %v", err)
	}
}

func TestLoadFromFile_SkipsInvalidJSON(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")

	// Create JSONL file with mix of valid and invalid lines
	now := time.Now().UTC()
	validRec := UpdateLog{
		ID: "valid-load-uuid", StartTime: now, EndTime: now.Add(5 * time.Second),
		Duration: 5000, Status: StatusSuccess, Instances: []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	// Write: invalid, valid, invalid, empty line
	f.WriteString("this is not json\n")
	data, _ := json.Marshal(validRec)
	f.Write(append(data, '\n'))
	f.WriteString("{bad json\n")
	f.WriteString("\n") // empty line
	f.Close()

	// Test 4: only the valid record is loaded
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	err = ul.LoadFromFile()
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 valid log loaded, got %d", len(logs))
	}
	if logs[0].ID != "valid-load-uuid" {
		t.Errorf("Expected ID 'valid-load-uuid', got '%s'", logs[0].ID)
	}
}

func TestLoadFromFile_AppendsToExisting(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")

	// Create JSONL file with 2 records (without using UpdateLogger to avoid file write side effects)
	now := time.Now().UTC()
	fileRecords := []UpdateLog{
		{
			ID: "file-uuid-1", StartTime: now, EndTime: now,
			Duration: 200, Status: StatusSuccess, Instances: []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID: "file-uuid-2", StartTime: now, EndTime: now,
			Duration: 300, Status: StatusFailed, Instances: []InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	for _, rec := range fileRecords {
		data, _ := json.Marshal(rec)
		f.Write(append(data, '\n'))
	}
	f.Close()

	// Create logger and add a pre-existing in-memory log directly (not via Record to avoid file write)
	existingLog := UpdateLog{
		ID: "existing-uuid", StartTime: time.Now().UTC(), EndTime: time.Now().UTC(),
		Duration: 100, Status: StatusSuccess, Instances: []InstanceUpdateDetail{},
		TriggeredBy: "api-trigger",
	}
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	// Directly add to internal slice to simulate pre-existing in-memory state
	ul.mu.Lock()
	ul.logs = append(ul.logs, existingLog)
	ul.mu.Unlock()

	// Test 5: LoadFromFile appends to existing in-memory logs
	err = ul.LoadFromFile()
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	// Test 6: GetAll returns both pre-existing and loaded logs
	logs := ul.GetAll()
	if len(logs) != 3 {
		t.Fatalf("Expected 3 logs (1 existing + 2 loaded), got %d", len(logs))
	}

	ids := make(map[string]bool)
	for _, l := range logs {
		ids[l.ID] = true
	}
	if !ids["existing-uuid"] {
		t.Error("Expected to find 'existing-uuid' in combined logs")
	}
	if !ids["file-uuid-1"] {
		t.Error("Expected to find 'file-uuid-1' in combined logs")
	}
	if !ids["file-uuid-2"] {
		t.Error("Expected to find 'file-uuid-2' in combined logs")
	}
}

func TestLoadFromFile_EmptyFile(t *testing.T) {
	logger := slog.Default()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "updates.jsonl")

	// Create empty file
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	f.Close()

	// Test 7: empty file returns nil error with no records loaded
	ul := NewUpdateLogger(logger, filePath)
	defer ul.Close()

	err = ul.LoadFromFile()
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	logs := ul.GetAll()
	if len(logs) != 0 {
		t.Errorf("Expected 0 logs from empty file, got %d", len(logs))
	}
}
