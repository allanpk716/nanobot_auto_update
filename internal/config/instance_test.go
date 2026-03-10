package config

import (
	"testing"
	"time"
)

func TestInstanceConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		instance    InstanceConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty name",
			instance: InstanceConfig{
				Name:         "",
				Port:         18790,
				StartCommand: "nanobot.exe",
			},
			expectError: true,
			errorMsg:    "缺少必填字段 \"name\"",
		},
		{
			name: "port zero",
			instance: InstanceConfig{
				Name:         "test-instance",
				Port:         0,
				StartCommand: "nanobot.exe",
			},
			expectError: true,
			errorMsg:    "端口必须在 1-65535 范围内",
		},
		{
			name: "port too large",
			instance: InstanceConfig{
				Name:         "test-instance",
				Port:         65536,
				StartCommand: "nanobot.exe",
			},
			expectError: true,
			errorMsg:    "端口必须在 1-65535 范围内",
		},
		{
			name: "empty start_command",
			instance: InstanceConfig{
				Name:         "test-instance",
				Port:         18790,
				StartCommand: "",
			},
			expectError: true,
			errorMsg:    "缺少必填字段 \"start_command\"",
		},
		{
			name: "startup_timeout less than 5s",
			instance: InstanceConfig{
				Name:           "test-instance",
				Port:           18790,
				StartCommand:   "nanobot.exe",
				StartupTimeout: 3 * time.Second,
			},
			expectError: true,
			errorMsg:    "startup_timeout 必须至少 5 秒",
		},
		{
			name: "valid config with all fields",
			instance: InstanceConfig{
				Name:           "test-instance",
				Port:           18790,
				StartCommand:   "nanobot.exe",
				StartupTimeout: 30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "valid config with zero startup_timeout",
			instance: InstanceConfig{
				Name:         "test-instance",
				Port:         18790,
				StartCommand: "nanobot.exe",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instance.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
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

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
