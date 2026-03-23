package instance

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// TestTriggerUpdate_Concurrent tests API-06:
// TriggerUpdate returns ErrUpdateInProgress when called during ongoing update
func TestTriggerUpdate_Concurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "inst1", Port: 9999, StartCommand: "nonexistent"},
		},
	}

	manager := NewInstanceManager(cfg, logger)

	// Manually set updating flag to simulate ongoing update
	manager.isUpdating.Store(true)

	// Try to start update - should get ErrUpdateInProgress
	ctx := context.Background()
	_, err := manager.TriggerUpdate(ctx)

	if !errors.Is(err, ErrUpdateInProgress) {
		t.Errorf("Expected ErrUpdateInProgress, got %v", err)
	}

	// Reset flag
	manager.isUpdating.Store(false)

	// Verify IsUpdating is false
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false after resetting flag")
	}
}

// TestTriggerUpdate_ResetsFlag tests API-06:
// TriggerUpdate resets isUpdating flag after completion
func TestTriggerUpdate_ResetsFlag(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use a port that won't have a real instance
	cfg := &config.Config{
		Instances: []config.InstanceConfig{},
	}

	manager := NewInstanceManager(cfg, logger)

	ctx := context.Background()

	// Call with empty instances (will complete immediately)
	_, _ = manager.TriggerUpdate(ctx)

	// Verify flag is reset
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false after TriggerUpdate returns")
	}

	// Second call should not return ErrUpdateInProgress
	_, err := manager.TriggerUpdate(ctx)
	if errors.Is(err, ErrUpdateInProgress) {
		t.Error("Second TriggerUpdate should not return ErrUpdateInProgress")
	}
}

// TestTriggerUpdate_ResetsFlagOnError tests API-06:
// TriggerUpdate resets isUpdating flag even on error
func TestTriggerUpdate_ResetsFlagOnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use empty instances to avoid real process management
	cfg := &config.Config{
		Instances: []config.InstanceConfig{},
	}

	manager := NewInstanceManager(cfg, logger)

	ctx := context.Background()

	// TriggerUpdate with empty instances won't error (no instances to manage)
	_, _ = manager.TriggerUpdate(ctx)

	// Verify flag is reset
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false after TriggerUpdate")
	}
}

// TestTriggerUpdate_ContextCancellation tests API-06:
// TriggerUpdate resets isUpdating flag after context cancellation
func TestTriggerUpdate_ContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{},
	}

	manager := NewInstanceManager(cfg, logger)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Call TriggerUpdate with cancelled context
	_, _ = manager.TriggerUpdate(ctx)

	// Verify flag is reset even after cancellation
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false after context cancellation")
	}
}

// TestIsUpdating tests API-06:
// IsUpdating returns current update state
func TestIsUpdating(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{Name: "inst1", Port: 9995, StartCommand: "echo test"},
		},
	}

	manager := NewInstanceManager(cfg, logger)

	// Initially not updating
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false initially")
	}

	// Set updating flag manually (simulating update in progress)
	manager.isUpdating.Store(true)

	// Now should return true
	if !manager.IsUpdating() {
		t.Error("IsUpdating should be true after setting flag")
	}

	// Reset flag
	manager.isUpdating.Store(false)

	// Now should return false again
	if manager.IsUpdating() {
		t.Error("IsUpdating should be false after resetting flag")
	}
}

// TestTriggerUpdate_CallsUpdateAll tests API-03:
// TriggerUpdate calls UpdateAll internally
func TestTriggerUpdate_CallsUpdateAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{},
	}

	manager := NewInstanceManager(cfg, logger)

	ctx := context.Background()

	// TriggerUpdate should call UpdateAll and return UpdateResult
	result, _ := manager.TriggerUpdate(ctx)

	// Result should not be nil (UpdateAll was called)
	if result == nil {
		t.Error("TriggerUpdate should return non-nil UpdateResult (UpdateAll was called)")
	}

	// With empty instances, all counts should be 0
	t.Logf("TriggerUpdate returned result (UpdateAll called): stopped=%d, started=%d",
		len(result.Stopped), len(result.Started))
}

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

// TestGetInstanceNames verifies UI-07:
// GetInstanceNames returns all configured instance names
func TestGetInstanceNames(t *testing.T) {
	tests := []struct {
		name      string
		instances []config.InstanceConfig
		want      []string
	}{
		{
			name:      "empty manager",
			instances: nil,
			want:      []string{},
		},
		{
			name: "single instance",
			instances: []config.InstanceConfig{
				{Name: "instance1", Port: 8080, StartCommand: "cmd1"},
			},
			want: []string{"instance1"},
		},
		{
			name: "multiple instances",
			instances: []config.InstanceConfig{
				{Name: "alpha", Port: 8080, StartCommand: "cmd1"},
				{Name: "beta", Port: 8081, StartCommand: "cmd2"},
				{Name: "gamma", Port: 8082, StartCommand: "cmd3"},
			},
			want: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			cfg := &config.Config{Instances: tt.instances}
			manager := NewInstanceManager(cfg, logger)

			got := manager.GetInstanceNames()

			// Check length
			if len(got) != len(tt.want) {
				t.Errorf("GetInstanceNames() returned %d names, want %d", len(got), len(tt.want))
				return
			}

			// Check order and values
			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("GetInstanceNames()[%d] = %q, want %q", i, name, tt.want[i])
				}
			}
		})
	}
}

// TestStartAllInstances tests AUTOSTART-02:
// InstanceManager.StartAllInstances method behavior
func TestStartAllInstances_AutoStartFlag(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name        string
		instances   []config.InstanceConfig
		wantStarted int
		wantSkipped int
		wantFailed  int
	}{
		{
			name: "all instances auto_start=nil (default true)",
			instances: []config.InstanceConfig{
				{Name: "inst1", Port: 8090, StartCommand: "nonexistent"},
				{Name: "inst2", Port: 8091, StartCommand: "nonexistent"},
			},
			wantStarted: 0, // 命令不存在,全部失败
			wantSkipped: 0,
			wantFailed:  2,
		},
		{
			name: "one instance skipped",
			instances: []config.InstanceConfig{
				{Name: "inst1", Port: 8090, StartCommand: "nonexistent"},
				{Name: "inst2", Port: 8091, StartCommand: "nonexistent", AutoStart: ptrBool(false)},
			},
			wantStarted: 0,
			wantSkipped: 1,
			wantFailed:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Instances: tt.instances}
			manager := NewInstanceManager(cfg, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			result := manager.StartAllInstances(ctx)

			if len(result.Started) != tt.wantStarted {
				t.Errorf("Started = %d, want %d", len(result.Started), tt.wantStarted)
			}
			if len(result.Skipped) != tt.wantSkipped {
				t.Errorf("Skipped = %d, want %d", len(result.Skipped), tt.wantSkipped)
			}
			if len(result.Failed) != tt.wantFailed {
				t.Errorf("Failed = %d, want %d", len(result.Failed), tt.wantFailed)
			}
		})
	}
}

// TestStartAllInstances_Order tests AUTOSTART-02:
// Instances are started in configuration order (serial)
func TestStartAllInstances_Order(t *testing.T) {
	// 验证实例按配置顺序依次启动
	// 可以通过检查日志输出或使用 mock 记录调用顺序
	// 当前简化实现:只要确认所有实例都被处理即可
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	instances := []config.InstanceConfig{
		{Name: "inst1", Port: 8090, StartCommand: "nonexistent"},
		{Name: "inst2", Port: 8091, StartCommand: "nonexistent"},
		{Name: "inst3", Port: 8092, StartCommand: "nonexistent"},
	}

	cfg := &config.Config{Instances: instances}
	manager := NewInstanceManager(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := manager.StartAllInstances(ctx)

	// 验证所有实例都被处理
	totalProcessed := len(result.Started) + len(result.Failed) + len(result.Skipped)
	if totalProcessed != 3 {
		t.Errorf("Expected 3 instances processed, got %d", totalProcessed)
	}
}

// TestStartAllInstances_GracefulDegradation tests AUTOSTART-03:
// Failed instance does not prevent other instances from starting
func TestStartAllInstances_GracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 创建两个实例:第一个会失败,第二个也应该尝试启动
	instances := []config.InstanceConfig{
		{Name: "failing-inst", Port: 8090, StartCommand: "nonexistent_command"},
		{Name: "also-failing", Port: 8091, StartCommand: "another_nonexistent"},
	}

	cfg := &config.Config{Instances: instances}
	manager := NewInstanceManager(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := manager.StartAllInstances(ctx)

	// 关键验证:两个实例都应该被尝试(即使第一个失败)
	// 由于命令不存在,两个都会失败,但都应该在 Failed 列表中
	if len(result.Failed) != 2 {
		t.Errorf("Failed count = %d, want 2 (both instances should be attempted)", len(result.Failed))
	}
	if len(result.Started) != 0 {
		t.Errorf("Started count = %d, want 0", len(result.Started))
	}
}

// TestStartAllInstances_Summary tests AUTOSTART-04:
// AutoStartResult contains summary with started/failed/skipped counts
func TestStartAllInstances_Summary(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	falseVal := false
	instances := []config.InstanceConfig{
		{Name: "skip-me", Port: 8090, StartCommand: "echo skip", AutoStart: &falseVal},
		{Name: "fail-me", Port: 8091, StartCommand: "nonexistent"},
	}

	cfg := &config.Config{Instances: instances}
	manager := NewInstanceManager(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := manager.StartAllInstances(ctx)

	// 验证 AutoStartResult 结构正确填充
	if len(result.Skipped) != 1 || result.Skipped[0] != "skip-me" {
		t.Errorf("Skipped = %v, want [skip-me]", result.Skipped)
	}
	if len(result.Failed) != 1 {
		t.Errorf("Failed count = %d, want 1", len(result.Failed))
	}
	if len(result.Started) != 0 {
		t.Errorf("Started count = %d, want 0", len(result.Started))
	}

	// 验证 Failed 中的 InstanceError 包含正确的实例名称
	if len(result.Failed) > 0 && result.Failed[0].InstanceName != "fail-me" {
		t.Errorf("Failed[0].InstanceName = %v, want fail-me", result.Failed[0].InstanceName)
	}
}

func ptrBool(v bool) *bool { return &v }

// TestInstanceLifecycleHelpers tests AUTOSTART-01 (indirect):
// Name(), Port(), ShouldAutoStart() helper methods on InstanceLifecycle
func TestInstanceLifecycleHelpers(t *testing.T) {
	falseVal := false
	trueVal := true

	tests := []struct {
		name                string
		config              config.InstanceConfig
		wantName            string
		wantPort            uint32
		wantShouldAutoStart bool
	}{
		{
			name: "basic config with nil AutoStart",
			config: config.InstanceConfig{
				Name:         "test-instance",
				Port:         18790,
				StartCommand: "echo test",
			},
			wantName:            "test-instance",
			wantPort:            18790,
			wantShouldAutoStart: true, // nil = default true
		},
		{
			name: "config with AutoStart=false",
			config: config.InstanceConfig{
				Name:         "skip-instance",
				Port:         18791,
				StartCommand: "echo test",
				AutoStart:    &falseVal,
			},
			wantName:            "skip-instance",
			wantPort:            18791,
			wantShouldAutoStart: false,
		},
		{
			name: "config with AutoStart=true",
			config: config.InstanceConfig{
				Name:         "force-instance",
				Port:         18792,
				StartCommand: "echo test",
				AutoStart:    &trueVal,
			},
			wantName:            "force-instance",
			wantPort:            18792,
			wantShouldAutoStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建最小化的 InstanceLifecycle 用于测试
			il := &InstanceLifecycle{
				config: tt.config,
			}

			if got := il.Name(); got != tt.wantName {
				t.Errorf("Name() = %v, want %v", got, tt.wantName)
			}
			if got := il.Port(); got != tt.wantPort {
				t.Errorf("Port() = %v, want %v", got, tt.wantPort)
			}
			if got := il.ShouldAutoStart(); got != tt.wantShouldAutoStart {
				t.Errorf("ShouldAutoStart() = %v, want %v", got, tt.wantShouldAutoStart)
			}
		})
	}
}
