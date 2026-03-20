# Phase 26: Network Monitoring Core - Research

**Researched:** 2026-03-21
**Domain:** Go HTTP 客户端、网络连通性监控、定时任务、状态追踪
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 监控目标和方法
- **HTTP HEAD 请求到 https://www.google.com**
  - 使用 HEAD 方法而非 GET,减少流量和耗时
  - 测试基础连通性,不下载响应体
  - 目标 URL: `https://www.google.com`
  - 禁用 HTTP 重定向跟随,严格测试直接响应
  - 实现: 使用 Go 标准库 `net/http.Client` 发送 HEAD 请求

#### 成功标准
- **仅 HTTP 200 OK 才算成功**
  - 严格标准,只有 200 状态码视为连通性成功
  - 所有其他 HTTP 状态码(包括 2xx、3xx、4xx、5xx)视为失败
  - 简单明确,适合基础连通性测试
  - 记录实际收到的状态码用于调试

#### 日志详情
- **记录全面的诊断信息**
  - 成功日志: INFO 级别,包含响应时间(ms)、HTTP 状态码(200)
  - 失败日志: ERROR 级别,包含响应时间(ms)、错误类型分类
  - 响应时间: 记录从发起请求到收到响应头的总耗时
  - 不细分 DNS 解析、TCP 连接、TLS 握手各阶段时间(保持简单)
  - 日志示例:
    ```
    INFO  Google 连通性检查成功 duration=234ms status_code=200
    ERROR Google 连通性检查失败 duration=5000ms error_type="连接超时"
    ```

#### 失败分类
- **统一 ERROR 日志 + 错误类型标注**
  - 所有失败统一记录为 ERROR 日志级别
  - 在日志消息中标注错误类型,帮助快速定位问题
  - 基础错误分类(覆盖常见场景):
    - DNS 解析失败: `net.DNSError`
    - 连接超时: `context.DeadlineExceeded` 或 `net.Error.Timeout() == true`
    - 连接拒绝: `syscall.ECONNREFUSED`
    - TLS 握手错误: `tls.CertificateError`, `x509.UnknownAuthorityError`
    - HTTP 非 200 响应: 根据状态码分类 (3xx, 4xx, 5xx)
  - 不使用不同的日志级别区分错误严重程度(保持简单)

#### HTTP 客户端配置
- **禁用重定向跟随**
  - 配置 `http.Client.CheckRedirect` 返回 `http.ErrUseLastResponse`
  - 避免跟随 301/302 重定向,严格测试 google.com 直接响应
  - 适合 HEAD 请求场景
- **使用标准 HTTP 客户端**
  - 不设置自定义 User-Agent(使用 Go 默认)
  - 不验证 TLS 证书(使用系统默认信任链)
  - 超时使用配置的 `monitor.timeout` (默认 10s)

#### 启动时机和生命周期
- **API 服务器启动后启动**
  - 启动顺序: 配置加载 → InstanceManager 创建 → API 服务器启动 → 健康监控启动 → 网络监控启动
  - 与 Phase 25 健康监控相同,确保 API 服务器先准备好
  - 网络监控在独立 goroutine 中运行,不阻塞其他组件
  - 实现位置: 在 `main.go` 中,API 服务器启动 goroutine 和健康监控启动之后

#### 状态追踪
- **追踪上一次连通性状态**
  - 类似 Phase 25 健康监控,维护 `lastState` 变量记录上一次连通性状态
  - 每次检查后更新状态
  - 在日志中标注状态变化(首次检查、状态保持、状态改变)
  - 为 Phase 27 连通性变化通知做准备
  - 状态定义:
    ```go
    type ConnectivityState struct {
        IsConnected bool      // true: 连通, false: 不连通
        LastCheck   time.Time // 上次检查时间
    }
    ```

#### 优雅关闭
- **与启动顺序相反**
  - 关闭顺序: 网络监控先停 → 健康监控后停 → API 服务器最后停
  - 使用 `context.Context` 实现优雅关闭
  - 监控循环监听 `ctx.Done()` 信号,收到后立即退出
  - 在应用 shutdown 钩子中调用 `networkMonitor.Stop()`

### Claude's Discretion
- 日志消息的具体措辞(中文/英文)
- 响应时间格式(毫秒 vs 秒)
- 错误类型标注格式(括号、冒号、等号)
- 监控循环的初始延迟(立即检查 vs 等待第一个 interval)

### Deferred Ideas (OUT OF SCOPE)
- **状态变化通知** — Phase 27 专门处理连通性状态变化时的 Pushover 通知
- **多目标监控** — 当前仅监控 google.com,如需监控多个端点(如 github.com、自定义服务器)需要新的配置和实现
- **详细的分阶段耗时** — 当前仅记录总耗时,如需 DNS 解析、TCP 连接、TLS 握手各阶段时间需要更复杂的实现
- **自适应检查间隔** — 当前使用固定间隔,如需根据历史连通性动态调整检查间隔需要新的算法

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MONITOR-01 | 定期测试 google.com 的连通性 | 使用 `net/http.Client` 发送 HEAD 请求到 https://www.google.com，`time.Ticker` 实现定期检查 |
| MONITOR-02 | HTTP 请求失败时记录 ERROR 日志 | 检测 HTTP 非 200 响应或网络错误，记录 ERROR 级别日志并包含错误类型分类 |
| MONITOR-03 | HTTP 请求成功时记录 INFO 日志 | 检测 HTTP 200 响应，记录 INFO 级别日志并包含响应时间 |
| MONITOR-06 | 监控间隔和超时可通过配置文件调整 | 在 `config.yaml` 中已有 `monitor.interval` 和 `monitor.timeout` 配置，`MonitorConfig` 已实现 |

</phase_requirements>

## Summary

Phase 26 实现网络连通性监控功能，定期向 https://www.google.com 发送 HTTP HEAD 请求测试连通性，仅将 HTTP 200 OK 视为成功，记录详细的诊断日志（响应时间、状态码、错误类型），追踪上一次连通性状态为 Phase 27 状态变化通知做准备。监控间隔和超时可通过配置文件调整（默认 15 分钟间隔，10 秒超时）。

**Primary recommendation:** 创建独立的 `NetworkMonitor` 结构体（参考 Phase 25 `HealthMonitor` 模式），使用 `net/http.Client` 发送 HEAD 请求，配置 `CheckRedirect` 禁用重定向跟随，使用 `time.Ticker` 定期检查，通过 `ConnectivityState` 追踪上一次状态，在日志中分类错误类型（DNS 失败、连接超时、TLS 错误、HTTP 非 200），在独立 goroutine 中运行并支持通过 context 实现优雅关闭，在 main.go 中健康监控之后启动。

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | Go 1.24 stdlib | HTTP 客户端,发送 HEAD 请求 | Go 标准库,稳定可靠,支持超时和重定向控制 |
| time.Ticker | Go 1.24 stdlib | 定期执行连通性检查 | Go 标准库,goroutine-safe,内置 Stop() 防止泄漏 |
| context.Context | Go 1.24 stdlib | 优雅关闭和超时控制 | Go 标准模式,支持取消信号传播 |
| log/slog | Go 1.24 stdlib | 结构化日志记录 | 项目已使用,记录连通性日志 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| net | Go 1.24 stdlib | 错误类型判断 (net.Error, net.DNSError) | 判断网络错误类型（超时、DNS 失败等） |
| syscall | Go 1.24 stdlib | 系统级错误判断 (ECONNREFUSED) | 判断连接拒绝错误 |
| crypto/tls | Go 1.24 stdlib | TLS 错误判断 | 判断证书错误 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| HEAD 请求 | GET 请求 | HEAD 更高效,不下载响应体,减少流量和耗时 |
| time.Ticker | time.Sleep 循环 | Ticker 更符合 Go 惯例,支持 Stop() 防止 goroutine 泄漏 |
| time.Ticker | cron 库（如 robfig/cron） | cron 更复杂,对于简单定时检查过度设计 |
| 标准 http.Client | 自定义 Transport | 标准客户端已满足需求,无需自定义连接池或代理 |
| 严格 200 OK | 接受 2xx 状态码 | 严格 200 更简单明确,避免歧义,适合基础连通性测试 |

**Installation:**
```bash
# 无需安装新依赖,使用 Go 标准库
```

**Version verification:**
```bash
$ go version
go version go1.24.13 windows/amd64
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── network/
│   ├── monitor.go          # NetworkMonitor 核心实现
│   └── monitor_test.go     # 单元测试
├── config/
│   ├── monitor.go          # 已存在: MonitorConfig 配置结构和验证逻辑
│   └── config.go           # 已存在: Config 结构已集成 Monitor 字段
└── health/
    └── monitor.go          # 已存在: HealthMonitor 参考实现模式
```

### Pattern 1: NetworkMonitor 结构体
**What:** 封装网络连通性监控逻辑,维护上一次连通性状态
**When to use:** 所有网络连通性监控场景
**Example:**
```go
// Source: 基于 Phase 25 HealthMonitor 模式 + Go HTTP 客户端最佳实践
package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
)

// ConnectivityState 连通性状态
type ConnectivityState struct {
	IsConnected bool
	LastCheck   time.Time
}

// NetworkMonitor 网络连通性监控器
type NetworkMonitor struct {
	targetURL  string
	interval   time.Duration
	timeout    time.Duration
	logger     *slog.Logger
	httpClient *http.Client

	// 状态追踪
	state  *ConnectivityState
	ctx    context.Context
	cancel context.CancelFunc
}

// NewNetworkMonitor 创建网络监控器
func NewNetworkMonitor(
	targetURL string,
	interval time.Duration,
	timeout time.Duration,
	logger *slog.Logger,
) *NetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建 HTTP 客户端,禁用重定向跟随
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随重定向
		},
	}

	return &NetworkMonitor{
		targetURL:  targetURL,
		interval:   interval,
		timeout:    timeout,
		logger:     logger.With("component", "network-monitor"),
		httpClient: client,
		state:      nil, // 初始状态为 nil,表示首次检查
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动网络监控循环（在独立 goroutine 中运行）
func (nm *NetworkMonitor) Start() {
	nm.logger.Info("网络监控已启动",
		"target", nm.targetURL,
		"interval", nm.interval,
		"timeout", nm.timeout)

	ticker := time.NewTicker(nm.interval)
	defer ticker.Stop()

	// 立即执行一次初始检查
	nm.checkConnectivity()

	for {
		select {
		case <-nm.ctx.Done():
			nm.logger.Info("网络监控已停止")
			return
		case <-ticker.C:
			nm.checkConnectivity()
		}
	}
}

// checkConnectivity 检查连通性并记录日志
func (nm *NetworkMonitor) checkConnectivity() {
	start := time.Now()
	isConnected, statusCode, errMsg := nm.performCheck()
	duration := time.Since(start)

	// 更新状态
	now := time.Now()
	previousState := nm.state
	nm.state = &ConnectivityState{
		IsConnected: isConnected,
		LastCheck:   now,
	}

	// 记录日志
	if isConnected {
		// MONITOR-03: 成功时记录 INFO
		nm.logger.Info("Google 连通性检查成功",
			"duration", duration.Milliseconds(),
			"status_code", statusCode)
	} else {
		// MONITOR-02: 失败时记录 ERROR
		nm.logger.Error("Google 连通性检查失败",
			"duration", duration.Milliseconds(),
			"error_type", errMsg)
	}

	// 记录状态变化（首次检查、状态保持、状态改变）
	if previousState == nil {
		nm.logger.Info("初始连通性状态",
			"is_connected", isConnected)
	} else if previousState.IsConnected != isConnected {
		if previousState.IsConnected && !isConnected {
			nm.logger.Warn("连通性状态改变: 从连通变为不连通")
		} else {
			nm.logger.Info("连通性状态改变: 从不连通变为连通")
		}
	}
}

// performCheck 执行 HTTP HEAD 请求检查连通性
// 返回: (是否连通, HTTP 状态码, 错误消息)
func (nm *NetworkMonitor) performCheck() (bool, int, string) {
	// 创建请求
	req, err := http.NewRequest(http.MethodHead, nm.targetURL, nil)
	if err != nil {
		return false, 0, fmt.Sprintf("创建请求失败: %v", err)
	}

	// 发送请求
	resp, err := nm.httpClient.Do(req)
	if err != nil {
		// 分类错误类型
		errMsg := nm.classifyError(err)
		return false, 0, errMsg
	}
	defer resp.Body.Close()

	// 仅 HTTP 200 OK 算成功
	if resp.StatusCode == http.StatusOK {
		return true, resp.StatusCode, ""
	}

	// 非 200 状态码视为失败
	errMsg := fmt.Sprintf("HTTP 状态码 %d (%s)", resp.StatusCode, resp.Status)
	return false, resp.StatusCode, errMsg
}

// classifyError 分类错误类型
func (nm *NetworkMonitor) classifyError(err error) string {
	// 解析 URL 错误
	if urlErr, ok := err.(*url.Error); ok {
		// 超时错误
		if urlErr.Timeout() {
			return "连接超时"
		}

		// 解析内层错误
		innerErr := urlErr.Err

		// DNS 解析失败
		if netErr, ok := innerErr.(*net.DNSError); ok {
			return fmt.Sprintf("DNS 解析失败: %s", netErr.Host)
		}

		// 连接拒绝
		if netErr, ok := innerErr.(*net.OpError); ok {
			if syscallErr, ok := netErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.ECONNREFUSED {
					return "连接被拒绝"
				}
			}
		}

		// TLS 握手错误
		if _, ok := innerErr.(tls.CertificateVerificationError); ok {
			return "TLS 证书验证失败"
		}
		if _, ok := innerErr.(x509.UnknownAuthorityError); ok {
			return "TLS 未知证书颁发机构"
		}

		// 其他网络错误
		return fmt.Sprintf("网络错误: %v", innerErr)
	}

	// 未知错误
	return fmt.Sprintf("未知错误: %v", err)
}

// Stop 停止网络监控
func (nm *NetworkMonitor) Stop() {
	nm.logger.Info("正在停止网络监控...")
	nm.cancel()
}

// GetState 获取当前连通性状态（供 Phase 27 使用）
func (nm *NetworkMonitor) GetState() *ConnectivityState {
	return nm.state
}
```

### Pattern 2: 配置文件集成（已存在）
**What:** 在 config.yaml 中已有网络监控配置
**When to use:** 用户需要自定义监控间隔和超时
**Example:**
```yaml
# config.yaml (已存在)
monitor:
  interval: 15m  # Google 连通性检查间隔（默认 15 分钟）
  timeout: 10s   # HTTP 请求超时（默认 10 秒）
```

**Go 配置结构（已存在）:**
```go
// internal/config/monitor.go (已存在)
package config

type MonitorConfig struct {
	Interval time.Duration `yaml:"interval" mapstructure:"interval"`
	Timeout  time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

func (mc *MonitorConfig) Validate() error {
	// Interval validation
	if mc.Interval < 1*time.Minute {
		return fmt.Errorf("monitor.interval must be at least 1 minute, got %v", mc.Interval)
	}

	// Timeout validation
	if mc.Timeout < 1*time.Second {
		return fmt.Errorf("monitor.timeout must be at least 1 second, got %v", mc.Timeout)
	}

	return nil
}

// internal/config/config.go (已存在)
type Config struct {
	Monitor MonitorConfig `yaml:"monitor" mapstructure:"monitor"`
	// ...
}

func (c *Config) defaults() {
	// ...
	c.Monitor.Interval = 15 * time.Minute
	c.Monitor.Timeout = 10 * time.Second
}
```

### Pattern 3: 在 main.go 中启动监控
**What:** 应用启动时在健康监控之后启动网络监控
**When to use:** API 服务器和健康监控启动后启动网络监控
**Example:**
```go
// cmd/nanobot-auto-updater/main.go
func main() {
	// ... 现有代码 ...

	// 创建 InstanceManager
	instanceManager := instance.NewInstanceManager(cfg, logger)

	// 创建并启动 API 服务器
	var apiServer *api.Server
	if cfg.API.Port != 0 {
		apiServer, err = api.NewServer(&cfg.API, instanceManager, logger)
		go func() {
			logger.Info("启动 API 服务器", "port", cfg.API.Port)
			apiServer.Start()
		}()
	}

	// 启动健康监控（Phase 25）
	var healthMonitor *health.HealthMonitor
	if len(cfg.Instances) > 0 {
		healthMonitor = health.NewHealthMonitor(
			cfg.Instances,
			cfg.HealthCheck.Interval,
			logger,
		)
		go healthMonitor.Start()
		logger.Info("健康监控已启动", "interval", cfg.HealthCheck.Interval)
	}

	// 启动网络监控（Phase 26）
	var networkMonitor *network.NetworkMonitor
	networkMonitor = network.NewNetworkMonitor(
		"https://www.google.com",
		cfg.Monitor.Interval,
		cfg.Monitor.Timeout,
		logger,
	)
	go networkMonitor.Start()
	logger.Info("网络监控已启动", "interval", cfg.Monitor.Interval)

	// Auto-start instances in goroutine (non-blocking)
	go func() {
		instanceManager.StartAllInstances(autoStartCtx)
	}()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutdown signal received")

	// 优雅关闭：网络监控先停 → 健康监控后停 → API 最后停
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if networkMonitor != nil {
		networkMonitor.Stop()
	}

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

- **忘记禁用重定向跟随:** 导致跟随 301/302 重定向,无法严格测试 google.com 直接响应
- **忘记调用 ticker.Stop():** 导致 goroutine 泄漏,必须使用 `defer ticker.Stop()`
- **忘记调用 context.CancelFunc:** 导致 context 泄漏,必须使用 `defer cancel()`
- **在主 goroutine 中运行监控循环:** 阻塞应用启动,必须在独立 goroutine 中运行
- **使用 GET 而非 HEAD 请求:** 下载不必要的响应体,浪费流量和耗时
- **接受 2xx 而非严格 200:** 成功标准不明确,可能误判连通性
- **错误分类过于复杂:** 细分过多错误类型增加维护成本,基础分类足够诊断问题
- **不记录响应时间:** 缺少性能诊断信息,无法判断网络延迟问题
- **忘记 resp.Body.Close():** 导致连接泄漏,必须使用 `defer resp.Body.Close()`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP 客户端 | 自定义 HTTP 请求逻辑 | `net/http.Client` | 标准库,稳定可靠,支持超时和重定向控制 |
| 定期任务 | time.Sleep 循环 | `time.Ticker` | 标准库,支持 Stop(),防止 goroutine 泄漏 |
| 优雅关闭 | channel + select | `context.Context` | Go 标准模式,支持超时和取消传播 |
| 错误分类 | 字符串匹配错误消息 | 类型断言 (net.Error, net.DNSError) | 类型断言更可靠,不受错误消息格式变化影响 |
| 禁用重定向 | 自定义 Transport | `http.Client.CheckRedirect` | 标准库提供的重定向控制机制,简单可靠 |

**Key insight:** HTTP 客户端和定时任务使用标准库即可,无需第三方库。错误分类使用 Go 标准的类型断言模式,比字符串匹配更可靠。禁用重定向使用 `CheckRedirect` 是官方推荐方式。

## Common Pitfalls

### Pitfall 1: HTTP 重定向跟随导致误判
**What goes wrong:** 没有禁用重定向,google.com 返回 301/302 时跟随重定向,无法测试直接响应
**Why it happens:** `http.Client` 默认跟随最多 10 次重定向
**How to avoid:** 配置 `CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }`
**Warning signs:** 连通性检查总是成功,即使 google.com 返回 3xx 重定向

### Pitfall 2: Goroutine 泄漏
**What goes wrong:** 忘记调用 `ticker.Stop()` 或 `cancel()`,导致 goroutine 永远不会退出
**Why it happens:** 对 Go 并发原语的清理机制理解不足
**How to avoid:** 始终使用 `defer ticker.Stop()` 和 `defer cancel()`
**Warning signs:** 应用退出时间变长,内存持续增长

### Pitfall 3: HTTP 状态码判断错误
**What goes wrong:** 接受 2xx 或 3xx 状态码视为成功,导致连通性判断不准确
**Why it happens:** 对 HTTP 状态码语义理解不正确
**How to avoid:** 严格只接受 `http.StatusOK` (200),其他状态码一律视为失败
**Warning signs:** 连通性检查成功,但实际 google.com 返回 204 或 304

### Pitfall 4: 响应体未关闭导致连接泄漏
**What goes wrong:** 忘记 `defer resp.Body.Close()`,导致 HTTP 连接无法复用,资源泄漏
**Why it happens:** HEAD 请求没有响应体,误以为不需要关闭
**How to avoid:** 即使是 HEAD 请求,也必须调用 `resp.Body.Close()`
**Warning signs:** 内存持续增长,连接数持续增长

### Pitfall 5: 错误分类使用字符串匹配
**What goes wrong:** 使用 `strings.Contains(err.Error(), "timeout")` 判断错误类型,受错误消息格式变化影响
**Why it happens:** 对 Go 错误处理机制理解不足
**How to avoid:** 使用类型断言 `if netErr, ok := err.(*net.DNSError); ok { ... }`
**Warning signs:** 错误分类失效,日志中错误类型不准确

## Code Examples

Verified patterns from official sources:

### HTTP HEAD 请求 + 禁用重定向
```go
// Source: Stack Overflow + Go 官方文档
// https://stackoverflow.com/questions/23297520/how-can-i-make-the-go-http-client-not-follow-redirects-automatically
// https://pkg.go.dev/net/http

client := &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // 不跟随重定向
	},
}

req, err := http.NewRequest(http.MethodHead, "https://www.google.com", nil)
if err != nil {
	// 处理错误
}

resp, err := client.Do(req)
if err != nil {
	// 处理错误
}
defer resp.Body.Close() // 必须关闭,即使是 HEAD 请求

if resp.StatusCode == http.StatusOK {
	// 连通性成功
} else {
	// 连通性失败
}
```

### 错误类型分类（类型断言）
```go
// Source: Go 官方文档 + 社区最佳实践
// https://pkg.go.dev/net
// https://pkg.go.dev/crypto/tls

func classifyError(err error) string {
	// URL 错误（http.Client 返回的错误包装在 *url.Error 中）
	if urlErr, ok := err.(*url.Error); ok {
		// 超时错误
		if urlErr.Timeout() {
			return "连接超时"
		}

		// DNS 解析失败
		if netErr, ok := urlErr.Err.(*net.DNSError); ok {
			return fmt.Sprintf("DNS 解析失败: %s", netErr.Host)
		}

		// 连接拒绝
		if netErr, ok := urlErr.Err.(*net.OpError); ok {
			if syscallErr, ok := netErr.Err.(*os.SyscallError); ok {
				if syscallErr.Err == syscall.ECONNREFUSED {
					return "连接被拒绝"
				}
			}
		}

		// TLS 证书验证失败
		if _, ok := urlErr.Err.(tls.CertificateVerificationError); ok {
			return "TLS 证书验证失败"
		}

		return fmt.Sprintf("网络错误: %v", urlErr.Err)
	}

	return fmt.Sprintf("未知错误: %v", err)
}
```

### Ticker + Context 优雅关闭模式（参考 Phase 25）
```go
// Source: Go 官方文档 + VictoriaMetrics 最佳实践
// https://victoriametrics.com/blog/go-graceful-shutdown/

func (nm *NetworkMonitor) Start() {
	nm.logger.Info("网络监控已启动")

	ticker := time.NewTicker(nm.interval)
	defer ticker.Stop() // 防止 goroutine 泄漏

	// 立即执行一次初始检查
	nm.checkConnectivity()

	for {
		select {
		case <-nm.ctx.Done():
			// 收到取消信号,退出循环
			nm.logger.Info("网络监控已停止")
			return
		case <-ticker.C:
			// 执行定期检查
			nm.checkConnectivity()
		}
	}
}

func (nm *NetworkMonitor) Stop() {
	nm.cancel() // 通知 goroutine 退出
}
```

### 状态追踪模式（参考 Phase 25）
```go
// Source: Phase 25 HealthMonitor 状态追踪模式

type ConnectivityState struct {
	IsConnected bool
	LastCheck   time.Time
}

type NetworkMonitor struct {
	state  *ConnectivityState // nil 表示首次检查
	// ...
}

func (nm *NetworkMonitor) checkConnectivity() {
	// 执行检查
	isConnected, statusCode, errMsg := nm.performCheck()

	// 记录上一次状态
	previousState := nm.state

	// 更新当前状态
	nm.state = &ConnectivityState{
		IsConnected: isConnected,
		LastCheck:   time.Now(),
	}

	// 检测状态变化
	if previousState == nil {
		// 首次检查
		nm.logger.Info("初始连通性状态", "is_connected", isConnected)
	} else if previousState.IsConnected != isConnected {
		// 状态改变
		if previousState.IsConnected {
			nm.logger.Warn("连通性状态改变: 从连通变为不连通")
		} else {
			nm.logger.Info("连通性状态改变: 从不连通变为连通")
		}
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| GET 请求测试连通性 | HEAD 请求 | 一直推荐 | 减少流量和耗时,更高效 |
| 接受 2xx 状态码 | 严格 200 OK | Phase 26 决策 | 成功标准更明确,避免歧义 |
| 字符串匹配错误 | 类型断言错误 | Go 1.13+ (2019) | 错误分类更可靠,不受错误消息格式变化影响 |
| channel 取消信号 | context.Context | Go 1.7+ (2016) | 标准化取消传播,支持超时和级联取消 |
| time.Sleep 循环 | time.Ticker + select | Go 1.0+ | 支持优雅关闭,防止 goroutine 泄漏 |
| 无状态监控 | 状态追踪 + 状态变化检测 | Phase 25 (2026-03-20) | 为状态变化通知做准备,记录状态变化 |

**Deprecated/outdated:**
- **GET 请求测试连通性:** 下载不必要的响应体,浪费流量和耗时,使用 HEAD 请求替代
- **字符串匹配错误:** 受错误消息格式变化影响,使用类型断言替代
- **time.Sleep in loop:** 不支持优雅关闭,容易导致 goroutine 泄漏,使用 `time.Ticker` + `select` 替代

## Open Questions

1. **是否需要重试机制？**
   - What we know: 单次 HTTP 请求可能因短暂网络波动失败
   - What's unclear: 是否需要连续 N 次失败才判定为不连通
   - Recommendation: Phase 26 先实现基础版本（单次检测）,后续根据实际运行情况决定是否添加重试

2. **错误分类的详细程度？**
   - What we know: 基础错误分类覆盖常见场景（DNS、超时、TLS、HTTP 非 200）
   - What's unclear: 是否需要更细粒度的错误分类（如 TCP RST vs ICMP 不可达）
   - Recommendation: Phase 26 使用基础分类,保持简单。如需更详细的诊断,可在后续 Phase 添加

3. **是否需要监控多个目标？**
   - What we know: 当前仅监控 google.com
   - What's unclear: 未来是否需要监控多个端点（如 github.com、自定义服务器）
   - Recommendation: Phase 26 仅监控 google.com,保持简单。多目标监控已标记为 Deferred,如需实现可在后续 Phase 添加

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — Go test 使用 `*_test.go` 文件 |
| Quick run command | `go test ./internal/network/... -v` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MONITOR-01 | 定期测试 google.com 的连通性 | unit | `go test ./internal/network -run TestCheckConnectivity -v` | ❌ Wave 0 |
| MONITOR-02 | HTTP 请求失败时记录 ERROR 日志 | unit | `go test ./internal/network -run TestCheckConnectivity_Failure -v` | ❌ Wave 0 |
| MONITOR-03 | HTTP 请求成功时记录 INFO 日志 | unit | `go test ./internal/network -run TestCheckConnectivity_Success -v` | ❌ Wave 0 |
| MONITOR-06 | 监控间隔和超时可通过配置文件调整 | unit | `go test ./internal/config -run TestMonitorConfig -v` | ✅ 已存在 |

### Sampling Rate
- **Per task commit:** `go test ./internal/network/... -v`
- **Per wave merge:** `go test ./... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/network/monitor.go` — 核心网络监控实现
- [ ] `internal/network/monitor_test.go` — 单元测试
- [ ] 更新 `cmd/nanobot-auto-updater/main.go` — 添加网络监控启动和关闭逻辑

*(If no gaps: "None — existing test infrastructure covers all phase requirements")*

## Sources

### Primary (HIGH confidence)
- **Go 标准库文档** - `net/http`, `net`, `context`, `time` - 官方推荐的 HTTP 客户端和并发模式
- **Go 1.24 文档** - 标准库 API 和最佳实践
- **项目现有代码** - `internal/health/monitor.go` - HealthMonitor 参考实现模式
- **项目配置** - `internal/config/monitor.go` - MonitorConfig 配置已实现

### Secondary (MEDIUM confidence)
- [Stack Overflow: Disable HTTP Client Redirects](https://stackoverflow.com/questions/23297520/how-can-i-make-the-go-http-client-not-follow-redirects-automatically) - 禁用重定向的标准方法
- [OneUptime: How to Handle HTTP Client Timeouts Properly in Go](https://oneuptime.com/blog/post/2026-02-01-go-http-client-timeouts/view) - HTTP 客户端超时最佳实践
- [OneUptime: How to Set HTTP Client Timeouts in Go](https://oneuptime.com/blog/post/2026-01-23-go-http-timeouts/view) - 永远不要使用 http.DefaultClient,总是设置超时
- [VictoriaMetrics: Graceful Shutdown in Go](https://victoriametrics.com/blog/go-graceful-shutdown/) - 优雅关闭模式,ticker 清理

### Tertiary (LOW confidence)
- [Boldly Go: Error handling in Go web apps](https://boldlygo.tech/posts/2024-01-08-error-handling/) - 错误处理模式参考
- [ITNext: HTTP request timeouts in Go for beginners](https://itnext.io/http-request-timeouts-in-go-for-beginners-fe6445137c90) - 超时配置入门指南

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖都是成熟的 Go 标准库,无需第三方库
- Architecture: HIGH - 基于 Phase 25 HealthMonitor 模式和 Go 标准并发模式,已有成功实现
- Pitfalls: HIGH - 常见的 Go HTTP 客户端和并发陷阱,有大量官方文档和社区经验

**Research date:** 2026-03-21
**Valid until:** 90 天（Go 1.24 稳定,模式成熟）
