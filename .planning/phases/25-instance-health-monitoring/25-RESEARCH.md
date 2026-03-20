# Phase 25: Instance Health Monitoring - Research

**Researched:** 2026-03-20
**Domain:** Go 健康监控、端口检测、定时任务、状态变化检测
**Confidence:** HIGH

## Summary

Phase 25 实现实例健康监控功能，定期检查每个 nanobot 实例的运行状态（通过端口监听），在状态变化时记录相应级别的日志（运行→停止记录 ERROR，停止→运行记录 INFO）。健康检查间隔可通过配置文件调整。

**Primary recommendation:** 使用现有的 `lifecycle.FindPIDByPort()` 和 `lifecycle.IsNanobotRunning()` 进行端口检测，创建独立的 `HealthMonitor` 结构体管理监控循环，使用 `time.Ticker` 定期检查，通过 map 记录每个实例的上一次状态来检测状态变化，在独立 goroutine 中运行监控循环并支持通过 context 实现优雅关闭。

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| HEALTH-01 | 定期检查每个实例的运行状态（通过端口监听） | 使用 `lifecycle.FindPIDByPort()` 检测端口，`time.Ticker` 实现定期检查 |
| HEALTH-02 | 实例从运行变为停止时记录 ERROR 日志 | 维护状态 map，比较前后状态，状态变化时记录 ERROR |
| HEALTH-03 | 实例从停止变为运行时记录 INFO 日志 | 维护状态 map，比较前后状态，状态变化时记录 INFO |
| HEALTH-04 | 健康检查间隔可通过配置文件调整 | 在 `config.Config` 中添加 `HealthCheck.Interval` 字段 |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gopsutil/v3 | v3.24.5 | 系统进程和端口检测 | 项目已使用，提供可靠的跨平台进程/网络信息 |
| time.Ticker | stdlib | 定期执行健康检查 | Go 标准库，goroutine-safe，内置 Stop() 防止泄漏 |
| context.Context | stdlib | 优雅关闭和超时控制 | Go 标准模式，支持取消信号传播 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| log/slog | stdlib | 结构化日志记录 | 项目已使用，记录状态变化日志 |
| sync.Map 或 map+mutex | stdlib | 并发安全的状态存储 | 记录实例上一次的运行状态 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gopsutil | 原生 net.Dial + 进程扫描 | gopsutil 更可靠，跨平台支持更好，已在项目中使用 |
| time.Ticker | time.Sleep 循环 | Ticker 更符合 Go 惯例，支持 Stop() 防止 goroutine 泄漏 |
| time.Ticker | cron 库（如 robfig/cron） | cron 更复杂，对于简单定时检查过度设计 |
| 独立 goroutine | 每个实例一个 goroutine | 单一监控循环更简单，避免多个 goroutine 管理 |

**Installation:**
```bash
# 无需安装新依赖，使用现有依赖
```

**Version verification:**
```bash
$ go list -m github.com/shirou/gopsutil/v3
github.com/shirou/gopsutil/v3 v3.24.5
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── health/
│   ├── monitor.go          # HealthMonitor 核心实现
│   └── monitor_test.go     # 单元测试
├── config/
│   ├── config.go           # 添加 HealthCheckConfig 字段
│   └── health.go           # 新增：健康检查配置验证
├── instance/
│   └── manager.go          # 添加 StartHealthMonitoring() 方法
└── lifecycle/
    └── detector.go         # 已存在：FindPIDByPort, IsNanobotRunning
```

### Pattern 1: HealthMonitor 结构体
**What:** 封装健康监控逻辑，维护每个实例的上一次状态
**When to use:** 所有健康监控场景
**Example:**
```go
// Source: 基于 Phase 24 auto-start 模式 + Go 健康监控最佳实践
package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
)

// InstanceHealthState 记录实例的健康状态
type InstanceHealthState struct {
	IsRunning bool
	LastCheck time.Time
}

// HealthMonitor 实例健康监控器
type HealthMonitor struct {
	instances []config.InstanceConfig
	interval  time.Duration
	logger    *slog.Logger

	// 状态追踪：instance name -> 上一次状态
	states map[string]*InstanceHealthState
	mu     sync.RWMutex // 保护 states 并发访问

	// 优雅关闭
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHealthMonitor 创建健康监控器
func NewHealthMonitor(
	instances []config.InstanceConfig,
	interval time.Duration,
	logger *slog.Logger,
) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		instances: instances,
		interval:  interval,
		logger:    logger.With("component", "health-monitor"),
		states:    make(map[string]*InstanceHealthState),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动健康监控循环（在独立 goroutine 中运行）
func (hm *HealthMonitor) Start() {
	hm.logger.Info("健康监控已启动", "interval", hm.interval)

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop() // 防止 goroutine 泄漏

	// 立即执行一次初始检查
	hm.checkAllInstances()

	for {
		select {
		case <-hm.ctx.Done():
			hm.logger.Info("健康监控已停止")
			return
		case <-ticker.C:
			hm.checkAllInstances()
		}
	}
}

// checkAllInstances 检查所有实例状态
func (hm *HealthMonitor) checkAllInstances() {
	for _, inst := range hm.instances {
		hm.checkInstance(inst)
	}
}

// checkInstance 检查单个实例状态并记录状态变化
func (hm *HealthMonitor) checkInstance(inst config.InstanceConfig) {
	// 检测实例是否运行（通过端口）
	running, pid, method, err := lifecycle.IsNanobotRunning(inst.Port)
	if err != nil {
		hm.logger.Error("检测实例状态失败",
			"instance", inst.Name,
			"port", inst.Port,
			"error", err)
		return
	}

	// 获取上一次状态
	hm.mu.RLock()
	prevState, exists := hm.states[inst.Name]
	hm.mu.RUnlock()

	currentState := &InstanceHealthState{
		IsRunning: running,
		LastCheck: time.Now(),
	}

	// 如果是第一次检查，只记录当前状态
	if !exists {
		hm.mu.Lock()
		hm.states[inst.Name] = currentState
		hm.mu.Unlock()

		hm.logger.Info("初始状态检查",
			"instance", inst.Name,
			"running", running,
			"pid", pid,
			"detection_method", method)
		return
	}

	// 检测状态变化并记录日志
	if prevState.IsRunning && !running {
		// HEALTH-02: 运行→停止，记录 ERROR
		hm.logger.Error("实例已停止",
			"instance", inst.Name,
			"port", inst.Port,
			"previous_pid", pid)
	} else if !prevState.IsRunning && running {
		// HEALTH-03: 停止→运行，记录 INFO
		hm.logger.Info("实例已恢复运行",
			"instance", inst.Name,
			"port", inst.Port,
			"pid", pid,
			"detection_method", method)
	}

	// 更新状态
	hm.mu.Lock()
	hm.states[inst.Name] = currentState
	hm.mu.Unlock()
}

// Stop 停止健康监控
func (hm *HealthMonitor) Stop() {
	hm.logger.Info("正在停止健康监控...")
	hm.cancel() // 取消 context，通知 goroutine 退出
}
```

### Pattern 2: 配置文件集成
**What:** 在 config.yaml 中添加健康检查间隔配置
**When to use:** 用户需要自定义健康检查频率
**Example:**
```yaml
# config.yaml
health_check:
  interval: 1m  # 健康检查间隔（默认 1 分钟）

instances:
  - name: "nanobot-me"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
```

**Go 配置结构:**
```go
// internal/config/health.go
package config

import (
	"fmt"
	"time"
)

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval" mapstructure:"interval"`
}

// Validate 验证健康检查配置
func (h *HealthCheckConfig) Validate() error {
	if h.Interval < 10*time.Second {
		return fmt.Errorf("health_check.interval 必须至少 10 秒，当前值: %v", h.Interval)
	}

	if h.Interval > 10*time.Minute {
		return fmt.Errorf("health_check.interval 不能超过 10 分钟，当前值: %v", h.Interval)
	}

	return nil
}

// internal/config/config.go
type Config struct {
	Instances   []InstanceConfig   `yaml:"instances" mapstructure:"instances"`
	Pushover    PushoverConfig     `yaml:"pushover" mapstructure:"pushover"`
	API         APIConfig          `yaml:"api" mapstructure:"api"`
	Monitor     MonitorConfig      `yaml:"monitor" mapstructure:"monitor"`
	HealthCheck HealthCheckConfig  `yaml:"health_check" mapstructure:"health_check"` // 新增
}

func (c *Config) defaults() {
	// ... 现有默认值 ...
	c.HealthCheck.Interval = 1 * time.Minute // 默认 1 分钟
}

func (c *Config) Validate() error {
	// ... 现有验证 ...
	if err := c.HealthCheck.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
```

### Pattern 3: 在 main.go 中启动监控
**What:** 应用启动时自动启动健康监控
**When to use:** AUTOSTART 完成后启动健康监控
**Example:**
```go
// cmd/nanobot-auto-updater/main.go
func main() {
	// ... 现有代码 ...

	// 创建 InstanceManager
	instanceManager := instance.NewInstanceManager(cfg, logger)

	// 创建并启动健康监控器
	var healthMonitor *health.HealthMonitor
	if len(cfg.Instances) > 0 {
		healthMonitor = health.NewHealthMonitor(
			cfg.Instances,
			cfg.HealthCheck.Interval,
			logger,
		)
		go healthMonitor.Start()
	}

	// Auto-start instances in goroutine (non-blocking)
	go func() {
		// ... 现有 auto-start 代码 ...
		instanceManager.StartAllInstances(autoStartCtx)
	}()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutdown signal received")

	// 优雅关闭：先停健康监控，再停 API
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if healthMonitor != nil {
		healthMonitor.Stop()
	}

	if apiServer != nil {
		apiServer.Shutdown(shutdownCtx)
	}

	logger.Info("Shutdown completed")
}
```

### Anti-Patterns to Avoid

- **忘记调用 ticker.Stop():** 导致 goroutine 泄漏，必须使用 `defer ticker.Stop()`
- **忘记调用 context.CancelFunc:** 导致 context 泄漏，必须使用 `defer cancel()`
- **在主 goroutine 中运行监控循环:** 阻塞应用启动，必须在独立 goroutine 中运行
- **使用 time.Sleep 而非 time.Ticker:** 难以实现优雅关闭，使用 `select` + `time.Ticker` 支持取消信号
- **状态检测不使用锁:** map 并发读写 panic，必须使用 `sync.RWMutex` 保护状态 map
- **在状态变化检测中阻塞:** 长时间阻塞影响其他实例检查，状态检测逻辑必须快速返回
- **每个实例一个 goroutine:** 过度复杂，使用单一监控循环检查所有实例更简单

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 端口检测 | 手动 net.Dial + 进程扫描 | `lifecycle.FindPIDByPort()` | 已实现，处理跨平台差异和错误场景 |
| 进程检测 | 手动 /proc 解析或 Windows API | `gopsutil` | 成熟库，处理平台差异和边界情况 |
| 定期任务 | time.Sleep 循环 | `time.Ticker` | 标准库，支持 Stop()，防止 goroutine 泄漏 |
| 优雅关闭 | channel + select | `context.Context` | Go 标准模式，支持超时和取消传播 |
| 并发安全 map | 手动 map + mutex | `sync.RWMutex` 或 `sync.Map` | 标准库，正确处理读写锁 |

**Key insight:** 端口检测和进程监控已有现成实现（`lifecycle` 包），直接复用即可。定期任务使用标准库 `time.Ticker` + `context.Context` 模式，避免重复造轮子。

## Common Pitfalls

### Pitfall 1: Goroutine 泄漏
**What goes wrong:** 忘记调用 `ticker.Stop()` 或 `cancel()`，导致 goroutine 永远不会退出
**Why it happens:** 对 Go 并发原语的清理机制理解不足
**How to avoid:** 始终使用 `defer ticker.Stop()` 和 `defer cancel()`
**Warning signs:** 应用退出时间变长，内存持续增长

### Pitfall 2: 状态检测误报
**What goes wrong:** 短暂网络波动导致端口检测失败，误判为实例停止
**Why it happens:** 单次检测不够可靠
**How to avoid:** 可以考虑连续 N 次检测失败才判定为停止（可选优化，Phase 25 先实现基础版本）
**Warning signs:** 频繁的 ERROR 日志但实例实际运行正常

### Pitfall 3: 配置验证不严格
**What goes wrong:** 用户设置过小或过大的检查间隔，导致资源浪费或监控不及时
**Why it happens:** 缺少配置边界检查
**How to avoid:** 在 `HealthCheckConfig.Validate()` 中限制最小 10s，最大 10m
**Warning signs:** CPU 占用过高（间隔过小）或监控失效（间隔过大）

### Pitfall 4: 并发访问状态 map 导致 panic
**What goes wrong:** 多个 goroutine 同时读写 map 导致 panic
**Why it happens:** Go map 不是并发安全的
**How to avoid:** 使用 `sync.RWMutex` 保护所有 map 访问，读用 `RLock()`，写用 `Lock()`
**Warning signs:** 偶发的 panic，难以复现

### Pitfall 5: 状态变化检测逻辑错误
**What goes wrong:** 每次检查都记录日志，或者漏掉状态变化
**Why it happens:** 状态比较逻辑错误，或初始状态处理不当
**How to avoid:** 第一次检查只记录初始状态不触发变化日志，后续检查比较前后状态
**Warning signs:** 日志过多或缺少关键状态变化日志

## Code Examples

Verified patterns from official sources:

### 端口检测（已实现）
```go
// Source: internal/lifecycle/detector.go (项目现有代码)
func FindPIDByPort(port uint32, logger *slog.Logger) (int32, error) {
	logger.Debug("Checking port for nanobot process", "port", port)

	connections, err := net.Connections("tcp")
	if err != nil {
		logger.Error("Failed to get network connections", "error", err)
		return 0, fmt.Errorf("failed to get network connections: %w", err)
	}

	for _, conn := range connections {
		// Check if connection is listening on the specified port
		if conn.Status == "LISTEN" && conn.Laddr.Port == port {
			logger.Info("Found nanobot by port", "pid", conn.Pid, "port", port)
			return conn.Pid, nil
		}
	}

	logger.Debug("No process found listening on port", "port", port)
	return 0, nil // No process found, not an error
}
```

### Ticker + Context 优雅关闭模式
```go
// Source: Go 官方文档 + VictoriaMetrics 最佳实践
// https://victoriametrics.com/blog/go-graceful-shutdown/
func monitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop() // 防止 goroutine 泄漏

	for {
		select {
		case <-ctx.Done():
			// 收到取消信号，退出循环
			return
		case <-ticker.C:
			// 执行定期检查
			doHealthCheck()
		}
	}
}

// 使用示例
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // 防止 context 泄漏

go monitor(ctx, 1*time.Minute)

// 需要停止时
cancel() // 通知 goroutine 退出
```

### 并发安全的状态 map
```go
// Source: Go 标准库模式
type HealthMonitor struct {
	states map[string]*InstanceHealthState
	mu     sync.RWMutex
}

func (hm *HealthMonitor) getState(name string) (*InstanceHealthState, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	state, exists := hm.states[name]
	return state, exists
}

func (hm *HealthMonitor) setState(name string, state *InstanceHealthState) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.states[name] = state
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| time.Sleep 循环 | time.Ticker + select | Go 1.0+ | 支持优雅关闭，防止 goroutine 泄漏 |
| channel 传递取消信号 | context.Context | Go 1.7+ (2016) | 标准化取消传播，支持超时和级联取消 |
| 全局变量存储状态 | 封装在结构体中 | Go 最佳实践 | 更好的封装性，易于测试和维护 |
| 手动进程扫描 | gopsutil 库 | 项目 Phase 0 | 跨平台支持，更可靠的进程检测 |

**Deprecated/outdated:**
- **time.Sleep in loop:** 不支持优雅关闭，容易导致 goroutine 泄漏，使用 `time.Ticker` + `select` 替代
- **channel-based cancellation:** 不如 `context.Context` 灵活，不支持超时和级联取消

## Open Questions

1. **是否需要重试机制？**
   - What we know: 单次端口检测可能因短暂网络波动失败
   - What's unclear: 是否需要连续 N 次失败才判定为停止
   - Recommendation: Phase 25 先实现基础版本（单次检测），后续根据实际运行情况决定是否添加重试

2. **是否需要暴露监控状态给 API？**
   - What we know: 当前只需记录日志
   - What's unclear: 未来是否需要 HTTP API 查询当前健康状态
   - Recommendation: Phase 25 只实现日志记录，保持简单。如需 API 查询，可在后续 Phase 添加

3. **检查间隔的合理默认值？**
   - What we know: 过小浪费资源，过大监控不及时
   - What's unclear: nanobot 实例的典型崩溃频率
   - Recommendation: 默认 1 分钟，用户可根据需求调整（10s - 10m 范围）

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — Go test 使用 `*_test.go` 文件 |
| Quick run command | `go test ./internal/health/... -v` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HEALTH-01 | 定期检查每个实例的运行状态（通过端口监听） | unit | `go test ./internal/health -run TestCheckInstance -v` | ❌ Wave 0 |
| HEALTH-02 | 实例从运行变为停止时记录 ERROR 日志 | unit | `go test ./internal/health -run TestStateChange_StopToRunning -v` | ❌ Wave 0 |
| HEALTH-03 | 实例从停止变为运行时记录 INFO 日志 | unit | `go test ./internal/health -run TestStateChange_RunningToStop -v` | ❌ Wave 0 |
| HEALTH-04 | 健康检查间隔可通过配置文件调整 | unit | `go test ./internal/config -run TestHealthCheckConfig -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/health/... -v`
- **Per wave merge:** `go test ./... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/health/monitor.go` — 核心健康监控实现
- [ ] `internal/health/monitor_test.go` — 单元测试
- [ ] `internal/config/health.go` — 健康检查配置验证
- [ ] `internal/config/health_test.go` — 配置验证测试
- [ ] 更新 `internal/config/config.go` — 添加 `HealthCheck` 字段和默认值

*(If no gaps: "None — existing test infrastructure covers all phase requirements")*

## Sources

### Primary (HIGH confidence)
- **项目现有代码** - `internal/lifecycle/detector.go` - 端口检测实现已验证可用
- **Go 标准库文档** - `time.Ticker`, `context.Context` - 官方推荐的并发模式
- **Go 1.24 文档** - 标准库 API 和最佳实践

### Secondary (MEDIUM confidence)
- [VictoriaMetrics: Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) - 优雅关闭模式，ticker 清理
- [OneUptime: Graceful Shutdown for Kubernetes (Jan 2026)](https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view) - context 取消模式
- [DEV.to: Preventing Goroutine Leaks](https://dev.to/serifcolakel/go-concurrency-mastery-preventing-goroutine-leaks-with-context-timeout-cancellation-best-1lg0) - goroutine 泄漏防护

### Tertiary (LOW confidence)
- [OneUptime: Monitoring Go Channels with OpenTelemetry (Feb 2026)](https://oneuptime.com/blog/post/2026-02-06-monitor-go-channels-concurrency-opentelemetry/view) - 监控模式参考

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖都是成熟的标准库或项目已使用的库
- Architecture: HIGH - 基于现有代码模式和 Go 标准并发模式
- Pitfalls: HIGH - 常见的 Go 并发陷阱，有大量官方文档和社区经验

**Research date:** 2026-03-20
**Valid until:** 90 天（Go 1.24 稳定，模式成熟）
