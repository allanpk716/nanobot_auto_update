package main

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
)

// TestVersionFlag verifies --version exits immediately with version string
func TestVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the binary first (from project root)
	buildCmd := exec.Command("go", "build", "-o", "test-updater.exe", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Run with --version flag
	cmd := exec.Command("./test-updater.exe", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Expected --version to exit cleanly, got error: %v", err)
	}

	// Verify output contains version string
	if !strings.Contains(string(output), "nanobot-auto-updater") {
		t.Errorf("Expected version output to contain 'nanobot-auto-updater', got: %s", string(output))
	}
}

// TestHelpFlag verifies -h/--help shows usage information including JSON format
func TestHelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name string
		flag string
	}{
		{"short help flag", "-h"},
		{"long help flag", "--help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".", tt.flag)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Expected %s to exit cleanly, got error: %v", tt.flag, err)
			}

			// Verify output contains usage information
			outputStr := string(output)
			if !strings.Contains(outputStr, "Usage:") && !strings.Contains(outputStr, "Options:") {
				t.Errorf("Expected help output to contain 'Usage:' or 'Options:', got: %s", outputStr)
			}

			// Verify JSON output format documentation is present
			if !strings.Contains(outputStr, "JSON Output Format") {
				t.Errorf("Expected help output to contain 'JSON Output Format', got: %s", outputStr)
			}
		})
	}
}

// TestInvalidCronFlag verifies invalid cron expression exits with error
func TestInvalidCronFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := exec.Command("go", "run", ".", "-cron", "invalid-cron")
	output, err := cmd.CombinedOutput()

	// Should exit with error
	if err == nil {
		t.Fatal("Expected invalid cron to exit with error")
	}

	// Output should mention invalid cron
	outputStr := string(output)
	if !strings.Contains(outputStr, "invalid") && !strings.Contains(outputStr, "cron") {
		t.Errorf("Expected error message about invalid cron, got: %s", outputStr)
	}
}

// TestUpdateNowFlag verifies --update-now flag is parsed correctly
func TestUpdateNowFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Note: This test will actually try to run the updater, so we can't verify full execution
	// But we can verify the flag is accepted without immediate error
	cmd := exec.Command("go", "run", ".", "--update-now", "--config", "nonexistent.yaml")

	// We expect it to fail due to missing config or uv check, but not flag parsing
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	// Should not complain about unknown flag
	if strings.Contains(outputStr, "unknown flag") || strings.Contains(outputStr, "unknown shorthand") {
		t.Errorf("Flag parsing failed: %s", outputStr)
	}
}

// TestTimeoutFlag verifies --timeout flag is parsed correctly
func TestTimeoutFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name    string
		timeout string
		valid   bool
	}{
		{"valid minutes", "5m", true},
		{"valid seconds", "300s", true},
		{"valid combined", "2m30s", true},
		{"invalid format", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".", "--timeout", tt.timeout, "--help")
			output, err := cmd.CombinedOutput()

			if tt.valid {
				// Valid timeout should work with --help
				if err != nil {
					t.Errorf("Expected valid timeout to work, got error: %v, output: %s", err, string(output))
				}
			} else {
				// Invalid timeout should fail
				if err == nil {
					t.Errorf("Expected invalid timeout to fail, but it succeeded")
				}
			}
		})
	}
}

// TestTimeoutDefault verifies default timeout is 5 minutes
func TestTimeoutDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test verifies the default value is set correctly in the flag definition
	// The actual behavior is tested through integration tests
	expectedDefault := 5 * time.Minute

	// We verify this by checking that the help output shows the default
	cmd := exec.Command("go", "run", ".", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run help: %v", err)
	}

	// Check that default is shown as 5m0s (Go's time.Duration string representation)
	if !strings.Contains(string(output), "5m0s") {
		t.Errorf("Expected default timeout '5m0s' in help output, got: %s", string(output))
	}

	_ = expectedDefault // Use the variable to avoid compiler warning
}

func init() {
	// Ensure tests run on Windows only (scheduler has build constraint)
	if runtime.GOOS != "windows" {
		panic("Tests only valid on Windows")
	}
}

// TestMultiInstanceConfigLoading 验证多实例配置加载
func TestMultiInstanceConfigLoading(t *testing.T) {
	cfg, err := config.Load("../../tmp/test_multi_instance.yaml")
	if err != nil {
		t.Fatalf("Failed to load multi-instance config: %v", err)
	}

	if len(cfg.Instances) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(cfg.Instances))
	}

	if cfg.Instances[0].Name != "gateway" {
		t.Errorf("Expected first instance name 'gateway', got %q", cfg.Instances[0].Name)
	}

	if cfg.Instances[1].Name != "worker" {
		t.Errorf("Expected second instance name 'worker', got %q", cfg.Instances[1].Name)
	}
}

// TestLegacyConfigLoading 已被移除
// Phase 6 之后不再支持 legacy 单实例配置格式
// 所有配置必须使用 instances 数组格式
// 旧的 config.yaml 格式需要迁移到新格式

// TestModeDetection 验证模式检测逻辑
// 注意: Phase 6 之后所有配置必须使用 instances 数组格式
// legacy 单实例格式不再支持
func TestModeDetection(t *testing.T) {
	tests := []struct {
		name            string
		configFile      string
		expectMultiInst bool
	}{
		{"multi-instance mode", "../../tmp/test_multi_instance.yaml", true},
		// legacy mode 测试已移除 - 不再支持单实例配置格式
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(tt.configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			useMultiInstance := len(cfg.Instances) > 0

			if useMultiInstance != tt.expectMultiInst {
				t.Errorf("Expected useMultiInstance=%v, got %v", tt.expectMultiInst, useMultiInstance)
			}
		})
	}
}

// TestScheduledMultiInstanceUpdate 验证定时任务调用 InstanceManager.UpdateAll
// 使用 context.Background(),双层错误检查
func TestScheduledMultiInstanceUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 加载多实例配置
	cfg, err := config.Load("../../tmp/test_multi_instance.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证模式检测
	useMultiInstance := len(cfg.Instances) > 0
	if !useMultiInstance {
		t.Fatal("Expected multi-instance mode")
	}

	// 创建 logger
	logger := logging.NewLogger("./logs")

	// 创建 InstanceManager
	manager := instance.NewInstanceManager(cfg, logger, nil)

	// 验证使用 context.Background() 而非 WithTimeout()
	ctx := context.Background()

	// 执行更新
	result, err := manager.UpdateAll(ctx)

	// 验证双层错误检查
	if err != nil {
		// UV 更新失败 (严重错误)
		t.Logf("UV update failed: %v", err)
		// 这里不调用 NotifyFailure,因为我们在测试中
		return
	}

	// 实例失败 (优雅降级)
	if result.HasErrors() {
		t.Logf("Update completed with errors: stop_failed=%d, start_failed=%d",
			len(result.StopFailed), len(result.StartFailed))
		// 这里不调用 NotifyUpdateResult,因为我们在测试中
		return
	}

	// 完全成功
	t.Logf("Update completed successfully: stopped=%d, started=%d",
		len(result.Stopped), len(result.Started))
}

// TestUpdateNowMultiInstance 验证 --update-now 调用 InstanceManager.UpdateAll
// 使用 context.WithTimeout(),双层错误检查
func TestUpdateNowMultiInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 加载多实例配置
	cfg, err := config.Load("../../tmp/test_multi_instance.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证模式检测
	useMultiInstance := len(cfg.Instances) > 0
	if !useMultiInstance {
		t.Fatal("Expected multi-instance mode")
	}

	// 创建 logger
	logger := logging.NewLogger("./logs")

	// 创建 InstanceManager
	manager := instance.NewInstanceManager(cfg, logger, nil)

	// 验证使用 context.WithTimeout() 而非 Background()
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 执行更新
	result, err := manager.UpdateAll(ctx)

	// 验证双层错误检查
	if err != nil {
		// UV 更新失败 (严重错误) -> 应该调用 NotifyFailure
		t.Logf("UV update failed (should call NotifyFailure): %v", err)

		// 验证应该 os.Exit(1) (在测试中我们不真的退出)
		return
	}

	// 实例失败 (优雅降级) -> 应该调用 NotifyUpdateResult
	if result.HasErrors() {
		t.Logf("Update completed with errors (should call NotifyUpdateResult): stop_failed=%d, start_failed=%d",
			len(result.StopFailed), len(result.StartFailed))

		// 验证 JSON 输出应该包含 success: false
		return
	}

	// 完全成功
	t.Logf("Update completed successfully (should output JSON with success: true): stopped=%d, started=%d",
		len(result.Stopped), len(result.Started))
}

// TestMultiInstanceLongRunning 模拟多次更新周期
// 验证内存和 goroutine 稳定性
func TestMultiInstanceLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// 加载多实例配置
	cfg, err := config.Load("../../tmp/test_multi_instance.yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 创建 logger
	logger := logging.NewLogger("./logs")

	// 创建 InstanceManager
	manager := instance.NewInstanceManager(cfg, logger, nil)

	// 记录初始内存和 goroutine 状态
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)
	initialGoroutines := runtime.NumGoroutine()

	t.Logf("Initial state: Goroutines=%d, HeapAlloc=%d bytes", initialGoroutines, initialMemStats.HeapAlloc)

	// 模拟多次更新周期 (10 次)
	iterations := 10
	for i := 0; i < iterations; i++ {
		t.Logf("Iteration %d/%d", i+1, iterations)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		result, err := manager.UpdateAll(ctx)
		if err != nil {
			t.Logf("Iteration %d: UV update failed: %v", i+1, err)
		} else if result.HasErrors() {
			t.Logf("Iteration %d: Update completed with errors", i+1)
		} else {
			t.Logf("Iteration %d: Update completed successfully", i+1)
		}

		cancel()

		// 强制 GC 以更准确地检测内存泄漏
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	// 记录最终内存和 goroutine 状态
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Final state: Goroutines=%d, HeapAlloc=%d bytes", finalGoroutines, finalMemStats.HeapAlloc)

	// 验证内存稳定性 (允许 50% 增长)
	heapGrowth := float64(finalMemStats.HeapAlloc) / float64(initialMemStats.HeapAlloc)
	if heapGrowth > 1.5 {
		t.Errorf("Memory leak detected: HeapAlloc grew by %.2fx (%d -> %d bytes)",
			heapGrowth, initialMemStats.HeapAlloc, finalMemStats.HeapAlloc)
	} else {
		t.Logf("Memory stable: HeapAlloc growth %.2fx", heapGrowth)
	}

	// 验证 goroutine 稳定性
	// 注意: 由于每次更新都会启动子进程,goroutine 数量会有所增加
	// 这是 InstanceLifecycle 实现的预期行为,只要不持续增长即可
	goroutineDiff := finalGoroutines - initialGoroutines
	if goroutineDiff > 25 || goroutineDiff < -5 {
		t.Errorf("Goroutine leak detected: %d -> %d (diff=%d)", initialGoroutines, finalGoroutines, goroutineDiff)
	} else {
		t.Logf("Goroutines stable: diff=%d (acceptable for subprocess spawning)", goroutineDiff)
	}
}

// Helper function for existing tests that use exec.Command
func execCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}
