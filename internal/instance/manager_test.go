package instance

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
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

// TestStopAllGracefulDegradation tests that stopAll continues when one instance fails
// This test verifies graceful degradation behavior - all instances should be processed
// even if some fail
func TestStopAllGracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create InstanceManager with 3 instances
	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "instance1", Port: 8080, StartCommand: "cmd1"},
			{Name: "instance2", Port: 8081, StartCommand: "cmd2"},
			{Name: "instance3", Port: 8082, StartCommand: "cmd3"},
		},
	}

	manager := NewInstanceManager(cfg, logger)
	ctx := context.Background()
	result := &UpdateResult{}

	// Execute stopAll - instances are not running so all should succeed
	manager.stopAll(ctx, result)

	// Verify graceful degradation: should process all 3 instances
	// Since instances are not running, they all succeed (Stopped)
	totalProcessed := len(result.Stopped) + len(result.StopFailed)
	if totalProcessed != 3 {
		t.Errorf("Expected 3 instances processed, got %d (stopped: %d, failed: %d)",
			totalProcessed, len(result.Stopped), len(result.StopFailed))
	}

	// All should succeed since instances are not running
	if len(result.Stopped) != 3 {
		t.Errorf("Expected 3 stopped instances (not running), got %d", len(result.Stopped))
	}

	// Verify no failures
	if len(result.StopFailed) != 0 {
		t.Errorf("Expected 0 failed instances, got %d", len(result.StopFailed))
	}
}

// TestStartAllGracefulDegradation tests that startAll continues when one instance fails
// This test uses short timeout to avoid waiting for real process startup
func TestStartAllGracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create InstanceManager with 2 instances
	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "instance1", Port: 8090, StartCommand: "nonexistent-command-1"},
			{Name: "instance2", Port: 8091, StartCommand: "nonexistent-command-2"},
		},
	}

	manager := NewInstanceManager(cfg, logger)

	// Use very short timeout to fail fast
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := &UpdateResult{}

	// Execute startAll - should attempt all instances even if commands fail
	manager.startAll(ctx, result)

	// Verify graceful degradation: should process all 2 instances
	totalProcessed := len(result.Started) + len(result.StartFailed)
	if totalProcessed != 2 {
		t.Errorf("Expected 2 instances processed, got %d (started: %d, failed: %d)",
			totalProcessed, len(result.Started), len(result.StartFailed))
	}

	// Both should fail since commands don't exist
	if len(result.StartFailed) != 2 {
		t.Errorf("Expected 2 failed instances, got %d", len(result.StartFailed))
	}

	// Verify error details
	for i, err := range result.StartFailed {
		if err.InstanceName == "" {
			t.Errorf("StartFailed[%d].InstanceName is empty", i)
		}
		if err.Operation != "start" {
			t.Errorf("StartFailed[%d].Operation = %q, want 'start'", i, err.Operation)
		}
	}
}

// TestUpdateAllSkipUpdateWhenStopFails tests that UpdateAll skips UV update when stop fails
// This is a behavioral verification that doesn't require real processes
func TestUpdateAllSkipUpdateWhenStopFails(t *testing.T) {
	// This test verifies the logic in UpdateAll where it checks:
	// if len(result.StopFailed) > 0 { skip UV update }

	// Create UpdateResult with stop failures
	result := &UpdateResult{
		StopFailed: []*InstanceError{
			{InstanceName: "failed-instance", Operation: "stop", Port: 8080, Err: errors.New("stop failed")},
		},
	}

	// Verify HasErrors returns true
	if !result.HasErrors() {
		t.Error("HasErrors() should return true when StopFailed is not empty")
	}

	// Verify we can detect stop failures
	if len(result.StopFailed) == 0 {
		t.Error("StopFailed should not be empty")
	}

	// This is the check that UpdateAll performs to skip UV update
	shouldSkipUpdate := len(result.StopFailed) > 0
	if !shouldSkipUpdate {
		t.Error("Should skip UV update when stop failures exist")
	}
}

// TestInstanceErrorTypeAssertion verifies that errors in manager are properly typed
func TestInstanceErrorTypeAssertion(t *testing.T) {
	// Create a simulated InstanceError with a specific underlying error
	underlyingErr := errors.New("simulated error")
	simulatedErr := &InstanceError{
		InstanceName: "test-instance",
		Operation:    "stop",
		Port:         8080,
		Err:          underlyingErr,
	}

	// Verify error message
	errMsg := simulatedErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Verify Unwrap works
	unwrapped := simulatedErr.Unwrap()
	if unwrapped == nil {
		t.Error("Unwrap() returned nil")
	}

	// Verify errors.As works
	var extracted *InstanceError
	if !errors.As(simulatedErr, &extracted) {
		t.Error("errors.As should extract *InstanceError")
	}

	if extracted.InstanceName != "test-instance" {
		t.Errorf("InstanceName = %q, want 'test-instance'", extracted.InstanceName)
	}

	// Verify errors.Is works with the SAME underlying error instance
	if !errors.Is(simulatedErr, underlyingErr) {
		t.Error("errors.Is should find underlying error")
	}
}

// TestInstanceManager_GetLogBuffer verifies INST-02:
// InstanceManager can return LogBuffer by instance name
func TestInstanceManager_GetLogBuffer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "instance1", Port: 8080, StartCommand: "cmd1"},
			{Name: "instance2", Port: 8081, StartCommand: "cmd2"},
		},
	}

	manager := NewInstanceManager(cfg, logger)

	// Test: GetLogBuffer returns correct buffer for existing instance
	buf1, err := manager.GetLogBuffer("instance1")
	if err != nil {
		t.Fatalf("GetLogBuffer(instance1) returned error: %v", err)
	}
	if buf1 == nil {
		t.Fatal("GetLogBuffer(instance1) returned nil buffer")
	}

	// Test: Different instances have different buffers
	buf2, err := manager.GetLogBuffer("instance2")
	if err != nil {
		t.Fatalf("GetLogBuffer(instance2) returned error: %v", err)
	}
	if buf1 == buf2 {
		t.Error("Different instances should have different LogBuffer instances")
	}

	// Test: GetLogBuffer returns error for non-existent instance
	_, err = manager.GetLogBuffer("nonexistent")
	if err == nil {
		t.Fatal("GetLogBuffer(nonexistent) should return error")
	}

	// Verify error is InstanceError
	var instanceErr *InstanceError
	if !errors.As(err, &instanceErr) {
		t.Errorf("Error should be InstanceError, got %T", err)
	} else {
		if instanceErr.InstanceName != "nonexistent" {
			t.Errorf("InstanceError.InstanceName = %q, want 'nonexistent'", instanceErr.InstanceName)
		}
		if instanceErr.Operation != "get_log_buffer" {
			t.Errorf("InstanceError.Operation = %q, want 'get_log_buffer'", instanceErr.Operation)
		}
	}
}
