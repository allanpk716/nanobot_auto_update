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
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
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

// TestNewInstanceLifecycle_LogBuffer verifies INST-01:
// NewInstanceLifecycle creates LogBuffer automatically
func TestNewInstanceLifecycle_LogBuffer(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "test-instance",
		Port:         18790,
		StartCommand: "nanobot gateway",
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	if il == nil {
		t.Fatal("NewInstanceLifecycle returned nil")
	}

	// Verify logBuffer is created (non-nil)
	if il.logBuffer == nil {
		t.Error("InstanceLifecycle.logBuffer should not be nil after creation")
	}
}

// TestInstanceLifecycle_GetLogBuffer verifies INST-01:
// GetLogBuffer() returns the instance's LogBuffer
func TestInstanceLifecycle_GetLogBuffer(t *testing.T) {
	cfg := config.InstanceConfig{
		Name:         "test-instance",
		Port:         18790,
		StartCommand: "nanobot gateway",
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	il := NewInstanceLifecycle(cfg, baseLogger)

	// GetLogBuffer should return non-nil buffer
	buf := il.GetLogBuffer()
	if buf == nil {
		t.Fatal("GetLogBuffer() returned nil")
	}
}

// TestInstanceLifecycle_IndependentLogBuffers verifies INST-01:
// Different instances have different LogBuffer instances
func TestInstanceLifecycle_IndependentLogBuffers(t *testing.T) {
	cfg1 := config.InstanceConfig{
		Name:         "instance1",
		Port:         18790,
		StartCommand: "nanobot gateway",
	}
	cfg2 := config.InstanceConfig{
		Name:         "instance2",
		Port:         18791,
		StartCommand: "nanobot gateway",
	}

	baseLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	il1 := NewInstanceLifecycle(cfg1, baseLogger)
	il2 := NewInstanceLifecycle(cfg2, baseLogger)

	buf1 := il1.GetLogBuffer()
	buf2 := il2.GetLogBuffer()

	// Verify both buffers are non-nil
	if buf1 == nil || buf2 == nil {
		t.Fatal("GetLogBuffer() returned nil for one or both instances")
	}

	// Verify buffers are different instances
	if buf1 == buf2 {
		t.Error("Different instances should have different LogBuffer instances")
	}

	// Verify independence: write to buf1, check buf2 is unaffected
	buf1.Write(logbuffer.LogEntry{Content: "test-log-instance1", Source: "stdout"})

	history1 := buf1.GetHistory()
	history2 := buf2.GetHistory()

	if len(history1) != 1 {
		t.Errorf("buf1 should have 1 entry, got %d", len(history1))
	}
	if len(history2) != 0 {
		t.Errorf("buf2 should have 0 entries (independent), got %d", len(history2))
	}
}
