package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
)

func newTestQueryHandler(ul *updatelog.UpdateLogger) *QueryHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	return NewQueryHandler(ul, logger)
}

func newTestQueryAuthMiddleware() func(http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.APIConfig{
		Port:        8080,
		BearerToken: "test-token-12345678901234567890",
		Timeout:     30 * time.Second,
	}
	return AuthMiddleware(func() string { return cfg.BearerToken }, logger)
}

// Test 1: GET /api/v1/update-logs returns 200 with empty data when no logs exist
func TestQueryHandler_EmptyData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewQueryHandler(ul, logger)

	req := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Data == nil || len(response.Data) != 0 {
		t.Errorf("Expected empty data array, got %v", response.Data)
	}
	if response.Meta.Total != 0 {
		t.Errorf("Expected meta.total=0, got %d", response.Meta.Total)
	}
	if response.Meta.Offset != 0 {
		t.Errorf("Expected meta.offset=0, got %d", response.Meta.Offset)
	}
	if response.Meta.Limit != 20 {
		t.Errorf("Expected meta.limit=20, got %d", response.Meta.Limit)
	}
}

// Test 2: GET /api/v1/update-logs with pre-recorded logs returns data with correct total
func TestQueryHandler_WithData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	baseTime := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ul.Record(updatelog.UpdateLog{
			ID:          "query-uuid-" + string(rune('A'+i)),
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      updatelog.StatusSuccess,
			Instances:   []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		})
	}

	handler := NewQueryHandler(ul, logger)
	req := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Meta.Total != 5 {
		t.Errorf("Expected meta.total=5, got %d", response.Meta.Total)
	}
	if len(response.Data) != 5 {
		t.Errorf("Expected 5 data items, got %d", len(response.Data))
	}
}

// Test 3 & 4: Auth tests - no token and invalid token
func TestQueryHandler_AuthRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewQueryHandler(ul, logger)
	authMiddleware := newTestQueryAuthMiddleware()

	wrappedHandler := authMiddleware(http.HandlerFunc(handler.Handle))

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "no auth header returns 401",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token returns 401",
			authHeader:     "Bearer invalid-token-00000000000000000000",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "valid token returns 200",
			authHeader:     "Bearer test-token-12345678901234567890",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// Test 6: limit parameter controls page size
func TestQueryHandler_LimitParam(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	baseTime := time.Now().UTC()
	for i := 0; i < 10; i++ {
		ul.Record(updatelog.UpdateLog{
			ID:          "limit-uuid-" + string(rune('A'+i)),
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      updatelog.StatusSuccess,
			Instances:   []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		})
	}

	handler := NewQueryHandler(ul, logger)
	req := httptest.NewRequest("GET", "/api/v1/update-logs?limit=5", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Data) != 5 {
		t.Errorf("Expected 5 data items with limit=5, got %d", len(response.Data))
	}
	if response.Meta.Limit != 5 {
		t.Errorf("Expected meta.limit=5, got %d", response.Meta.Limit)
	}
}

// Test 7: limit capped at 100
func TestQueryHandler_LimitCappedAt100(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewQueryHandler(ul, logger)

	req := httptest.NewRequest("GET", "/api/v1/update-logs?limit=200", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Meta.Limit != 100 {
		t.Errorf("Expected meta.limit=100 (capped), got %d", response.Meta.Limit)
	}
}

// Test 8: offset parameter skips records
func TestQueryHandler_OffsetParam(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	baseTime := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ul.Record(updatelog.UpdateLog{
			ID:          "offset-uuid-" + string(rune('A'+i)),
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      updatelog.StatusSuccess,
			Instances:   []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		})
	}

	handler := NewQueryHandler(ul, logger)
	req := httptest.NewRequest("GET", "/api/v1/update-logs?offset=3&limit=2", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Meta.Total != 5 {
		t.Errorf("Expected meta.total=5, got %d", response.Meta.Total)
	}
	if response.Meta.Offset != 3 {
		t.Errorf("Expected meta.offset=3, got %d", response.Meta.Offset)
	}
	// 5 total - offset 3 = 2 remaining
	if len(response.Data) != 2 {
		t.Errorf("Expected 2 data items, got %d", len(response.Data))
	}
}

// Test 9: offset beyond total returns empty data
func TestQueryHandler_OffsetBeyondTotal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	baseTime := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ul.Record(updatelog.UpdateLog{
			ID:          "beyond-uuid-" + string(rune('A'+i)),
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      updatelog.StatusSuccess,
			Instances:   []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		})
	}

	handler := NewQueryHandler(ul, logger)
	req := httptest.NewRequest("GET", "/api/v1/update-logs?offset=99999", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Meta.Total != 3 {
		t.Errorf("Expected meta.total=3, got %d", response.Meta.Total)
	}
	if response.Meta.Offset != 99999 {
		t.Errorf("Expected meta.offset=99999, got %d", response.Meta.Offset)
	}
	if len(response.Data) != 0 {
		t.Errorf("Expected 0 data items, got %d", len(response.Data))
	}
}

// Test 10 & 11: non-numeric limit/offset use defaults
func TestQueryHandler_NonNumericParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewQueryHandler(ul, logger)

	tests := []struct {
		name          string
		url           string
		expectLimit   int
		expectOffset  int
	}{
		{
			name:         "non-numeric limit uses default 20",
			url:          "/api/v1/update-logs?limit=abc",
			expectLimit:  20,
			expectOffset: 0,
		},
		{
			name:         "non-numeric offset uses default 0",
			url:          "/api/v1/update-logs?offset=abc",
			expectLimit:  20,
			expectOffset: 0,
		},
		{
			name:         "negative limit uses default 20",
			url:          "/api/v1/update-logs?limit=-1",
			expectLimit:  20,
			expectOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			rec := httptest.NewRecorder()
			handler.Handle(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
			}

			var response UpdateLogsResponse
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Meta.Limit != tt.expectLimit {
				t.Errorf("Expected meta.limit=%d, got %d", tt.expectLimit, response.Meta.Limit)
			}
			if response.Meta.Offset != tt.expectOffset {
				t.Errorf("Expected meta.offset=%d, got %d", tt.expectOffset, response.Meta.Offset)
			}
		})
	}
}

// Test 13: POST method returns 405
func TestQueryHandler_MethodNotAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")
	handler := NewQueryHandler(ul, logger)

	req := httptest.NewRequest("POST", "/api/v1/update-logs", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["error"] != "method_not_allowed" {
		t.Errorf("Expected error='method_not_allowed', got '%s'", response["error"])
	}
}

// Test 14: nil updateLogger returns 500
func TestQueryHandler_NilUpdateLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewQueryHandler(nil, logger)

	req := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// Test 15: Data is newest-first
func TestQueryHandler_NewestFirst(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ul := updatelog.NewUpdateLogger(logger, "")

	baseTime := time.Now().UTC()
	ids := []string{"oldest-uuid", "middle-uuid", "newest-uuid"}
	for i, id := range ids {
		ul.Record(updatelog.UpdateLog{
			ID:          id,
			StartTime:   baseTime.Add(time.Duration(i) * time.Minute),
			EndTime:     baseTime.Add(time.Duration(i)*time.Minute + 5*time.Second),
			Duration:    5000,
			Status:      updatelog.StatusSuccess,
			Instances:   []updatelog.InstanceUpdateDetail{},
			TriggeredBy: "api-trigger",
		})
	}

	handler := NewQueryHandler(ul, logger)
	req := httptest.NewRequest("GET", "/api/v1/update-logs", nil)
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	var response UpdateLogsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Data) != 3 {
		t.Fatalf("Expected 3 data items, got %d", len(response.Data))
	}
	// Newest first: data[0] should be "newest-uuid"
	if response.Data[0].ID != "newest-uuid" {
		t.Errorf("Expected data[0].ID='newest-uuid', got '%s'", response.Data[0].ID)
	}
	if response.Data[1].ID != "middle-uuid" {
		t.Errorf("Expected data[1].ID='middle-uuid', got '%s'", response.Data[1].ID)
	}
	if response.Data[2].ID != "oldest-uuid" {
		t.Errorf("Expected data[2].ID='oldest-uuid', got '%s'", response.Data[2].ID)
	}
}
