package notification

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/network"
)

// MockNetworkMonitor 模拟 NetworkMonitor（实现 NetworkMonitor 接口）
type MockNetworkMonitor struct {
	mu    sync.RWMutex
	state *network.ConnectivityState
}

func (m *MockNetworkMonitor) GetState() *network.ConnectivityState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *MockNetworkMonitor) SetState(isConnected bool, errorMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = &network.ConnectivityState{
		IsConnected:  isConnected,
		LastCheck:    time.Now(),
		ErrorMessage: errorMessage,
	}
}

// MockNotifier 模拟 Notifier（实现 Notifier 接口）
type MockNotifier struct {
	mu           sync.Mutex
	enabled      bool
	notifyCalled bool
	lastTitle    string
	lastMessage  string
	notifyError  error
}

func (m *MockNotifier) IsEnabled() bool {
	return m.enabled
}

func (m *MockNotifier) Notify(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalled = true
	m.lastTitle = title
	m.lastMessage = message
	return m.notifyError
}

func (m *MockNotifier) WasNotifyCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifyCalled
}

func (m *MockNotifier) GetLastNotification() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastTitle, m.lastMessage
}

// TestStateChangeDetection 测试状态变化检测
func TestStateChangeDetection(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.checkStateChange()

	// 验证首次检查记录了初始状态
	if nm.previousState == nil {
		t.Fatal("previousState should be set after first check")
	}
	if !nm.previousState.IsConnected {
		t.Error("previousState should be connected")
	}

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "DNS 解析失败")
	nm.checkStateChange()

	// 验证冷却 timer 已设置
	if nm.cooldownTimer == nil {
		t.Error("cooldown timer should be set after state change")
	}

	// 清理
	nm.Stop()
}

// TestFirstCheckNoNotification 测试首次检查仅记录初始状态，不触发通知
func TestFirstCheckNoNotification(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：不连通
	monitor.SetState(false, "连接超时")
	nm.checkStateChange()

	// 验证首次检查不触发通知
	if notifier.WasNotifyCalled() {
		t.Error("first check should not trigger notification")
	}

	// 验证冷却 timer 未设置（因为只是首次检查）
	if nm.cooldownTimer != nil {
		t.Error("cooldown timer should not be set on first check")
	}

	// 清理
	nm.Stop()
}

// TestDisabledNotifier 测试 Pushover 未配置时不调用 Notify()
func TestDisabledNotifier(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: false}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.checkStateChange()

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "连接超时")
	nm.checkStateChange()

	// 手动触发通知（模拟冷却期满）
	nm.mu.Lock()
	change := &stateChange{
		from: true,
		to:   false,
		time: time.Now(),
	}
	nm.mu.Unlock()
	nm.sendNotification(change)

	// 等待 goroutine 完成
	time.Sleep(50 * time.Millisecond)

	// 验证 Notify 未被调用
	if notifier.WasNotifyCalled() {
		t.Error("Notify should not be called when Pushover is disabled")
	}

	// 清理
	nm.Stop()
}

// TestStopCancelsCooldownTimer 测试 Stop() 取消待执行的冷却 timer
func TestStopCancelsCooldownTimer(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.checkStateChange()

	// 状态变化：连通 -> 不连通（启动冷却 timer）
	monitor.SetState(false, "连接超时")
	nm.checkStateChange()

	// 验证冷却 timer 已设置
	if nm.cooldownTimer == nil {
		t.Fatal("cooldown timer should be set")
	}

	// 立即停止（在冷却期结束前）
	nm.Stop()

	// 等待足够时间确保冷却 timer 不会触发
	time.Sleep(100 * time.Millisecond)

	// 验证通知未被调用（因为 Stop 取消了 timer）
	if notifier.WasNotifyCalled() {
		t.Error("notification should not be called after Stop()")
	}
}

// TestGetErrorType 测试错误类型获取
func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name            string
		state           *network.ConnectivityState
		expectedMessage string
	}{
		{
			name: "with error message",
			state: &network.ConnectivityState{
				IsConnected:  false,
				ErrorMessage: "DNS 解析失败",
			},
			expectedMessage: "DNS 解析失败",
		},
		{
			name: "empty error message",
			state: &network.ConnectivityState{
				IsConnected:  false,
				ErrorMessage: "",
			},
			expectedMessage: "未知错误",
		},
		{
			name:            "nil state",
			state:           nil,
			expectedMessage: "未知错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := &MockNetworkMonitor{}
			monitor.SetState(false, "")
			if tt.state != nil {
				monitor.state = tt.state
			} else {
				monitor.state = nil
			}

			notifier := &MockNotifier{enabled: true}
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

			nm := NewNotificationManager(monitor, notifier, logger)
			message := nm.getErrorType()

			if message != tt.expectedMessage {
				t.Errorf("expected %s, got %s", tt.expectedMessage, message)
			}
		})
	}
}
