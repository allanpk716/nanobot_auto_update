# Phase 27: Network Monitoring Notifications - Research

**Researched:** 2026-03-21
**Domain:** Network connectivity state change notifications via Pushover
**Confidence:** HIGH

## Summary

Phase 27 实现网络连通性状态变化时的 Pushover 通知功能。基于 Phase 26 已完成的 NetworkMonitor 基础设施,本阶段需要创建独立的 NotificationManager 来监听连通性状态变化、管理 1 分钟冷却时间、异步发送通知并处理失败场景。

**核心技术决策:**
- **通知订阅机制**: 基于轮询模式,NotificationManager 定期调用 `NetworkMonitor.GetState()` + 内部状态追踪,避免 channel 复杂性
- **冷却时间实现**: 使用 `time.AfterFunc()` 实现 1 分钟延迟确认,状态变化后启动 timer,期满后检查状态是否稳定
- **异步通知**: 在独立 goroutine 中调用 `Notifier.Notify()`,即使 Pushover API 响应慢也不阻塞监控循环
- **失败处理**: 仅记录 ERROR 日志,不重试,保持简单可靠

**Primary recommendation:** 创建独立的 `NotificationManager` 结构体,通过轮询 NetworkMonitor.GetState() 检测状态变化,使用 time.AfterFunc() 实现冷却时间,在 goroutine 中异步发送通知,未配置 Pushover 时记录 WARN 日志提醒用户。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 通知触发时机和频率控制
- **1 分钟冷却时间防止通知风暴**
  - 状态变化后等待 1 分钟确认,避免网络抖动时的频繁通知
  - 冷却逻辑: 状态变化 → 启动 1 分钟 timer → 冷却期满检查状态 → 如果仍保持新状态则发送通知
  - 适合快速响应场景,同时过滤短期波动
  - 实现: 使用 `time.AfterFunc()` 或 goroutine + `time.Sleep()`

#### 通知内容设计
- **简洁通知(状态+错误类型)**
  - 失败通知标题: "网络连通性检查失败"
  - 失败通知内容: "{错误类型}" (如 "连接超时"、"DNS 解析失败")
  - 恢复通知标题: "网络连通性已恢复"
  - 恢复通知内容: "" (空内容,标题已足够)
  - **不包含**: 持续时间、时间戳、目标 URL、HTTP 状态码等详细信息
  - 保持简洁,用户可查看应用日志了解详细诊断信息

#### Pushover 未配置处理
- **WARN 日志提醒用户配置**
  - 状态变化时如果 Pushover 未配置(Notifier.IsEnabled() == false),记录 WARN 日志
  - 日志消息: "网络连通性状态变化，但 Pushover 通知未配置。请在 config.yaml 中设置 pushover.api_token 和 pushover.user_key"
  - 包含状态变化方向: "从连通变为不连通" 或 "从不连通变为连通"
  - 确保用户知道错过了通知机会

#### 通知发送模式
- **异步发送(独立 goroutine)**
  - 通知发送在独立 goroutine 中执行,不阻塞监控循环
  - 实现: `go func() { if err := notifier.Notify(title, message); err != nil { ... } }()`
  - 即使 Pushover API 慢或失败,也不影响连通性检查的定时执行
  - 推荐做法,确保监控系统稳定性

#### 通知发送失败处理
- **仅记录 ERROR 日志,不重试**
  - 通知发送失败时记录 ERROR 日志,包含错误详情
  - 日志示例: "发送连通性变化通知失败 - 错误: pushover API timeout"
  - 不实现重试机制(保持简单)
  - 服务继续运行,下次状态变化会再次尝试通知

#### 架构组织
- **独立的 NotificationManager**
  - 创建独立的 `NotificationManager` 结构体,负责:
    - 监听 NetworkMonitor 的状态变化事件
    - 管理冷却时间逻辑(timer + 状态确认)
    - 调用 Notifier 发送通知
  - NetworkMonitor 保持纯粹的状态监控职责
  - NotificationManager 处理通知相关逻辑
  - 职责分离,更易测试和维护
  - 通过依赖注入接收 NetworkMonitor 和 Notifier

#### 集成和生命周期
- **在 main.go 中集成**
  - NetworkMonitor 启动后创建 NotificationManager
  - NotificationManager 订阅 NetworkMonitor 的状态变化
  - 实现方式(二选一,由 Claude 决定):
    - 方案 A: NetworkMonitor 提供 `Subscribe() <-chan ConnectivityChangeEvent` channel
    - 方案 B: NotificationManager 定期轮询 `NetworkMonitor.GetState()` + 内部状态追踪
  - 启动顺序: API 服务器 → 健康监控 → 网络监控 → 通知管理器
  - 关闭顺序: 通知管理器 → 网络监控 → 健康监控 → API 服务器

### Claude's Discretion
- 订阅机制的具体实现(channel vs 轮询)
- NotificationManager 的具体结构设计
- 冷却时间 timer 的管理方式(重置、取消)
- 通知失败时的具体日志格式
- 通知发送 goroutine 的错误处理细节

### Deferred Ideas (OUT OF SCOPE)
- **详细诊断信息通知** — 当前简洁通知足够,如需详细信息可查看应用日志
- **通知重试机制** — 当前失败仅记录日志,如需提高通知可靠性可添加重试逻辑
- **通知配置开关** — 当前 Pushover 可选配置即可,如需更细粒度控制可添加 enable_notify 开关
- **可配置冷却时间** — 当前固定 1 分钟,如需灵活性可添加到 monitor 配置中
- **多种通知渠道** — 当前仅 Pushover,如需支持邮件、Slack 等需要新的抽象层

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MONITOR-04 | 连通性从失败变为成功时发送 Pushover 恢复通知 | NotificationManager 检测状态变化 (IsConnected: false → true),冷却期满后调用 Notifier.Notify("网络连通性已恢复", "") |
| MONITOR-05 | 连通性从成功变为失败时发送 Pushover 失败通知 | NotificationManager 检测状态变化 (IsConnected: true → false),冷却期满后调用 Notifier.Notify("网络连通性检查失败", errorType) |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/gregdel/pushover | v1.4.0 | Pushover notification client | 已在 Phase 9 引入,项目标准通知库,当前最新稳定版,API 简单可靠 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| context | (stdlib) | Goroutine lifecycle management | NotificationManager 启动/停止控制,优雅关闭 |
| time | (stdlib) | Timer and cooldown management | 冷却时间计时(time.AfterFunc),状态检查间隔控制 |
| log/slog | (stdlib) | Structured logging | 状态变化日志,通知发送日志,错误日志 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| time.AfterFunc() | goroutine + time.Sleep() | AfterFunc 更简洁,避免 goroutine 泄漏风险,推荐使用 |
| channel 订阅模式 | 轮询 GetState() | Channel 更"事件驱动",但增加 NetworkMonitor 复杂性;轮询更简单,状态检查间隔 15 分钟时性能影响可忽略 |

**Installation:**
无需额外安装,所有依赖已存在于项目中。

**Version verification:**
```
github.com/gregdel/pushover v1.4.0 (已验证,当前最新版本)
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── notification/
│   └── manager.go          # NotificationManager 实现
├── network/
│   └── monitor.go          # NetworkMonitor (Phase 26,已存在)
├── notifier/
│   └── notifier.go         # Notifier (Phase 9,已存在)
└── config/
    └── config.go           # 配置结构(已存在,无需修改)
cmd/nanobot-auto-updater/
└── main.go                 # 生命周期集成(需修改)
```

### Pattern 1: NotificationManager 轮询模式
**What:** NotificationManager 定期调用 NetworkMonitor.GetState() 检测状态变化,内部维护 previousState 进行对比
**When to use:** 状态检查频率较低(15 分钟)且监控循环已在运行的场景,轮询比 channel 订阅更简单
**Example:**
```go
// Source: 基于现有 HealthMonitor 模式推导
package notification

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/HQGroup/nanobot-auto-updater/internal/network"
    "github.com/HQGroup/nanobot-auto-updater/internal/notifier"
)

// NotificationManager 网络连通性状态变化通知管理器
type NotificationManager struct {
    monitor  *network.NetworkMonitor
    notifier *notifier.Notifier
    logger   *slog.Logger

    // 内部状态追踪
    previousState *network.ConnectivityState
    mu            sync.RWMutex

    // 冷却时间管理
    cooldownTimer *time.Timer
    pendingChange *stateChange // 待确认的状态变化

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
    monitor *network.NetworkMonitor,
    notifier *notifier.Notifier,
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

// Start 启动通知管理器(在独立 goroutine 中运行)
func (nm *NotificationManager) Start(checkInterval time.Duration) {
    nm.logger.Info("通知管理器已启动", "check_interval", checkInterval)

    ticker := time.NewTicker(checkInterval)
    defer ticker.Stop()

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
        // 首次检查,NetworkMonitor 还未完成初始检查
        return
    }

    nm.mu.Lock()
    defer nm.mu.Unlock()

    // 首次状态记录(不触发通知)
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

        // 取消之前的待确认变化(如果存在)
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

    // 状态已恢复原值,取消通知(网络抖动)
    if currentState.IsConnected != change.to {
        nm.logger.Info("冷却期内状态已恢复,取消通知",
            "change_to", change.to,
            "current", currentState.IsConnected)
        return
    }

    // 状态稳定,发送通知
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
        // 从 NetworkMonitor 获取错误类型(需要扩展 GetState 或新增方法)
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

    // 异步发送通知(不阻塞)
    go func() {
        if err := nm.notifier.Notify(title, message); err != nil {
            nm.logger.Error("发送连通性变化通知失败",
                "error", err,
                "title", title)
        }
    }()
}

// getErrorType 获取错误类型(需要从 NetworkMonitor 扩展)
func (nm *NotificationManager) getErrorType() string {
    // TODO: 需要扩展 NetworkMonitor.GetState() 返回错误信息
    // 或新增 GetLastErrorMessage() 方法
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
```

### Pattern 2: NetworkMonitor 状态扩展(推荐)
**What:** 扩展 ConnectivityState 结构,包含错误类型信息,便于 NotificationManager 获取
**When to use:** 通知内容需要详细错误类型时
**Example:**
```go
// Source: 基于 internal/network/monitor.go 现有结构扩展
package network

// ConnectivityState 连通性状态(扩展版)
type ConnectivityState struct {
    IsConnected  bool
    LastCheck    time.Time
    ErrorMessage string // 新增: 最后一次错误消息(连通时为空)
}

// checkConnectivity 检查连通性并记录日志(修改版)
func (nm *NetworkMonitor) checkConnectivity() {
    // ... 现有逻辑 ...

    // 更新状态(扩展)
    nm.state = &ConnectivityState{
        IsConnected:  isConnected,
        LastCheck:    now,
        ErrorMessage: errMsg, // 记录错误消息
    }

    // ... 现有日志逻辑 ...
}
```

### Anti-Patterns to Avoid
- **在 NetworkMonitor 中直接调用 Notifier**: 违反单一职责原则,NetworkMonitor 应专注连通性检查,通知逻辑应由独立的 NotificationManager 处理
- **同步发送通知阻塞监控循环**: Notifier.Notify() 调用 Pushover API 可能耗时数秒,必须在 goroutine 中执行,否则阻塞 NetworkMonitor 的定时检查
- **使用 channel 订阅模式过度设计**: 状态检查间隔 15 分钟,轮询 GetState() 性能影响可忽略,channel 订阅增加 NetworkMonitor 复杂性,不推荐
- **冷却期内使用 time.Sleep() 阻塞**: 阻塞 checkStateChange() 方法,无法及时响应 Stop() 信号,应使用 time.AfterFunc() 非阻塞实现
- **忽略 Pushover 未配置场景**: 状态变化时必须检查 `notifier.IsEnabled()`,未配置时记录 WARN 日志提醒用户,避免用户错过重要通知

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pushover API 客户端 | 手动 HTTP 请求构造 | `github.com/gregdel/pushover` 库 | 已集成,API 简单可靠,处理认证、重试、响应解析 |
| 冷却时间管理 | 手动 goroutine + channel + select | `time.AfterFunc()` | 标准库提供,避免 goroutine 泄漏,代码更简洁 |
| 通知重试逻辑 | 指数退避、重试队列 | 不重试,仅记录 ERROR | 保持简单,下次状态变化会再次尝试,避免复杂性 |
| 状态变化检测 | 复杂的状态机框架 | 简单的 previousState != currentState 对比 | 状态仅有 true/false 两种,无需状态机,简单对比足够 |

**Key insight:** 通知系统应该保持简单可靠。异步发送、失败日志、下次重试的模式比复杂的重试机制更可靠。冷却时间防止通知风暴,1 分钟足够过滤网络抖动同时快速响应真实故障。

## Common Pitfalls

### Pitfall 1: 冷却 Timer 泄漏
**What goes wrong:** 每次状态变化创建新 timer,但未取消旧 timer,导致多个 timer 同时触发,发送重复通知
**Why it happens:** 忘记在创建新 timer 前调用 `timer.Stop()` 取消旧 timer
**How to avoid:**
```go
// 正确做法: 创建新 timer 前停止旧 timer
if nm.cooldownTimer != nil {
    nm.cooldownTimer.Stop()
}
nm.cooldownTimer = time.AfterFunc(1*time.Minute, func() {
    nm.confirmAndNotify(change)
})
```
**Warning signs:** 连续收到多条相同通知,日志中出现重复的 "冷却期满确认" 消息

### Pitfall 2: 通知发送 goroutine panic 导致应用崩溃
**What goes wrong:** Notifier.Notify() panic 时,整个应用崩溃退出
**Why it happens:** goroutine 中未捕获 panic,异常向上传播到 Go 运行时
**How to avoid:**
```go
// 正确做法: goroutine 中使用 defer recover
go func() {
    defer func() {
        if r := recover(); r != nil {
            nm.logger.Error("通知发送 goroutine panic",
                "panic", r,
                "stack", string(debug.Stack()))
        }
    }()

    if err := nm.notifier.Notify(title, message); err != nil {
        nm.logger.Error("发送连通性变化通知失败", "error", err, "title", title)
    }
}()
```
**Warning signs:** 应用突然退出,日志中出现 "panic" 关键字,但无后续日志

### Pitfall 3: 状态检查 race condition
**What goes wrong:** NotificationManager 的 checkStateChange() 和 NetworkMonitor 的 checkConnectivity() 并发访问 state,导致数据竞争
**Why it happens:** NetworkMonitor.state 在 goroutine 中更新,NotificationManager 在另一个 goroutine 中读取,未加锁
**How to avoid:**
```go
// NetworkMonitor.GetState() 应该加锁保护(当前实现未加锁,需要修复)
func (nm *NetworkMonitor) GetState() *ConnectivityState {
    nm.mu.RLock()
    defer nm.mu.RUnlock()
    return nm.state
}

// checkConnectivity() 更新 state 时也要加锁
func (nm *NetworkMonitor) checkConnectivity() {
    // ... 现有逻辑 ...

    nm.mu.Lock()
    nm.state = &ConnectivityState{...}
    nm.mu.Unlock()

    // ... 现有日志逻辑 ...
}
```
**Warning signs:** 使用 `go run -race` 检测到 DATA RACE 警告,状态读取返回 nil 或错误值

### Pitfall 4: Pushover 未配置时静默失败
**What goes wrong:** 用户配置了网络监控但未配置 Pushover,状态变化时无任何日志提示,用户以为通知功能正常工作
**Why it happens:** Notifier.Notify() 在 disabled 时返回 nil(无错误),NotificationManager 未检查 IsEnabled()
**How to avoid:**
```go
// 正确做法: 发送通知前检查 IsEnabled()
if !nm.notifier.IsEnabled() {
    nm.logger.Warn("网络连通性状态变化，但 Pushover 通知未配置。请在 config.yaml 中设置 pushover.api_token 和 pushover.user_key",
        "direction", direction)
    return
}
```
**Warning signs:** 用户报告 "网络故障但未收到通知",检查日志无 WARN 提示

### Pitfall 5: Stop() 未取消冷却 timer
**What goes wrong:** 应用关闭时,冷却 timer 仍在运行,可能在应用退出后尝试访问已释放的资源
**Why it happens:** Stop() 方法仅调用 cancel(),忘记停止 timer
**How to avoid:**
```go
// 正确做法: Stop() 中取消 timer
func (nm *NotificationManager) Stop() {
    nm.logger.Info("正在停止通知管理器...")

    if nm.cooldownTimer != nil {
        nm.cooldownTimer.Stop()
    }

    nm.cancel()
}
```
**Warning signs:** 应用关闭后日志中出现 "冷却期满确认" 消息,或 goroutine 泄漏警告

## Code Examples

### NotificationManager 完整实现(推荐模式)
```go
// Source: 基于现有 HealthMonitor 和 Notifier 模式推导
// File: internal/notification/manager.go

package notification

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/network"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
)

// NotificationManager 网络连通性状态变化通知管理器
type NotificationManager struct {
	monitor  *network.NetworkMonitor
	notifier *notifier.Notifier
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
	from bool
	to   bool
	time time.Time
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager(
	monitor *network.NetworkMonitor,
	notifier *notifier.Notifier,
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

// Start 启动通知管理器(在独立 goroutine 中运行)
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
		return
	}

	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 首次状态记录
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

		// 取消之前的待确认变化
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

	nm.previousState = currentState
}

// confirmAndNotify 冷却期满后确认状态并发送通知
func (nm *NotificationManager) confirmAndNotify(change *stateChange) {
	currentState := nm.monitor.GetState()
	if currentState == nil {
		return
	}

	// 状态已恢复原值,取消通知
	if currentState.IsConnected != change.to {
		nm.logger.Info("冷却期内状态已恢复,取消通知",
			"change_to", change.to,
			"current", currentState.IsConnected)
		return
	}

	nm.sendNotification(change)
}

// sendNotification 异步发送通知
func (nm *NotificationManager) sendNotification(change *stateChange) {
	var title, message string

	if change.to {
		title = "网络连通性已恢复"
		message = ""
	} else {
		title = "网络连通性检查失败"
		// 需要从 NetworkMonitor 获取错误类型
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

	// 异步发送通知
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
	// 需要扩展 NetworkMonitor.ConnectivityState 包含 ErrorMessage
	state := nm.monitor.GetState()
	if state != nil && state.ErrorMessage != "" {
		return state.ErrorMessage
	}
	return "未知错误"
}

// Stop 停止通知管理器
func (nm *NotificationManager) Stop() {
	nm.logger.Info("正在停止通知管理器...")

	if nm.cooldownTimer != nil {
		nm.cooldownTimer.Stop()
	}

	nm.cancel()
}
```

### NetworkMonitor 状态扩展(配合 NotificationManager)
```go
// Source: 基于 internal/network/monitor.go 扩展
// File: internal/network/monitor.go

package network

import "sync"

// ConnectivityState 连通性状态(扩展版)
type ConnectivityState struct {
	IsConnected  bool
	LastCheck    time.Time
	ErrorMessage string // 新增: 最后一次错误消息(连通时为空)
}

type NetworkMonitor struct {
	// ... 现有字段 ...
	mu sync.RWMutex // 新增: 保护 state 的读写锁
	// ... 现有字段 ...
}

// checkConnectivity 检查连通性并记录日志(修改版)
func (nm *NetworkMonitor) checkConnectivity() {
	start := time.Now()
	isConnected, statusCode, errMsg := nm.performCheck()
	duration := time.Since(start)

	// 更新状态(加锁)
	now := time.Now()
	nm.mu.Lock()
	previousState := nm.state
	nm.state = &ConnectivityState{
		IsConnected:  isConnected,
		LastCheck:    now,
		ErrorMessage: errMsg, // 新增
	}
	nm.mu.Unlock()

	// ... 现有日志逻辑 ...
}

// GetState 获取当前连通性状态(加锁版)
func (nm *NetworkMonitor) GetState() *ConnectivityState {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return nm.state
}
```

### main.go 集成(生命周期管理)
```go
// Source: 基于 cmd/nanobot-auto-updater/main.go 扩展
// File: cmd/nanobot-auto-updater/main.go

package main

import (
	// ... 现有 imports ...
	"github.com/HQGroup/nanobot-auto-updater/internal/notification"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
)

func main() {
	// ... 现有代码 ...

	// 创建 Notifier(如果尚未创建)
	notif := notifier.NewWithConfig(
		config.PushoverConfig{
			ApiToken: cfg.Pushover.ApiToken,
			UserKey:  cfg.Pushover.UserKey,
		},
		logger,
	)

	// ... API 服务器启动 ...

	// ... 健康监控启动 ...

	// 启动网络监控 (MONITOR-01, MONITOR-06)
	networkMonitor := network.NewNetworkMonitor(
		"https://www.google.com",
		cfg.Monitor.Interval,
		cfg.Monitor.Timeout,
		logger,
	)
	go networkMonitor.Start()
	logger.Info("网络监控已启动", "interval", cfg.Monitor.Interval)

	// 启动通知管理器 (MONITOR-04, MONITOR-05)
	notificationManager := notification.NewNotificationManager(
		networkMonitor,
		notif,
		logger,
	)
	// 使用与网络监控相同的检查间隔
	go notificationManager.Start(cfg.Monitor.Interval)
	logger.Info("通知管理器已启动", "check_interval", cfg.Monitor.Interval)

	// ... 自动启动实例 ...

	// ... 优雅关闭 ...

	// 停止通知管理器(优先于网络监控)
	if notificationManager != nil {
		notificationManager.Stop()
	}

	// 停止网络监控
	if networkMonitor != nil {
		networkMonitor.Stop()
	}

	// ... 健康监控停止 ...
	// ... API 服务器停止 ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 同步通知发送 | 异步 goroutine 发送 | Phase 27 设计 | 避免阻塞监控循环,即使 Pushover API 慢也不影响连通性检查 |
| 无冷却时间 | 1 分钟冷却确认 | Phase 27 设计 | 过滤网络抖动,避免通知风暴,同时保持快速响应 |
| channel 订阅模式 | 轮询 GetState() | Phase 27 设计 | 简化架构,避免 NetworkMonitor 复杂性,性能影响可忽略(15 分钟间隔) |
| 通知失败忽略 | 仅记录 ERROR 日志 | Phase 27 设计 | 保持简单,避免重试逻辑复杂性,下次状态变化会再次尝试 |

**Deprecated/outdated:**
- **Channel 订阅模式**: 虽然更"事件驱动",但增加 NetworkMonitor 复杂性,轮询模式在低频率场景下更简单可靠
- **通知重试机制**: 当前失败仅记录日志,下次状态变化会再次尝试,避免重试队列、指数退避等复杂性

## Open Questions

1. **NetworkMonitor 状态访问的线程安全性**
   - What we know: NetworkMonitor.state 在 goroutine 中更新,GetState() 在另一个 goroutine 中读取
   - What's unclear: 当前 GetState() 未加锁保护,可能存在 race condition
   - Recommendation: 必须在 NetworkMonitor 中添加 sync.RWMutex,GetState() 使用 RLock(),checkConnectivity() 使用 Lock() 更新 state

2. **错误类型信息的传递**
   - What we know: 通知内容需要包含错误类型("连接超时"、"DNS 解析失败"等)
   - What's unclear: 当前 ConnectivityState 结构仅包含 IsConnected 和 LastCheck,缺少错误信息
   - Recommendation: 扩展 ConnectivityState 添加 ErrorMessage 字段,checkConnectivity() 中记录 errMsg

3. **状态检查间隔与网络监控间隔的关系**
   - What we know: NetworkMonitor 每 15 分钟检查一次,NotificationManager 需要定期轮询 GetState()
   - What's unclear: NotificationManager 的检查间隔应该设置为多少?
   - Recommendation: 使用相同的 cfg.Monitor.Interval (15 分钟),保持一致,避免过度轮询

4. **首次状态变化的处理**
   - What we know: 应用启动后首次连通性检查可能立即触发状态变化(nil → true/false)
   - What's unclear: 首次状态变化是否需要发送通知?
   - Recommendation: 首次检查仅记录初始状态,不触发通知,仅在状态实际变化时(从连通到不连通或反向)发送通知

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (已存在) |
| Config file | None — standard Go test pattern |
| Quick run command | `go test ./internal/notification/... -v` |
| Full suite command | `go test ./... -v -race` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MONITOR-04 | 连通性从失败变为成功时发送 Pushover 恢复通知 | unit | `go test ./internal/notification -run TestRecoveryNotification -v` | ❌ Wave 0 |
| MONITOR-05 | 连通性从成功变为失败时发送 Pushover 失败通知 | unit | `go test ./internal/notification -run TestFailureNotification -v` | ❌ Wave 0 |
| MONITOR-04/05 | 冷却时间过滤网络抖动 | unit | `go test ./internal/notification -run TestCooldownTimer -v` | ❌ Wave 0 |
| MONITOR-04/05 | Pushover 未配置时记录 WARN 日志 | unit | `go test ./internal/notification -run TestDisabledNotifier -v` | ❌ Wave 0 |
| MONITOR-04/05 | 异步通知发送不阻塞监控循环 | unit | `go test ./internal/notification -run TestAsyncNotification -v` | ❌ Wave 0 |
| MONITOR-04/05 | 状态变化检测(轮询模式) | unit | `go test ./internal/notification -run TestStateChangeDetection -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/notification/... -v`
- **Per wave merge:** `go test ./... -v -race`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/notification/manager_test.go` — 核心通知逻辑测试
- [ ] `internal/notification/manager_test.go` — 冷却时间测试
- [ ] `internal/notification/manager_test.go` — Mock NetworkMonitor 和 Notifier
- [ ] `internal/network/monitor_test.go` — GetState() 线程安全测试(如果修改 NetworkMonitor)

*(如果 Phase 27 仅新增 NotificationManager,测试框架已存在,仅需编写测试文件)*

## Sources

### Primary (HIGH confidence)
- `.planning/phases/27-network-monitoring-notifications/27-CONTEXT.md` — Phase 27 上下文和决策
- `.planning/REQUIREMENTS.md` § MONITOR-04, MONITOR-05 — 通知需求定义
- `internal/network/monitor.go` — NetworkMonitor 现有实现,GetState() 方法(197-200 行)
- `internal/notifier/notifier.go` — Notifier 现有实现,Notify() 和 IsEnabled() 方法(86-112 行)
- `internal/health/monitor.go` — HealthMonitor 生命周期模式参考(独立结构体 + context + goroutine)

### Secondary (MEDIUM confidence)
- `.planning/phases/26-network-monitoring-core/26-CONTEXT.md` — NetworkMonitor 设计决策和状态追踪模式
- `.planning/phases/25-instance-health-monitoring/25-CONTEXT.md` — 健康监控器生命周期集成模式
- `cmd/nanobot-auto-updater/main.go` — 应用生命周期管理,组件启动/停止顺序(128-193 行)

### Tertiary (LOW confidence)
- `github.com/gregdel/pushover` 文档 — Pushover API 使用(已验证 v1.4.0 是最新版本)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 所有依赖已存在于项目中,Pushover 库 v1.4.0 是最新稳定版
- Architecture: HIGH — 基于 HealthMonitor 成功模式,轮询 + 冷却 timer + 异步通知是成熟方案
- Pitfalls: HIGH — 基于 Go 并发编程常见陷阱和现有代码审查发现的问题

**Research date:** 2026-03-21
**Valid until:** 30 天 — Go 标准库 API 稳定,Pushover 库 v1.4.0 长期支持,架构模式成熟

---

*Phase: 27-network-monitoring-notifications*
*Research completed: 2026-03-21*
