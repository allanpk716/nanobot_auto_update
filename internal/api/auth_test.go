package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestAuthMiddleware_MissingHeader tests API-02, API-05:
// AuthMiddleware returns 401 when Authorization header is missing
func TestAuthMiddleware_MissingHeader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	expectedToken := "valid-token-12345678901234567890"

	// Create a test handler that should not be called
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called when auth header is missing")
	})

	// Wrap with auth middleware
	authMiddleware := AuthMiddleware(func() string { return expectedToken }, logger)
	handler := authMiddleware(nextHandler)

	// Create request without Authorization header
	req := httptest.NewRequest("POST", "/api/v1/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Verify JSON body
	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "unauthorized" {
		t.Errorf("error = %q, want %q", response["error"], "unauthorized")
	}

	if response["message"] != "Missing Authorization header" {
		t.Errorf("message = %q, want %q", response["message"], "Missing Authorization header")
	}

	// Verify Content-Type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestAuthMiddleware_InvalidFormat tests API-02, API-05:
// AuthMiddleware returns 401 when Authorization header has wrong format
func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	expectedToken := "valid-token-12345678901234567890"

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "wrong scheme (Basic)",
			authHeader:     "Basic abc123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no space after Bearer",
			authHeader:     "Bearertoken123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Next handler should not be called with invalid auth format")
			})

			authMiddleware := AuthMiddleware(func() string { return expectedToken }, logger)
			handler := authMiddleware(nextHandler)

			req := httptest.NewRequest("POST", "/api/v1/test", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.expectedStatus)
			}

			var response map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response JSON: %v", err)
			}

			if response["error"] != "unauthorized" {
				t.Errorf("error = %q, want %q", response["error"], "unauthorized")
			}
		})
	}
}

// TestAuthMiddleware_InvalidToken tests API-02, API-05:
// AuthMiddleware returns 401 when token does not match
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	expectedToken := "valid-token-12345678901234567890"

	tests := []struct {
		name       string
		authHeader string
	}{
		{
			name:       "wrong token",
			authHeader: "Bearer wrong-token-00000000000000000000",
		},
		{
			name:       "empty token",
			authHeader: "Bearer ",
		},
		{
			name:       "partially matching token",
			authHeader: "Bearer valid-token-1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Next handler should not be called with invalid token")
			})

			authMiddleware := AuthMiddleware(func() string { return expectedToken }, logger)
			handler := authMiddleware(nextHandler)

			req := httptest.NewRequest("POST", "/api/v1/test", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Status code = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			var response map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response JSON: %v", err)
			}

			if response["error"] != "unauthorized" {
				t.Errorf("error = %q, want %q", response["error"], "unauthorized")
			}
		})
	}
}

// TestAuthMiddleware_ValidToken tests API-02:
// AuthMiddleware calls next handler when token matches
func TestAuthMiddleware_ValidToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	expectedToken := "valid-token-12345678901234567890"

	// Create a test handler that sets a custom header to verify it was called
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.Header().Set("X-Custom-Header", "called")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	authMiddleware := AuthMiddleware(func() string { return expectedToken }, logger)
	handler := authMiddleware(nextHandler)

	req := httptest.NewRequest("POST", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+expectedToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify next handler was called
	if !nextCalled {
		t.Error("Next handler should be called with valid token")
	}

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Header().Get("X-Custom-Header") != "called" {
		t.Error("Custom header not set by next handler")
	}
}

// TestAuthMiddleware_ConstantTimeComparison verifies API-05:
// Implementation uses subtle.ConstantTimeCompare to prevent timing attacks
func TestAuthMiddleware_ConstantTimeComparison(t *testing.T) {
	// This test verifies that the implementation uses subtle.ConstantTimeCompare
	// We do this by inspecting the source code to ensure it's using the correct function

	// Read the auth.go source file
	sourceCode, err := os.ReadFile("auth.go")
	if err != nil {
		t.Skip("Cannot read auth.go source file for inspection")
	}

	sourceStr := string(sourceCode)

	// Verify that subtle.ConstantTimeCompare is used
	if !strings.Contains(sourceStr, "subtle.ConstantTimeCompare") {
		t.Error("Implementation should use subtle.ConstantTimeCompare for token comparison")
	}

	// Verify that subtle package is imported
	if !strings.Contains(sourceStr, `"crypto/subtle"`) {
		t.Error("Implementation should import crypto/subtle package")
	}
}

// TestWriteJSONError tests API-05:
// writeJSONError returns correct JSON format with Content-Type header
func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		errorCode   string
		message     string
	}{
		{
			name:      "unauthorized error",
			status:    http.StatusUnauthorized,
			errorCode: "unauthorized",
			message:   "Missing Authorization header",
		},
		{
			name:      "conflict error",
			status:    http.StatusConflict,
			errorCode: "conflict",
			message:   "Update already in progress",
		},
		{
			name:      "timeout error",
			status:    http.StatusGatewayTimeout,
			errorCode: "timeout",
			message:   "Update operation timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeJSONError(rec, tt.status, tt.errorCode, tt.message)

			// Verify status code
			if rec.Code != tt.status {
				t.Errorf("Status code = %d, want %d", rec.Code, tt.status)
			}

			// Verify Content-Type
			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
			}

			// Verify JSON body
			var response map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response JSON: %v (body: %q)", err, rec.Body.String())
			}

			if response["error"] != tt.errorCode {
				t.Errorf("error = %q, want %q", response["error"], tt.errorCode)
			}

			if response["message"] != tt.message {
				t.Errorf("message = %q, want %q", response["message"], tt.message)
			}
		})
	}
}
