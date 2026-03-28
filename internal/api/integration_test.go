package api

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

// --- E2E Integration Tests ---
// Verify the complete update log flow: trigger -> file persistence -> query retrieval,
// and update ID consistency.

// TestE2E_TriggerUpdate_RecordsTo_QueryReturns verifies the full chain:
// trigger update -> log recorded to file -> query returns same data
func TestE2E_TriggerUpdate_RecordsTo_QueryReturns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "updates.jsonl")

	// Create UpdateLogger with temp JSONL file
	ul := updatelog.NewUpdateLogger(logger, jsonlPath)
	defer ul.Close()

	// Create TriggerHandler with mock TriggerUpdater (successful result)
	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	triggerHandler := newTestHandler(logger, ul, mock)

	// Create QueryHandler sharing the same UpdateLogger
	queryHandler := NewQueryHandler(ul, logger)

	// Step 1: Trigger an update
	triggerReq := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	triggerRec := httptest.NewRecorder()
	triggerHandler.Handle(triggerRec, triggerReq)

	if triggerRec.Code != http.StatusOK {
		t.Fatalf("Trigger: status = %d, want %d", triggerRec.Code, http.StatusOK)
	}

	var triggerResp APIUpdateResult
	if err := json.NewDecoder(triggerRec.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("Failed to decode trigger response: %v", err)
	}

	if triggerResp.UpdateID == "" {
		t.Fatal("Trigger response missing update_id")
	}

	// Step 2: Query update-logs
	queryReq := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	queryRec := httptest.NewRecorder()
	queryHandler.Handle(queryRec, queryReq)

	if queryRec.Code != http.StatusOK {
		t.Fatalf("Query: status = %d, want %d", queryRec.Code, http.StatusOK)
	}

	var queryResp UpdateLogsResponse
	if err := json.NewDecoder(queryRec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("Failed to decode query response: %v", err)
	}

	// Step 3: Verify query returns the same update_id
	if queryResp.Meta.Total != 1 {
		t.Fatalf("Query total = %d, want 1", queryResp.Meta.Total)
	}
	if len(queryResp.Data) != 1 {
		t.Fatalf("Query data length = %d, want 1", len(queryResp.Data))
	}
	if queryResp.Data[0].ID != triggerResp.UpdateID {
		t.Errorf("Query data[0].ID = %q, want %q (same as trigger update_id)",
			queryResp.Data[0].ID, triggerResp.UpdateID)
	}

	// Step 4: Verify JSONL file was created and contains 1 line with the update_id
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("JSONL file has %d lines, want 1", len(lines))
	}
	if !strings.Contains(lines[0], triggerResp.UpdateID) {
		t.Errorf("JSONL line does not contain update_id %q: %s", triggerResp.UpdateID, lines[0])
	}
}

// TestE2E_UpdateID_Consistency verifies update_id match across multiple triggers and query.
// Triggers 3 sequential updates, queries all, verifies all IDs present in newest-first order.
func TestE2E_UpdateID_Consistency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "updates.jsonl")

	ul := updatelog.NewUpdateLogger(logger, jsonlPath)
	defer ul.Close()

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}

	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     30 * time.Second,
	}
	triggerHandler := NewTriggerHandler(mock, cfg, logger, ul)
	queryHandler := NewQueryHandler(ul, logger)

	// Trigger 3 sequential updates and collect update_ids
	updateIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
		rec := httptest.NewRecorder()
		triggerHandler.Handle(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Trigger %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}

		var resp APIUpdateResult
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("Trigger %d: failed to decode response: %v", i+1, err)
		}
		updateIDs[i] = resp.UpdateID

		// Small delay to ensure distinct timestamps for ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Query all update-logs
	queryReq := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	queryRec := httptest.NewRecorder()
	queryHandler.Handle(queryRec, queryReq)

	var queryResp UpdateLogsResponse
	if err := json.NewDecoder(queryRec.Body).Decode(&queryResp); err != nil {
		t.Fatalf("Failed to decode query response: %v", err)
	}

	// Verify all 3 records returned
	if queryResp.Meta.Total != 3 {
		t.Errorf("Query total = %d, want 3", queryResp.Meta.Total)
	}
	if len(queryResp.Data) != 3 {
		t.Fatalf("Query data length = %d, want 3", len(queryResp.Data))
	}

	// Verify all 3 IDs appear in results
	foundIDs := make(map[string]bool)
	for _, log := range queryResp.Data {
		foundIDs[log.ID] = true
	}
	for i, id := range updateIDs {
		if !foundIDs[id] {
			t.Errorf("Update ID %d (%s) not found in query results", i+1, id)
		}
	}

	// Verify newest-first order: data[0] should be updateIDs[2] (last triggered)
	if queryResp.Data[0].ID != updateIDs[2] {
		t.Errorf("Query data[0].ID = %q, want %q (newest-first: last triggered)",
			queryResp.Data[0].ID, updateIDs[2])
	}
	if queryResp.Data[1].ID != updateIDs[1] {
		t.Errorf("Query data[1].ID = %q, want %q (middle)",
			queryResp.Data[1].ID, updateIDs[1])
	}
	if queryResp.Data[2].ID != updateIDs[0] {
		t.Errorf("Query data[2].ID = %q, want %q (oldest: first triggered)",
			queryResp.Data[2].ID, updateIDs[0])
	}
}

// TestE2E_NonBlocking_FileWriteFailure verifies that file write failure does not
// affect the update operation. Update should succeed and log should be in memory.
func TestE2E_NonBlocking_FileWriteFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use invalid file path that will fail to open
	ul := updatelog.NewUpdateLogger(logger, "/nonexistent/deep/nested/dir/updates.jsonl")
	defer ul.Close()

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	triggerHandler := newTestHandler(logger, ul, mock)

	// Trigger an update
	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()
	triggerHandler.Handle(rec, req)

	// Verify 200 OK (update succeeds despite file write failure)
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d (update should succeed despite file write failure)",
			rec.Code, http.StatusOK)
	}

	var resp APIUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp.UpdateID == "" {
		t.Error("Response missing update_id")
	}
	if !resp.Success {
		t.Error("Response success = false, want true")
	}

	// Verify in-memory log was still recorded (GetAll returns 1 entry)
	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("GetAll() returned %d logs, want 1 (in-memory record should exist)", len(logs))
	}
	if logs[0].ID != resp.UpdateID {
		t.Errorf("In-memory log ID = %q, want %q", logs[0].ID, resp.UpdateID)
	}
}

// TestE2E_LoadFromFile_StartupRecovery verifies startup history recovery:
// pre-existing JSONL file -> LoadFromFile -> query returns loaded records ->
// trigger new update -> query returns all (loaded + new)
func TestE2E_LoadFromFile_StartupRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "updates.jsonl")

	// Write 3 JSONL lines manually to simulate pre-existing file
	now := time.Now().UTC()
	records := []updatelog.UpdateLog{
		{
			ID: "startup-uuid-1", StartTime: now.Add(-2 * time.Minute),
			EndTime: now.Add(-2*time.Minute + 5*time.Second), Duration: 5000,
			Status: updatelog.StatusSuccess, Instances: []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID: "startup-uuid-2", StartTime: now.Add(-1 * time.Minute),
			EndTime: now.Add(-1*time.Minute + 3*time.Second), Duration: 3000,
			Status: updatelog.StatusSuccess, Instances: []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
		{
			ID: "startup-uuid-3", StartTime: now,
			EndTime: now.Add(2 * time.Second), Duration: 2000,
			Status: updatelog.StatusSuccess, Instances: []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		},
	}

	dir := filepath.Dir(jsonlPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	for _, rec := range records {
		data, _ := json.Marshal(rec)
		f.Write(append(data, '\n'))
	}
	f.Close()

	// Create UpdateLogger with the pre-existing file
	ul := updatelog.NewUpdateLogger(logger, jsonlPath)
	defer ul.Close()

	// Simulate startup: LoadFromFile
	if err := ul.LoadFromFile(); err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	// Create handlers
	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	triggerHandler := newTestHandler(logger, ul, mock)
	queryHandler := NewQueryHandler(ul, logger)

	// Step 1: Query should return all 3 loaded records
	queryReq := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	queryRec := httptest.NewRecorder()
	queryHandler.Handle(queryRec, queryReq)

	var queryResp1 UpdateLogsResponse
	if err := json.NewDecoder(queryRec.Body).Decode(&queryResp1); err != nil {
		t.Fatalf("Failed to decode query response: %v", err)
	}
	if queryResp1.Meta.Total != 3 {
		t.Errorf("After LoadFromFile: total = %d, want 3", queryResp1.Meta.Total)
	}
	if len(queryResp1.Data) != 3 {
		t.Errorf("After LoadFromFile: data length = %d, want 3", len(queryResp1.Data))
	}

	// Verify all 3 pre-existing IDs are present
	loadedIDs := map[string]bool{"startup-uuid-1": true, "startup-uuid-2": true, "startup-uuid-3": true}
	for _, d := range queryResp1.Data {
		if !loadedIDs[d.ID] {
			t.Errorf("Unexpected ID in loaded data: %q", d.ID)
		}
	}

	// Step 2: Trigger a new update
	triggerReq := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	triggerRec := httptest.NewRecorder()
	triggerHandler.Handle(triggerRec, triggerReq)

	if triggerRec.Code != http.StatusOK {
		t.Fatalf("Trigger: status = %d, want %d", triggerRec.Code, http.StatusOK)
	}

	var triggerResp APIUpdateResult
	if err := json.NewDecoder(triggerRec.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("Failed to decode trigger response: %v", err)
	}

	// Step 3: Query again - should have 4 records (3 loaded + 1 new)
	queryReq2 := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	queryRec2 := httptest.NewRecorder()
	queryHandler.Handle(queryRec2, queryReq2)

	var queryResp2 UpdateLogsResponse
	if err := json.NewDecoder(queryRec2.Body).Decode(&queryResp2); err != nil {
		t.Fatalf("Failed to decode second query response: %v", err)
	}
	if queryResp2.Meta.Total != 4 {
		t.Errorf("After trigger: total = %d, want 4", queryResp2.Meta.Total)
	}
	if len(queryResp2.Data) != 4 {
		t.Errorf("After trigger: data length = %d, want 4", len(queryResp2.Data))
	}

	// Verify the newest record (data[0]) is the newly triggered one
	if queryResp2.Data[0].ID != triggerResp.UpdateID {
		t.Errorf("Newest record ID = %q, want %q (triggered update_id)",
			queryResp2.Data[0].ID, triggerResp.UpdateID)
	}

	// Verify the JSONL file now has 4 lines (3 original + 1 new)
	fileData, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("Failed to read JSONL file: %v", err)
	}
	lineCount := 0
	scanner := bufio.NewScanner(strings.NewReader(string(fileData)))
	for scanner.Scan() {
		if scanner.Text() != "" {
			lineCount++
		}
	}
	if lineCount != 4 {
		t.Errorf("JSONL file has %d lines, want 4", lineCount)
	}
}
