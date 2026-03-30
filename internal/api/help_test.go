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
)

// TestHelpHandler_Success tests HELP-01, HELP-02, HELP-03:
// - HELP-01: GET /api/v1/help returns 200 OK
// - HELP-02: No Authorization header required
// - HELP-03: Response contains version, endpoints, config, cli_flags
func TestHelpHandler_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		API: config.APIConfig{
			Port:        8080,
			BearerToken: "test-token",
			Timeout:     30 * time.Second,
		},
		Monitor: config.MonitorConfig{
			Interval: 15 * time.Minute,
			Timeout:  10 * time.Second,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval: 1 * time.Minute,
		},
	}

	handler := NewHelpHandler("v0.5", cfg, logger)

	// Create GET request WITHOUT Authorization header (HELP-02)
	req := httptest.NewRequest("GET", "/api/v1/help", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// HELP-01: Verify 200 OK (will FAIL with stub returning 501)
	if rec.Code != http.StatusOK {
		t.Errorf("HELP-01: Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// HELP-03: Verify JSON response structure
	var response HelpResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("HELP-03: Failed to decode response JSON: %v", err)
	}

	// HELP-03: Verify required fields
	if response.Version == "" {
		t.Error("HELP-03: Version field is empty")
	}
	if response.Architecture == "" {
		t.Error("HELP-03: Architecture field is empty")
	}
	if len(response.Endpoints) == 0 {
		t.Error("HELP-03: Endpoints field is empty")
	}
	if response.Config.APIPort == 0 {
		t.Error("HELP-03: Config.APIPort is zero")
	}
	if len(response.CLIFlags) == 0 {
		t.Error("HELP-03: CLIFlags field is empty")
	}

	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestHelpHandler_MethodNotAllowed tests HELP-01:
// Returns 405 for POST request
func TestHelpHandler_MethodNotAllowed(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{}
	handler := NewHelpHandler("v0.5", cfg, logger)

	// Create POST request
	req := httptest.NewRequest("POST", "/api/v1/help", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// HELP-01: Verify 405 Method Not Allowed (will FAIL with stub returning 501)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("HELP-01: Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// TestHelpHandler_SelfUpdateEndpoints tests API-04:
// Help response includes self_update_check and self_update endpoints
func TestHelpHandler_SelfUpdateEndpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		API: config.APIConfig{
			Port:        8080,
			BearerToken: "test-token",
			Timeout:     30 * time.Second,
		},
		Monitor: config.MonitorConfig{
			Interval: 15 * time.Minute,
			Timeout:  10 * time.Second,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval: 1 * time.Minute,
		},
	}

	handler := NewHelpHandler("v0.8", cfg, logger)

	req := httptest.NewRequest("GET", "/api/v1/help", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var response HelpResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify self_update_check endpoint (API-04)
	check, ok := response.Endpoints["self_update_check"]
	if !ok {
		t.Fatal("self_update_check endpoint not found in help response")
	}
	if check.Method != "GET" {
		t.Errorf("self_update_check.Method = %q, want %q", check.Method, "GET")
	}
	if check.Path != "/api/v1/self-update/check" {
		t.Errorf("self_update_check.Path = %q, want %q", check.Path, "/api/v1/self-update/check")
	}
	if check.Auth != "required" {
		t.Errorf("self_update_check.Auth = %q, want %q", check.Auth, "required")
	}

	// Verify self_update endpoint (API-04)
	update, ok := response.Endpoints["self_update"]
	if !ok {
		t.Fatal("self_update endpoint not found in help response")
	}
	if update.Method != "POST" {
		t.Errorf("self_update.Method = %q, want %q", update.Method, "POST")
	}
	if update.Path != "/api/v1/self-update" {
		t.Errorf("self_update.Path = %q, want %q", update.Path, "/api/v1/self-update")
	}
	if update.Auth != "required" {
		t.Errorf("self_update.Auth = %q, want %q", update.Auth, "required")
	}
}
