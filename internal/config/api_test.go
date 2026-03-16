package config

import (
	"strings"
	"testing"
	"time"
)

// TestAPIConfigValidate tests the validation logic for APIConfig.
func TestAPIConfigValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := APIConfig{
			Port:        8080,
			BearerToken: "this-is-a-secure-token-with-at-least-32-chars",
			Timeout:     30 * time.Second,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("invalid port zero", func(t *testing.T) {
		cfg := APIConfig{
			Port:        0,
			BearerToken: "this-is-a-secure-token-with-at-least-32-chars",
			Timeout:     30 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for invalid port, got nil")
		}
		if !strings.Contains(err.Error(), "api.port must be between 1 and 65535") {
			t.Errorf("error message should contain port validation, got: %v", err)
		}
	})

	t.Run("invalid port too large", func(t *testing.T) {
		cfg := APIConfig{
			Port:        65536,
			BearerToken: "this-is-a-secure-token-with-at-least-32-chars",
			Timeout:     30 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for invalid port, got nil")
		}
		if !strings.Contains(err.Error(), "api.port must be between 1 and 65535") {
			t.Errorf("error message should contain port validation, got: %v", err)
		}
	})

	t.Run("bearer token too short", func(t *testing.T) {
		cfg := APIConfig{
			Port:        8080,
			BearerToken: "too-short-token-31-chars-xxxx",
			Timeout:     30 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for short token, got nil")
		}
		if !strings.Contains(err.Error(), "api.bearer_token must be at least 32 characters for security") {
			t.Errorf("error message should contain token length validation, got: %v", err)
		}
	})

	t.Run("timeout too short", func(t *testing.T) {
		cfg := APIConfig{
			Port:        8080,
			BearerToken: "this-is-a-secure-token-with-at-least-32-chars",
			Timeout:     4 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for short timeout, got nil")
		}
		if !strings.Contains(err.Error(), "api.timeout must be at least 5 seconds") {
			t.Errorf("error message should contain timeout validation, got: %v", err)
		}
	})

	t.Run("empty bearer token fails validation", func(t *testing.T) {
		cfg := APIConfig{
			Port:        8080,
			BearerToken: "",
			Timeout:     30 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for empty token, got nil")
		}
		if !strings.Contains(err.Error(), "api.bearer_token must be at least 32 characters for security") {
			t.Errorf("error message should contain token length validation, got: %v", err)
		}
	})
}

// TestAPIConfigPortValidation provides detailed port validation tests.
func TestAPIConfigPortValidation(t *testing.T) {
	tests := []struct {
		name        string
		port        uint32
		expectError bool
		errorMsg    string
	}{
		{
			name:        "port zero is invalid",
			port:        0,
			expectError: true,
			errorMsg:    "port",
		},
		{
			name:        "port 1 is valid",
			port:        1,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "port 8080 is valid",
			port:        8080,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "port 65535 is valid",
			port:        65535,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "port 65536 is invalid",
			port:        65536,
			expectError: true,
			errorMsg:    "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := APIConfig{
				Port:        tt.port,
				BearerToken: "valid-token-with-at-least-32-characters",
				Timeout:     30 * time.Second,
			}
			err := cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestAPIConfigBearerTokenValidation provides detailed Bearer Token validation tests (SEC-03).
func TestAPIConfigBearerTokenValidation(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty token is invalid",
			bearerToken: "",
			expectError: true,
			errorMsg:    "bearer_token",
		},
		{
			name:        "token with 1 char is invalid",
			bearerToken: "a",
			expectError: true,
			errorMsg:    "bearer_token",
		},
		{
			name:        "token with 31 chars is invalid",
			bearerToken: "1234567890123456789012345678901",
			expectError: true,
			errorMsg:    "bearer_token",
		},
		{
			name:        "token with 32 chars is valid",
			bearerToken: "12345678901234567890123456789012",
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "token with 64 chars is valid",
			bearerToken: "1234567890123456789012345678901234567890123456789012345678901234",
			expectError: false,
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := APIConfig{
				Port:        8080,
				BearerToken: tt.bearerToken,
				Timeout:     30 * time.Second,
			}
			err := cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestAPIConfigTimeoutValidation provides detailed timeout validation tests.
func TestAPIConfigTimeoutValidation(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name:        "timeout zero is invalid",
			timeout:     0,
			expectError: true,
			errorMsg:    "timeout",
		},
		{
			name:        "timeout 1s is invalid",
			timeout:     1 * time.Second,
			expectError: true,
			errorMsg:    "timeout",
		},
		{
			name:        "timeout 4s is invalid",
			timeout:     4 * time.Second,
			expectError: true,
			errorMsg:    "timeout",
		},
		{
			name:        "timeout 5s is valid",
			timeout:     5 * time.Second,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "timeout 30s is valid",
			timeout:     30 * time.Second,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "timeout 1m is valid",
			timeout:     1 * time.Minute,
			expectError: false,
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := APIConfig{
				Port:        8080,
				BearerToken: "valid-token-with-at-least-32-characters",
				Timeout:     tt.timeout,
			}
			err := cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
