package config

import (
	"strings"
	"testing"
)

// TestServiceConfigValidate tests the validation logic for ServiceConfig.
func TestServiceConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      ServiceConfig
		expectError bool
		errorMatch  string
	}{
		{
			name:        "auto_start nil skips validation",
			config:      ServiceConfig{AutoStart: nil, ServiceName: "", DisplayName: ""},
			expectError: false,
		},
		{
			name:        "auto_start false skips validation",
			config:      ServiceConfig{AutoStart: ptrBool(false), ServiceName: "", DisplayName: ""},
			expectError: false,
		},
		{
			name:        "auto_start true valid names",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "NanobotAutoUpdater", DisplayName: "Nanobot Auto Updater"},
			expectError: false,
		},
		{
			name:        "auto_start true service_name with space",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "My Service", DisplayName: "Valid"},
			expectError: true,
			errorMatch:  "service_name",
		},
		{
			name:        "auto_start true service_name with hyphen",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "My-Service", DisplayName: "Valid"},
			expectError: true,
			errorMatch:  "service_name",
		},
		{
			name:        "auto_start true empty service_name",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "", DisplayName: "Valid"},
			expectError: true,
			errorMatch:  "service_name",
		},
		{
			name:        "auto_start true display_name too long",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "Svc", DisplayName: strings.Repeat("a", 257)},
			expectError: true,
			errorMatch:  "display_name",
		},
		{
			name:        "auto_start true empty display_name",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "Svc", DisplayName: ""},
			expectError: true,
			errorMatch:  "display_name",
		},
		{
			name:        "auto_start true display_name at max 256",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "Svc", DisplayName: strings.Repeat("a", 256)},
			expectError: false,
		},
		{
			name:        "auto_start true service_name letters and digits",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "Service123", DisplayName: "My Service"},
			expectError: false,
		},
		{
			name:        "auto_start true service_name too long 257 chars",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: strings.Repeat("a", 257), DisplayName: "Valid"},
			expectError: true,
			errorMatch:  "service_name",
		},
		{
			name:        "auto_start true service_name at max 256 chars",
			config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: strings.Repeat("a", 256), DisplayName: "Valid"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMatch)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMatch) {
					t.Errorf("expected error containing %q, got %q", tt.errorMatch, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
