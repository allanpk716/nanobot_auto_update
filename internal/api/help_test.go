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
