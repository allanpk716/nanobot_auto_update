package notification

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/network"
)

// NetworkMonitor 接口定义（用于依赖注入和测试）
type NetworkMonitor interface {
	GetState() *network.ConnectivityState
}

// Notifier 接口定义（用于依赖注入和测试）
type Notifier interface {
	IsEnabled() bool
	Notify(title, message string) error
}

// NotificationManager 网络连通性状态变化通知管理器
type NotificationManager struct {
	monitor  NetworkMonitor
	notifier Notifier
	logger   *slog.Logger

	// 内部状态追踪
	previousState *network.ConnectivityState
	mu            sync.RWMutex

	// 冷却时间管理
	cooldownTimer *time.Timer
	pendingChange *stateChange

	// 生命周期控制
	ctx    context.Context
	cancel context.CancelFunc
}

type stateChange struct {
	from bool // 之前状态: true=连通, false=不连通
	to   bool // 新状态
	time time.Time
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager(
	monitor NetworkMonitor,
	notifier Notifier,
	logger *slog.Logger,
) *NotificationManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &NotificationManager{
		monitor:  monitor,
		notifier: notifier,
		logger:   logger.With("component", "notification-manager"),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动通知管理器（在独立 goroutine 中运行）
func (nm *NotificationManager) Start(checkInterval time.Duration) {
	nm.logger.Info("通知管理器已启动", "check_interval", checkInterval)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// 立即执行一次初始检查
	nm.checkStateChange()

	for {
		select {
		case <-nm.ctx.Done():
			nm.logger.Info("通知管理器已停止")
			return
		case <-ticker.C:
			nm.checkStateChange()
		}
	}
}

// checkStateChange 检查连通性状态变化
func (nm *NotificationManager) checkStateChange() {
	currentState := nm.monitor.GetState()
	if currentState == nil {
		// 首次检查，NetworkMonitor 还未完成初始检查
		return
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 首次状态记录（不触发通知）
	if nm.previousState == nil {
		nm.previousState = currentState
		nm.logger.Info("初始连通性状态已记录", "is_connected", currentState.IsConnected)
		return
	}

	// 检测状态变化
	if nm.previousState.IsConnected != currentState.IsConnected {
		change := &stateChange{
			from: nm.previousState.IsConnected,
			to:   currentState.IsConnected,
			time: time.Now(),
		}

		// 取消之前的待确认变化（如果存在）
		if nm.cooldownTimer != nil {
			nm.cooldownTimer.Stop()
		}

		// 启动 1 分钟冷却确认
		nm.pendingChange = change
		nm.cooldownTimer = time.AfterFunc(1*time.Minute, func() {
			nm.confirmAndNotify(change)
		})

		nm.logger.Info("连通性状态变化检测,启动冷却确认",
			"from", change.from,
			"to", change.to,
			"cooldown", "1分钟")
	}

	// 更新前状态
	nm.previousState = currentState
}

// confirmAndNotify 冷却期满后确认状态并发送通知
func (nm *NotificationManager) confirmAndNotify(change *stateChange) {
	// 再次检查当前状态是否仍保持
	currentState := nm.monitor.GetState()
	if currentState == nil {
		return
	}

	// 状态已恢复原值，取消通知（网络抖动）
	if currentState.IsConnected != change.to {
		nm.logger.Info("冷却期内状态已恢复,取消通知",
			"change_to", change.to,
			"current", currentState.IsConnected)
		return
	}

	// 状态稳定，发送通知
	nm.sendNotification(change)
}

// sendNotification 异步发送通知
func (nm *NotificationManager) sendNotification(change *stateChange) {
	var title, message string

	if change.to {
		// 恢复通知
		title = "网络连通性已恢复"
		message = ""
	} else {
		// 失败通知
		title = "网络连通性检查失败"
		message = nm.getErrorType()
	}

	// 检查 Pushover 是否配置
	if !nm.notifier.IsEnabled() {
		direction := "从连通变为不连通"
		if change.to {
			direction = "从不连通变为连通"
		}
		nm.logger.Warn("网络连通性状态变化，但 Pushover 通知未配置。请在 config.yaml 中设置 pushover.api_token 和 pushover.user_key",
			"direction", direction)
		return
	}

	// 异步发送通知（不阻塞）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				nm.logger.Error("通知发送 goroutine panic",
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()

		if err := nm.notifier.Notify(title, message); err != nil {
			nm.logger.Error("发送连通性变化通知失败",
				"error", err,
				"title", title)
		}
	}()
}

// getErrorType 获取错误类型
func (nm *NotificationManager) getErrorType() string {
	state := nm.monitor.GetState()
	if state != nil && state.ErrorMessage != "" {
		return state.ErrorMessage
	}
	return "未知错误"
}

// Stop 停止通知管理器
func (nm *NotificationManager) Stop() {
	nm.logger.Info("正在停止通知管理器...")

	// 取消待执行的 timer
	if nm.cooldownTimer != nil {
		nm.cooldownTimer.Stop()
	}

	nm.cancel()
}
