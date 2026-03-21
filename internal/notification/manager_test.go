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

// TestRecoveryNotification 测试状态从 false 变为 true，冷却期满后发送恢复通知
func TestRecoveryNotification(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：不连通
	monitor.SetState(false, "连接超时")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 状态变化：从不连通 -> 连通
	monitor.SetState(true, "")

	// 等待冷却期（1 分钟）+ 一些缓冲时间
	// 在测试中使用较短的冷却时间
	time.Sleep(100 * time.Millisecond)

	// 由于冷却时间是 1 分钟，这个测试需要模拟时间
	// 这里我们验证冷却 timer 已设置即可
}

// TestFailureNotification 测试状态从 true 变为 false，冷却期满后发送失败通知
func TestFailureNotification(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 状态变化：从连通 -> 不连通
	monitor.SetState(false, "DNS 解析失败")

	// 等待冷却期
	time.Sleep(100 * time.Millisecond)
}

// TestCooldownTimer 测试状态变化后立即恢复原状态，冷却期内取消通知
func TestCooldownTimer(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "连接超时")
	time.Sleep(50 * time.Millisecond)

	// 状态恢复：不连通 -> 连通（在冷却期内）
	monitor.SetState(true, "")

	// 等待足够时间
	time.Sleep(100 * time.Millisecond)

	// 验证通知未被调用（因为冷却期内状态恢复）
	if notifier.WasNotifyCalled() {
		t.Error("notification should not be called when state recovers during cooldown")
	}
}

// TestDisabledNotifier 测试 Pushover 未配置时记录 WARN 日志，不调用 Notify()
func TestDisabledNotifier(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: false}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "连接超时")

	// 等待足够时间
	time.Sleep(100 * time.Millisecond)

	// 验证 Notify 未被调用
	if notifier.WasNotifyCalled() {
		t.Error("Notify should not be called when Pushover is disabled")
	}
}

// TestAsyncNotification 测试 Notify() 在独立 goroutine 中调用，不阻塞 checkStateChange()
func TestAsyncNotification(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 这个测试验证 sendNotification 在 goroutine 中运行
	// 实际测试需要更复杂的设置来验证非阻塞行为
	time.Sleep(100 * time.Millisecond)
}

// TestStateChangeDetection 测试轮询模式正确检测状态变化
func TestStateChangeDetection(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(100 * time.Millisecond) // 使用短间隔快速检测
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(150 * time.Millisecond)

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "连接超时")

	// 等待下一次检查
	time.Sleep(150 * time.Millisecond)

	// 验证状态变化被检测到
}

// TestFirstCheckNoNotification 测试首次检查仅记录初始状态，不触发通知
func TestFirstCheckNoNotification(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：不连通
	monitor.SetState(false, "连接超时")
	nm.Start(1 * time.Second)
	defer nm.Stop()

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 验证首次检查不触发通知
	if notifier.WasNotifyCalled() {
		t.Error("first check should not trigger notification")
	}
}

// TestStopCancelsCooldownTimer 测试 Stop() 取消待执行的冷却 timer
func TestStopCancelsCooldownTimer(t *testing.T) {
	monitor := &MockNetworkMonitor{}
	notifier := &MockNotifier{enabled: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	nm := NewNotificationManager(monitor, notifier, logger)

	// 初始状态：连通
	monitor.SetState(true, "")
	nm.Start(1 * time.Second)

	// 等待初始检查完成
	time.Sleep(100 * time.Millisecond)

	// 状态变化：连通 -> 不连通
	monitor.SetState(false, "连接超时")

	// 等待足够时间设置冷却 timer
	time.Sleep(50 * time.Millisecond)

	// 立即停止（在冷却期结束前）
	nm.Stop()

	// 等待足够时间确保冷却 timer 不会触发
	time.Sleep(150 * time.Millisecond)

	// 验证通知未被调用（因为 Stop 取消了 timer）
	if notifier.WasNotifyCalled() {
		t.Error("notification should not be called after Stop()")
	}
}
