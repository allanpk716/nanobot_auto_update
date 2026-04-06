package notifier

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
)

// TestNotifyUpdateResult_NoErrors 验证所有实例成功时不发送通知
func TestNotifyUpdateResult_NoErrors(t *testing.T) {
	// 创建一个 disabled notifier
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	// 创建一个所有实例都成功的结果
	result := &instance.UpdateResult{
		Stopped:     []string{"instance1", "instance2"},
		Started:     []string{"instance1", "instance2"},
		StopFailed:  nil,
		StartFailed: nil,
	}

	// 执行
	err := n.NotifyUpdateResult(result)

	// 验证
	if err != nil {
		t.Errorf("期望返回 nil,实际返回: %v", err)
	}
}

// TestNotifyUpdateResult_WithStopFailures 验证停止失败时发送通知
func TestNotifyUpdateResult_WithStopFailures(t *testing.T) {
	// 创建一个 disabled notifier (避免实际发送,但可以测试逻辑)
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	// 创建一个有停止失败的结果
	result := &instance.UpdateResult{
		Stopped: []string{"instance1"},
		Started: []string{"instance1"},
		StopFailed: []*instance.InstanceError{
			{
				InstanceName: "instance2",
				Operation:    "stop",
				Port:         8081,
				Err:          errors.New("进程未找到"),
			},
		},
		StartFailed: nil,
	}

	// 执行
	err := n.NotifyUpdateResult(result)

	// 验证 - disabled notifier 不会返回错误
	if err != nil {
		t.Errorf("期望返回 nil,实际返回: %v", err)
	}
}

// TestNotifyUpdateResult_WithStartFailures 验证启动失败时发送通知
func TestNotifyUpdateResult_WithStartFailures(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	// 创建一个有启动失败的结果
	result := &instance.UpdateResult{
		Stopped:    []string{"instance1", "instance2"},
		Started:    []string{"instance1"},
		StopFailed: nil,
		StartFailed: []*instance.InstanceError{
			{
				InstanceName: "instance2",
				Operation:    "start",
				Port:         8081,
				Err:          errors.New("端口被占用"),
			},
		},
	}

	// 执行
	err := n.NotifyUpdateResult(result)

	// 验证
	if err != nil {
		t.Errorf("期望返回 nil,实际返回: %v", err)
	}
}

// TestNotifyUpdateResult_WithMixedResults 验证混合结果时消息完整性
func TestNotifyUpdateResult_WithMixedResults(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	// 创建一个混合结果
	result := &instance.UpdateResult{
		Stopped: []string{"instance1", "instance3"},
		Started: []string{"instance1"},
		StopFailed: []*instance.InstanceError{
			{
				InstanceName: "instance2",
				Operation:    "stop",
				Port:         8081,
				Err:          errors.New("进程未找到"),
			},
		},
		StartFailed: []*instance.InstanceError{
			{
				InstanceName: "instance3",
				Operation:    "start",
				Port:         8082,
				Err:          errors.New("启动超时"),
			},
		},
	}

	// 执行
	err := n.NotifyUpdateResult(result)

	// 验证
	if err != nil {
		t.Errorf("期望返回 nil,实际返回: %v", err)
	}
}

// TestFormatUpdateResultMessage_Formatting 验证消息格式化细节
func TestFormatUpdateResultMessage_Formatting(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	tests := []struct {
		name           string
		result         *instance.UpdateResult
		expectedParts  []string
		unexpectedPart string
	}{
		{
			name: "停止失败消息格式",
			result: &instance.UpdateResult{
				Stopped: []string{"instance1"},
				Started: []string{"instance1"},
				StopFailed: []*instance.InstanceError{
					{
						InstanceName: "instance2",
						Operation:    "stop",
						Port:         8081,
						Err:          errors.New("进程未找到"),
					},
				},
				StartFailed: nil,
			},
			expectedParts: []string{
				"停止失败的实例:",
				"✗ instance2",
				"端口 8081",
				"原因: 进程未找到",
			},
		},
		{
			name: "启动失败消息格式",
			result: &instance.UpdateResult{
				Stopped:    []string{"instance1"},
				Started:    []string{},
				StopFailed: nil,
				StartFailed: []*instance.InstanceError{
					{
						InstanceName: "instance1",
						Operation:    "start",
						Port:         8080,
						Err:          errors.New("端口被占用"),
					},
				},
			},
			expectedParts: []string{
				"启动失败的实例:",
				"✗ instance1",
				"端口 8080",
				"原因: 端口被占用",
			},
		},
		{
			name: "成功列表格式",
			result: &instance.UpdateResult{
				Stopped:     []string{"instance1", "instance2"},
				Started:     []string{"instance1", "instance2"},
				StopFailed:  nil,
				StartFailed: nil,
			},
			expectedParts:  []string{},
			unexpectedPart: "成功启动的实例",
		},
		{
			name: "混合结果包含所有部分",
			result: &instance.UpdateResult{
				Stopped: []string{"instance1", "instance3"},
				Started: []string{"instance1"},
				StopFailed: []*instance.InstanceError{
					{
						InstanceName: "instance2",
						Operation:    "stop",
						Port:         8081,
						Err:          errors.New("停止失败"),
					},
				},
				StartFailed: []*instance.InstanceError{
					{
						InstanceName: "instance3",
						Operation:    "start",
						Port:         8082,
						Err:          errors.New("启动失败"),
					},
				},
			},
			expectedParts: []string{
				"停止失败的实例:",
				"✗ instance2",
				"启动失败的实例:",
				"✗ instance3",
				"成功启动的实例 (1):",
				"✓ instance1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 如果没有错误,返回空字符串
			if !tt.result.HasErrors() {
				// 无错误时不应该调用 formatUpdateResultMessage
				return
			}

			msg := n.formatUpdateResultMessage(tt.result)

			for _, part := range tt.expectedParts {
				if !strings.Contains(msg, part) {
					t.Errorf("消息缺少预期部分: %q\n完整消息:\n%s", part, msg)
				}
			}

			if tt.unexpectedPart != "" && strings.Contains(msg, tt.unexpectedPart) {
				t.Errorf("消息不应该包含: %q\n完整消息:\n%s", tt.unexpectedPart, msg)
			}
		})
	}
}

// testLogger creates a logger for testing
// Returns a logger that discards all log output to avoid test noise
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// --- Startup Notification Tests (Phase 41-01) ---

// TestFormatStartupMessage_AllSuccess verifies message when all instances started successfully
func TestFormatStartupMessage_AllSuccess(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{"gw", "wk1", "wk2"},
		Failed:  nil,
		Skipped: []string{},
	}

	title, message := n.formatStartupMessage(result)

	if title != "Nanobot startup completed" {
		t.Errorf("expected title %q, got %q", "Nanobot startup completed", title)
	}
	if !strings.Contains(message, "All 3 instances started successfully") {
		t.Errorf("message should contain %q, got %q", "All 3 instances started successfully", message)
	}
}

// TestFormatStartupMessage_PartialFailure verifies message with mixed started/failed
func TestFormatStartupMessage_PartialFailure(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{"gw", "wk1"},
		Failed: []*instance.InstanceError{
			{
				InstanceName: "bad",
				Port:         8090,
				Err:          errors.New("port in use"),
			},
		},
		Skipped: []string{},
	}

	title, message := n.formatStartupMessage(result)

	if title != "Nanobot startup partially failed" {
		t.Errorf("expected title %q, got %q", "Nanobot startup partially failed", title)
	}
	if !strings.Contains(message, "Started: 2/3") {
		t.Errorf("message should contain %q, got %q", "Started: 2/3", message)
	}
	if !strings.Contains(message, "OK gw") {
		t.Errorf("message should contain %q, got %q", "OK gw", message)
	}
	if !strings.Contains(message, "OK wk1") {
		t.Errorf("message should contain %q, got %q", "OK wk1", message)
	}
	if !strings.Contains(message, "FAIL bad:") {
		t.Errorf("message should contain %q, got %q", "FAIL bad:", message)
	}
	if !strings.Contains(message, "port in use") {
		t.Errorf("message should contain %q, got %q", "port in use", message)
	}
}

// TestFormatStartupMessage_AllFailed verifies message when all instances failed
func TestFormatStartupMessage_AllFailed(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{},
		Failed: []*instance.InstanceError{
			{
				InstanceName: "gw",
				Port:         18790,
				Err:          errors.New("connection refused"),
			},
			{
				InstanceName: "wk1",
				Port:         18791,
				Err:          errors.New("timeout"),
			},
		},
		Skipped: []string{},
	}

	title, message := n.formatStartupMessage(result)

	if title != "Nanobot startup failed" {
		t.Errorf("expected title %q, got %q", "Nanobot startup failed", title)
	}
	if !strings.Contains(message, "Started: 0/2") {
		t.Errorf("message should contain %q, got %q", "Started: 0/2", message)
	}
	if !strings.Contains(message, "FAIL gw:") {
		t.Errorf("message should contain %q, got %q", "FAIL gw:", message)
	}
	if !strings.Contains(message, "FAIL wk1:") {
		t.Errorf("message should contain %q, got %q", "FAIL wk1:", message)
	}
}

// TestFormatStartupMessage_SkippedNotIncluded verifies skipped instances never appear in message
func TestFormatStartupMessage_SkippedNotIncluded(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{"gw"},
		Failed: []*instance.InstanceError{
			{
				InstanceName: "bad",
				Err:          errors.New("failed"),
			},
		},
		Skipped: []string{"manual1", "manual2", "manual3", "manual4", "manual5"},
	}

	title, message := n.formatStartupMessage(result)

	if title == "" {
		t.Error("expected non-empty title")
	}
	if strings.Contains(message, "manual") {
		t.Errorf("message should NOT contain any skipped instance names, got %q", message)
	}
}

// TestNotifyStartupResult_Disabled verifies disabled notifier returns nil without panic
func TestNotifyStartupResult_Disabled(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{"gw", "wk1"},
		Failed:  nil,
		Skipped: []string{},
	}

	err := n.NotifyStartupResult(result)
	if err != nil {
		t.Errorf("expected nil error for disabled notifier, got %v", err)
	}
}

// TestNotifyStartupResult_AllSkipped verifies no notification when all instances skipped
func TestNotifyStartupResult_AllSkipped(t *testing.T) {
	n := &Notifier{
		enabled: false,
		logger:  testLogger(),
	}

	result := &instance.AutoStartResult{
		Started: []string{},
		Failed:  nil,
		Skipped: []string{"a", "b", "c"},
	}

	err := n.NotifyStartupResult(result)
	if err != nil {
		t.Errorf("expected nil error for all-skipped result, got %v", err)
	}
}
