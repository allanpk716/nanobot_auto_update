package instance

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// TestNewInstanceManager tests InstanceManager initialization
func TestNewInstanceManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "test1", Port: 8080, StartCommand: "cmd1"},
			{Name: "test2", Port: 8081, StartCommand: "cmd2"},
		},
	}

	manager := NewInstanceManager(cfg, logger)

	if manager == nil {
		t.Fatal("NewInstanceManager returned nil")
	}

	if len(manager.instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(manager.instances))
	}

	if manager.logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestUpdateResultHasErrors tests HasErrors method
func TestUpdateResultHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   UpdateResult
		expected bool
	}{
		{
			name:     "No errors",
			result:   UpdateResult{Stopped: []string{"a"}, Started: []string{"a"}},
			expected: false,
		},
		{
			name:     "Stop failed",
			result:   UpdateResult{StopFailed: []*InstanceError{{InstanceName: "a"}}},
			expected: true,
		},
		{
			name:     "Start failed",
			result:   UpdateResult{StartFailed: []*InstanceError{{InstanceName: "a"}}},
			expected: true,
		},
		{
			name:     "Both failed",
			result:   UpdateResult{StopFailed: []*InstanceError{{InstanceName: "a"}}, StartFailed: []*InstanceError{{InstanceName: "b"}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestUpdateError tests UpdateError error aggregation
func TestUpdateError(t *testing.T) {
	errs := []*InstanceError{
		{InstanceName: "instance1", Operation: "stop", Err: context.Canceled},
		{InstanceName: "instance2", Operation: "start", Err: context.DeadlineExceeded},
	}

	updateErr := &UpdateError{Errors: errs}

	// Test Error() method
	errMsg := updateErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Should contain instance names
	if len(errMsg) < 20 {
		t.Errorf("Error message too short: %s", errMsg)
	}

	// Test Unwrap() method
	unwrapped := updateErr.Unwrap()
	if len(unwrapped) != 2 {
		t.Errorf("Unwrap() returned %d errors, expected 2", len(unwrapped))
	}
}
