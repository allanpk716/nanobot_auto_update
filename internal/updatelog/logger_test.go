package updatelog

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestNewUpdateLogger(t *testing.T) {
	logger := slog.Default()
	ul := NewUpdateLogger(logger)

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
	ul := NewUpdateLogger(logger)

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
	ul := NewUpdateLogger(logger)

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
	ul := NewUpdateLogger(logger)

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
