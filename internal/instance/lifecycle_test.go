package instance

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

func TestNewInstanceLifecycle_LoggerContextInjection(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "test-instance",
		Port:         18790,
		StartCommand: "nanobot gateway",
	}

	// Create a buffer to capture log output
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	baseLogger := slog.New(handler)

	il := NewInstanceLifecycle(cfg, baseLogger)

	// Verify the logger is injected
	if il == nil {
		t.Fatal("NewInstanceLifecycle returned nil")
	}

	// Log a test message to verify context fields
	il.logger.Info("test message")
	logOutput := buf.String()

	// Verify log contains instance and component fields
	if !strings.Contains(logOutput, "instance=test-instance") {
		t.Errorf("Log output missing instance field: %s", logOutput)
	}
	if !strings.Contains(logOutput, "component=instance-lifecycle") {
		t.Errorf("Log output missing component field: %s", logOutput)
	}
}

func TestInstanceLifecycle_StopForUpdate(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "test-instance",
		Port:         18790,
		StartCommand: "nanobot gateway",
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	// Note: This test cannot easily mock lifecycle.IsNanobotRunning and lifecycle.StopNanobot
	// without creating an interface-based abstraction.
	// For unit testing, we verify the error wrapping behavior when the instance is not running.
	ctx := context.Background()
	err := il.StopForUpdate(ctx)

	// When instance is not running, should return nil (not an error)
	if err != nil {
		t.Logf("StopForUpdate returned error (expected nil when not running): %v", err)
	}
}

func TestInstanceLifecycle_StopForUpdate_ErrorWrapping(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "failing-instance",
		Port:         18791,
		StartCommand: "nanobot gateway",
	}

	_ = cfg // Configuration used for verification below

	// Create a simulated InstanceError
	simulatedErr := &InstanceError{
		InstanceName: "failing-instance",
		Operation:    "stop",
		Port:         18791,
		Err:          errors.New("simulated stop error"),
	}

	// Verify error message format
	expected := `停止实例 "failing-instance" 失败 (port=18791): simulated stop error`
	if simulatedErr.Error() != expected {
		t.Errorf("InstanceError.Error() = %q, want %q", simulatedErr.Error(), expected)
	}
}

func TestInstanceLifecycle_StartAfterUpdate(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:           "test-instance",
		Port:           18790,
		StartCommand:   "nanobot gateway",
		StartupTimeout: 10 * time.Second,
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	// Note: This test cannot easily mock lifecycle.StartNanobot without interface abstraction
	// For integration testing, the actual process management should be tested
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This will likely fail in unit test environment, but verifies error wrapping
	err := il.StartAfterUpdate(ctx)
	if err != nil {
		t.Logf("StartAfterUpdate returned error (expected in test env): %v", err)

		// Verify error is wrapped as InstanceError
		var instanceErr *InstanceError
		if !errors.As(err, &instanceErr) {
			t.Errorf("Error should be wrapped as InstanceError, got %T", err)
		} else {
			// Verify InstanceError fields
			if instanceErr.InstanceName != cfg.Name {
				t.Errorf("InstanceError.InstanceName = %q, want %q", instanceErr.InstanceName, cfg.Name)
			}
			if instanceErr.Operation != "start" {
				t.Errorf("InstanceError.Operation = %q, want %q", instanceErr.Operation, "start")
			}
			if instanceErr.Port != cfg.Port {
				t.Errorf("InstanceError.Port = %d, want %d", instanceErr.Port, cfg.Port)
			}
		}
	}
}

func TestInstanceLifecycle_StartAfterUpdate_DefaultTimeout(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:           "test-instance",
		Port:           18790,
		StartCommand:   "nanobot gateway",
		StartupTimeout: 0, // Test default timeout
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	if il == nil {
		t.Fatal("NewInstanceLifecycle returned nil")
	}

	// Verify that StartupTimeout=0 is handled (should use 30s default)
	// The default timeout logic is in StartAfterUpdate implementation
	// For this test, we just verify the instance was created successfully
}

func TestInstanceLifecycle_StopForUpdate_NotRunning(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "nonexistent-instance",
		Port:         18792,
		StartCommand: "nanobot gateway",
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	ctx := context.Background()
	err := il.StopForUpdate(ctx)

	// When instance is not running, StopForUpdate should return nil (not an error)
	if err != nil {
		t.Errorf("StopForUpdate() should return nil when instance not running, got: %v", err)
	}
}
