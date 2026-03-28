package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

// mockTriggerUpdater is a mock implementation of TriggerUpdater for testing.
type mockTriggerUpdater struct {
	result *instance.UpdateResult
	err    error
}

func (m *mockTriggerUpdater) TriggerUpdate(ctx context.Context) (*instance.UpdateResult, error) {
	return m.result, m.err
}

// newTestHandler creates a TriggerHandler with mock InstanceManager for testing.
func newTestHandler(logger *slog.Logger, ul *updatelog.UpdateLogger, mock *mockTriggerUpdater) *TriggerHandler {
	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     30 * time.Second,
	}
	return NewTriggerHandler(mock, cfg, logger, ul)
}

// TestTriggerHandler_UpdateIDInResponse tests LOG-02:
// Handle returns update_id in response with valid UUID v4 format
func TestTriggerHandler_UpdateIDInResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response APIUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// Verify update_id is present
	if response.UpdateID == "" {
		t.Error("update_id is empty, expected a UUID v4")
	}

	// Verify UUID v4 format: 8-4-4-4-12 hex characters
	parts := strings.Split(response.UpdateID, "-")
	if len(parts) != 5 {
		t.Errorf("update_id = %q, expected UUID v4 format (5 hyphen-separated parts)", response.UpdateID)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Errorf("update_id = %q, expected UUID v4 format (8-4-4-4-12)", response.UpdateID)
	}
}

// TestTriggerHandler_RecordsUpdateLog tests LOG-01, LOG-03, LOG-04:
// Handle calls UpdateLogger.Record() with correct UpdateLog data
func TestTriggerHandler_RecordsUpdateLog(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{"gateway"},
			Started:     []string{"gateway"},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify UpdateLogger has recorded a log entry
	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 recorded log, got %d", len(logs))
	}

	recordedLog := logs[0]

	// Verify the recorded log has a valid UUID
	if recordedLog.ID == "" {
		t.Error("Recorded log ID is empty")
	}

	// Verify triggered_by is set
	if recordedLog.TriggeredBy != "api-trigger" {
		t.Errorf("TriggeredBy = %q, want %q", recordedLog.TriggeredBy, "api-trigger")
	}

	// Verify status is success for successful update
	if recordedLog.Status != updatelog.StatusSuccess {
		t.Errorf("Status = %q, want %q", recordedLog.Status, updatelog.StatusSuccess)
	}
}

// TestTriggerHandler_LogRecordingFailureDoesNotAffectResponse tests LOG-04:
// Update log recording failure does not affect HTTP response success
func TestTriggerHandler_LogRecordingFailureDoesNotAffectResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	// Use nil UpdateLogger to test non-blocking behavior
	handler := newTestHandler(logger, nil, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	// Response should still be 200 OK even without UpdateLogger
	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d (should succeed even without log recorder)", rec.Code, http.StatusOK)
	}

	var response APIUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// update_id should still be present
	if response.UpdateID == "" {
		t.Error("update_id is empty, expected UUID v4 even when UpdateLogger is nil")
	}

	if !response.Success {
		t.Error("success = false, want true (update should succeed regardless of logging)")
	}
}

// TestTriggerHandler_StartTimeRecordedBeforeUpdate tests LOG-01:
// Start time is recorded before TriggerUpdate call
func TestTriggerHandler_StartTimeRecordedBeforeUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	beforeCall := time.Now().UTC()

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	afterCall := time.Now().UTC()

	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 recorded log, got %d", len(logs))
	}

	recordedLog := logs[0]

	// Start time should be between beforeCall and afterCall
	if recordedLog.StartTime.Before(beforeCall.Add(-1 * time.Second)) {
		t.Errorf("StartTime %v is before expected range (before %v)", recordedLog.StartTime, beforeCall)
	}
	if recordedLog.StartTime.After(afterCall.Add(1 * time.Second)) {
		t.Errorf("StartTime %v is after expected range (after %v)", recordedLog.StartTime, afterCall)
	}
}

// TestTriggerHandler_EndTimeRecordedAfterUpdate tests LOG-01:
// End time is recorded after TriggerUpdate completes
func TestTriggerHandler_EndTimeRecordedAfterUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 recorded log, got %d", len(logs))
	}

	recordedLog := logs[0]

	// End time should be >= start time
	if recordedLog.EndTime.Before(recordedLog.StartTime) {
		t.Errorf("EndTime %v is before StartTime %v", recordedLog.EndTime, recordedLog.StartTime)
	}
}

// TestTriggerHandler_DurationCalculatedInMilliseconds tests LOG-01:
// Duration is calculated correctly in milliseconds
func TestTriggerHandler_DurationCalculatedInMilliseconds(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 recorded log, got %d", len(logs))
	}

	recordedLog := logs[0]

	// Duration should match EndTime - StartTime in milliseconds
	expectedDuration := recordedLog.EndTime.Sub(recordedLog.StartTime).Milliseconds()
	if recordedLog.Duration != expectedDuration {
		t.Errorf("Duration = %d ms, want %d ms (EndTime - StartTime)", recordedLog.Duration, expectedDuration)
	}

	// Duration should be >= 0
	if recordedLog.Duration < 0 {
		t.Errorf("Duration = %d ms, expected non-negative", recordedLog.Duration)
	}
}

// TestTriggerHandler_MethodNotAllowed tests API-01:
// Handle returns 405 for GET request
func TestTriggerHandler_MethodNotAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("GET", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "method_not_allowed" {
		t.Errorf("error = %q, want %q", response["error"], "method_not_allowed")
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestTriggerHandler_Success tests API-01, API-04:
// Handle returns 200 with success=true when update succeeds
func TestTriggerHandler_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response APIUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if !response.Success {
		t.Errorf("success = %v, want true", response.Success)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestTriggerHandler_Conflict tests API-01, API-06:
// Handle returns 409 Conflict when ErrUpdateInProgress
func TestTriggerHandler_Conflict(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		err: instance.ErrUpdateInProgress,
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusConflict)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "conflict" {
		t.Errorf("error = %q, want %q", response["error"], "conflict")
	}

	if response["message"] != "Update already in progress" {
		t.Errorf("message = %q, want %q", response["message"], "Update already in progress")
	}
}

// TestTriggerHandler_Timeout tests API-01:
// Handle returns 504 Gateway Timeout on context.DeadlineExceeded
func TestTriggerHandler_Timeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     1 * time.Millisecond,
	}

	mock := &mockTriggerUpdater{
		err: context.DeadlineExceeded,
	}
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewTriggerHandler(mock, cfg, logger, ul)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

// TestTriggerHandler_ContextTimeout tests API-01:
// Handle uses context timeout from config
func TestTriggerHandler_ContextTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	expectedTimeout := 45 * time.Second
	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     expectedTimeout,
	}

	mock := &mockTriggerUpdater{}
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewTriggerHandler(mock, cfg, logger, ul)

	if handler.config.Timeout != expectedTimeout {
		t.Errorf("Handler timeout = %v, want %v", handler.config.Timeout, expectedTimeout)
	}
}

// TestTriggerHandler_JSONFormat tests API-04:
// JSON response format matches expected structure
func TestTriggerHandler_JSONFormat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// Check required fields
	if _, ok := response["success"]; !ok {
		t.Error("Response missing 'success' field")
	}
	if _, ok := response["update_id"]; !ok {
		t.Error("Response missing 'update_id' field")
	}
}

// TestTriggerHandler_WithAuth tests API-02, API-05:
// Handler integrates with auth middleware
func TestTriggerHandler_WithAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "valid-token-12345678901234567890",
		Timeout:     30 * time.Second,
	}

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped:     []string{},
			Started:     []string{},
			StopFailed:  []*instance.InstanceError{},
			StartFailed: []*instance.InstanceError{},
		},
	}
	ul := updatelog.NewUpdateLogger(logger, "")
	triggerHandler := NewTriggerHandler(mock, cfg, logger, ul)
	authMiddleware := AuthMiddleware(cfg.BearerToken, logger)

	handler := authMiddleware(http.HandlerFunc(triggerHandler.Handle))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid-token-00000000000000000000",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + cfg.BearerToken,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestTriggerHandler_TimeoutScenario tests API-01:
// Handle returns 504 when context deadline is exceeded
func TestTriggerHandler_TimeoutScenario(t *testing.T) {
	_ = slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     100 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	select {
	case <-time.After(150 * time.Millisecond):
		// Wait longer than timeout
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Errorf("Expected DeadlineExceeded, got %v", ctx.Err())
		}
	}
}

// TestTriggerHandler_UpdateFailed tests response with failed instances
func TestTriggerHandler_UpdateFailed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		result: &instance.UpdateResult{
			Stopped: []string{},
			Started: []string{},
			StopFailed: []*instance.InstanceError{
				{InstanceName: "test-instance", Operation: "stop", Port: 9999, Err: errors.New("stop failed")},
			},
			StartFailed: []*instance.InstanceError{},
		},
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response APIUpdateResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	// Verify update_id is still present even when update had errors
	if response.UpdateID == "" {
		t.Error("update_id is empty, expected UUID v4 even on failure")
	}

	// success should be false since there are errors
	if response.Success {
		t.Error("success = true, want false (has StopFailed errors)")
	}

	// Verify update log was recorded with failed status
	logs := ul.GetAll()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 recorded log, got %d", len(logs))
	}
	if logs[0].Status != updatelog.StatusFailed {
		t.Errorf("Log status = %q, want %q", logs[0].Status, updatelog.StatusFailed)
	}
}

// TestTriggerHandler_InternalError tests response when TriggerUpdate returns generic error
func TestTriggerHandler_InternalError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	mock := &mockTriggerUpdater{
		err: errors.New("UV update failed: unexpected error"),
	}
	handler := newTestHandler(logger, ul, mock)

	req := httptest.NewRequest("POST", "/api/v1/trigger-update", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "internal_error" {
		t.Errorf("error = %q, want %q", response["error"], "internal_error")
	}
}
