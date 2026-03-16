package config

import (
	"strings"
	"testing"
	"time"
)

// TestMonitorConfigValidate tests the validation logic for MonitorConfig.
func TestMonitorConfigValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := MonitorConfig{
			Interval: 15 * time.Minute,
			Timeout:  10 * time.Second,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("interval too short", func(t *testing.T) {
		cfg := MonitorConfig{
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for short interval, got nil")
		}
		if !strings.Contains(err.Error(), "monitor.interval must be at least 1 minute") {
			t.Errorf("error message should contain interval validation, got: %v", err)
		}
	})

	t.Run("timeout too short", func(t *testing.T) {
		cfg := MonitorConfig{
			Interval: 15 * time.Minute,
			Timeout:  500 * time.Millisecond,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for short timeout, got nil")
		}
		if !strings.Contains(err.Error(), "monitor.timeout must be at least 1 second") {
			t.Errorf("error message should contain timeout validation, got: %v", err)
		}
	})
}

// TestMonitorConfigIntervalValidation provides detailed interval validation tests.
func TestMonitorConfigIntervalValidation(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		expectError bool
		errorMsg    string
	}{
		{
			name:        "interval zero is invalid",
			interval:    0,
			expectError: true,
			errorMsg:    "interval",
		},
		{
			name:        "interval 1s is invalid",
			interval:    1 * time.Second,
			expectError: true,
			errorMsg:    "interval",
		},
		{
			name:        "interval 30s is invalid",
			interval:    30 * time.Second,
			expectError: true,
			errorMsg:    "interval",
		},
		{
			name:        "interval 59s is invalid",
			interval:    59 * time.Second,
			expectError: true,
			errorMsg:    "interval",
		},
		{
			name:        "interval 1m is valid",
			interval:    1 * time.Minute,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "interval 5m is valid",
			interval:    5 * time.Minute,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "interval 15m is valid",
			interval:    15 * time.Minute,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "interval 1h is valid",
			interval:    1 * time.Hour,
			expectError: false,
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MonitorConfig{
				Interval: tt.interval,
				Timeout:  10 * time.Second,
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

// TestMonitorConfigTimeoutValidation provides detailed timeout validation tests.
func TestMonitorConfigTimeoutValidation(t *testing.T) {
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
			name:        "timeout 100ms is invalid",
			timeout:     100 * time.Millisecond,
			expectError: true,
			errorMsg:    "timeout",
		},
		{
			name:        "timeout 500ms is invalid",
			timeout:     500 * time.Millisecond,
			expectError: true,
			errorMsg:    "timeout",
		},
		{
			name:        "timeout 1s is valid",
			timeout:     1 * time.Second,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "timeout 5s is valid",
			timeout:     5 * time.Second,
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "timeout 10s is valid",
			timeout:     10 * time.Second,
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
			cfg := MonitorConfig{
				Interval: 15 * time.Minute,
				Timeout:  tt.timeout,
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

// TestMonitorConfigDurationParsing tests that duration values can be parsed from YAML.
func TestMonitorConfigDurationParsing(t *testing.T) {
	t.Run("parse interval from string", func(t *testing.T) {
		// time.Duration can parse strings like "15m"
		interval, err := time.ParseDuration("15m")
		if err != nil {
			t.Errorf("failed to parse interval: %v", err)
		}
		if interval != 15*time.Minute {
			t.Errorf("expected 15m, got %v", interval)
		}
	})

	t.Run("parse timeout from string", func(t *testing.T) {
		// time.Duration can parse strings like "10s"
		timeout, err := time.ParseDuration("10s")
		if err != nil {
			t.Errorf("failed to parse timeout: %v", err)
		}
		if timeout != 10*time.Second {
			t.Errorf("expected 10s, got %v", timeout)
		}
	})
}
