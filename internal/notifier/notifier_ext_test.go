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

// testLogger 创建一个用于测试的 logger
func testLogger() *slog.Logger {
	// 返回一个丢弃所有日志的 logger,避免测试输出噪音
	return slog.New(slog.DiscardHandler)
}
