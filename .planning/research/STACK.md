# Stack Research

**Domain:** HTTP API 服务和监控服务 (v0.3 新增功能)
**Researched:** 2026-03-16
**Confidence:** HIGH

## Executive Summary

v0.3 里程碑将项目从定时更新工具转变为监控服务 + HTTP API 触发更新模式。研究表明：

1. **HTTP API 服务器** - 使用 Go 标准库 `net/http`，无需框架。单个 POST 端点 + 简单 Bearer Token 认证足够。
2. **Google 连通性监控** - 使用 `net/http` + `context.WithTimeout()` 实现带超时的 HTTP GET 请求。
3. **零新增依赖** - 所有功能通过 Go 1.24.11 标准库实现，与现有 viper/logrus 生态系统完美集成。

**关键决策：避免过度工程化。** 社区共识明确指向标准库优先，仅在复杂场景考虑框架。

---

## Recommended Stack

### Core Technologies (v0.3 新增)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go 标准库 `net/http` | 1.24.11+ (现有) | HTTP API 服务器 | Go 社区普遍避免使用框架。标准库稳定、向后兼容、无依赖风险。Go 1.22+ 增强的 `http.ServeMux` 支持方法匹配和路径变量。 |
| Go 标准库 `context` | 1.24.11+ (现有) | 超时和取消控制 | Go 惯用的超时处理方式，支持跨 API 边界传播取消信号。比 `http.Client.Timeout` 更灵活，避免 goroutine 泄漏。 |
| 现有 Pushover 库 | `github.com/gregdel/pushover` v1.4.0 (现有) | 连通性失败/恢复通知 | 项目已集成，用于监控失败时发送通知。配置从环境变量迁移到 YAML 配置文件。 |
| `time.Ticker` | 标准库 (现有) | 定时监控调度 | 替代 cron 库，实现每 15 分钟检查 Google 连通性。简单直接，无需第三方调度库。 |

### Supporting Libraries (无需新增)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| 无需额外库 | - | Bearer Token 认证 | 手写中间件验证 `Authorization: Bearer <token>`，无需 JWT 或第三方认证库。 |
| 无需额外库 | - | HTTP 客户端监控 | 使用 `net/http` + `context.WithTimeout()` 实现带超时的 HTTP GET 监控。 |
| 无需额外库 | - | 健康检查端点 | 简单 `/health` 端点返回 200 OK，无需第三方健康检查库。 |
| `encoding/json` | 标准库 (现有) | API 响应 JSON 编码 | 用于 POST `/api/v1/trigger-update` 返回结构化响应。 |

### Development Tools (现有)

| Tool | Purpose | Notes |
|------|---------|-------|
| Go 1.24.11 | 编译和运行时 | 项目当前版本，标准库功能完整 |
| `log/slog` | 结构化日志 | 项目已实现自定义格式，API 服务日志保持一致 |
| `github.com/spf13/viper` | 配置管理 | 现有配置库，用于加载 API 和监控配置 |

## Installation

```bash
# v0.3 无需安装额外依赖
# 所有新功能通过 Go 标准库实现

# 将移除的依赖 (v0.3):
# - github.com/robfig/cron/v3  # 不再需要定时调度

# 现有依赖保持不变:
# - github.com/gregdel/pushover (通知)
# - github.com/spf13/viper (配置)
# - golang.org/x/sys (Windows 系统调用)
# - gopkg.in/natefinch/lumberjack.v2 (日志轮转)
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| 标准库 `net/http` | Gin/Echo/Chi/Fiber 框架 | 当需要复杂路由、验证框架、大量中间件时考虑框架。本项目仅需单个 POST 端点，标准库足够。 |
| 手写 Bearer Token 中间件 | `github.com/auth0/go-jwt-middleware` | 当需要 JWT 验证、OAuth2、复杂认证流程时使用。本项目仅需简单 token 比对。 |
| `net/http` + context | `github.com/projectdiscovery/retryablehttp` | 当需要自动重试、指数退避、复杂请求策略时使用。Google 监控失败仅需记录日志和通知。 |
| 简单健康检查端点 | `github.com/core-go/health` / `github.com/brpaz/go-healthcheck` | 当需要检查数据库、Redis、多个外部依赖时使用。本项目仅需基础健康状态。 |
| `time.Ticker` | 保持 `robfig/cron` | 当需要 cron 表达式灵活调度时保持。v0.3 改为固定 15 分钟间隔，`time.Ticker` 更简单。 |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| HTTP 框架 (Gin/Echo/Fiber) | 过度工程化，增加依赖风险，单个 POST 端点不需要框架 | 标准 `net/http` + `http.ServeMux` |
| JWT 认证库 | Bearer token 仅需简单字符串比对，JWT 库过重 | 手写 10 行中间件验证 token |
| `http.Client.Timeout` 字段 | 不会传播到 Request.Context，可能导致 goroutine 泄漏 | `context.WithTimeout()` + `req.WithContext()` |
| 重试库 (retryablehttp, go-retry) | Google 监控失败后仅记录日志和通知，不需要自动重试 | 简单 `http.Get()` + 错误处理 |
| 第三方健康检查库 | 项目无需检查数据库、Redis 等复杂依赖 | `/health` 端点返回 `{"status": "ok"}` |
| `http.DefaultClient` | 超时为 0 (无超时)，生产环境可能导致永久挂起 | 创建自定义 `http.Client{Timeout: 10s}` |
| `robfig/cron` (v0.3 移除) | 监控服务使用固定 15 分钟间隔，`time.Ticker` 更轻量 | `time.NewTicker(15 * time.Minute)` |

## Stack Patterns by Variant

### Pattern 1: HTTP API 服务器 (标准库)

**实现方式:**
```go
// 简单的 Bearer Token 认证中间件
func authMiddleware(expectedToken string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader != "Bearer "+expectedToken {
                http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// API 处理器
func (s *Server) handleTriggerUpdate(w http.ResponseWriter, r *http.Request) {
    // Go 1.22+ ServeMux 已确保仅 POST 方法到达此处
    // 触发更新逻辑
    err := s.updater.TriggerUpdate(r.Context())
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "error",
            "error":  err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "triggered",
    })
}

// 路由设置 (Go 1.22+ 方法匹配语法)
mux := http.NewServeMux()
mux.HandleFunc("POST /api/v1/trigger-update", s.handleTriggerUpdate)
mux.HandleFunc("GET /health", s.handleHealth)

// 应用中间件
handler := authMiddleware(cfg.API.AuthToken)(mux)

// 启动服务器
server := &http.Server{
    Addr:         fmt.Sprintf(":%d", cfg.API.Port),
    Handler:      handler,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
}
```

**为什么不用框架:**
- Gin/Echo 提供 Router、Validator、Middleware 生态系统，但本项目仅 1 个端点
- 社区共识：简单服务用标准库，复杂应用考虑框架 (来源: Three Dots Labs, Reddit 讨论)
- Go 1.22+ ServeMux 已支持方法匹配、路径变量，功能差距缩小

### Pattern 2: Google 连通性监控 (带超时)

**实现方式:**
```go
type Monitor struct {
    client    *http.Client
    targetURL string
    interval  time.Duration
    notifier  *notifier.Notifier
    logger    *slog.Logger
}

func NewMonitor(cfg MonitoringConfig, notifier *notifier.Notifier, logger *slog.Logger) *Monitor {
    return &Monitor{
        client: &http.Client{
            Timeout: cfg.Timeout, // 整体超时 (10s)
        },
        targetURL: cfg.TargetURL,
        interval:  cfg.CheckInterval,
        notifier:  notifier,
        logger:    logger,
    }
}

func (m *Monitor) Start(ctx context.Context) {
    ticker := time.NewTicker(m.interval)
    defer ticker.Stop()

    var lastStatus bool = true // 初始假设连通

    for {
        select {
        case <-ctx.Done():
            m.logger.Info("监控服务停止")
            return
        case <-ticker.C:
            isConnected := m.checkConnectivity(ctx)

            // 状态变化时发送通知
            if isConnected != lastStatus {
                if isConnected {
                    m.logger.Info("Google 连通性恢复")
                    m.notifier.NotifySuccess("Google 连通性监控", "网络已恢复")
                } else {
                    m.logger.Error("Google 连通性失败")
                    // NotifyFailure 会在内部记录错误
                }
                lastStatus = isConnected
            }
        }
    }
}

func (m *Monitor) checkConnectivity(ctx context.Context) bool {
    // 使用 context 控制超时 (优先于 Client.Timeout)
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", m.targetURL, nil)
    if err != nil {
        m.logger.Error("创建监控请求失败", "error", err)
        return false
    }

    resp, err := m.client.Do(req)
    if err != nil {
        m.logger.Warn("Google 连通性检查失败", "error", err)
        return false
    }
    defer resp.Body.Close()

    // 2xx 状态码视为成功
    return resp.StatusCode >= 200 && resp.StatusCode < 300
}
```

**为什么不用重试库:**
- Google 监控失败后不需要自动重试 (15 分钟后会再次检查)
- 失败时仅需记录日志 + 发送通知，业务逻辑简单
- `retryablehttp` 增加依赖但无法提供额外价值

### Pattern 3: 配置结构扩展

**新增配置 (v0.3):**
```yaml
# HTTP API 服务配置
api:
  enabled: true
  port: 8080
  auth_token: "your-secure-token-here"  # Bearer token

# Google 连通性监控配置
monitoring:
  enabled: true
  check_interval: 15m  # 每 15 分钟检查一次
  target_url: "https://www.google.com"
  timeout: 10s

# Pushover 配置 (从环境变量迁移到配置文件)
pushover:
  api_token: "your-api-token"
  user_key: "your-user-key"

# 移除 cron 配置 (v0.3 不再使用)
# cron: "0 3 * * *"  # DEPRECATED - 使用 HTTP API 触发更新
```

**Go 配置结构:**
```go
type APIConfig struct {
    Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
    Port      int    `yaml:"port" mapstructure:"port"`
    AuthToken string `yaml:"auth_token" mapstructure:"auth_token"`
}

type MonitoringConfig struct {
    Enabled       bool          `yaml:"enabled" mapstructure:"enabled"`
    CheckInterval time.Duration `yaml:"check_interval" mapstructure:"check_interval"`
    TargetURL     string        `yaml:"target_url" mapstructure:"target_url"`
    Timeout       time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

type Config struct {
    API        APIConfig        `yaml:"api" mapstructure:"api"`
    Monitoring MonitoringConfig `yaml:"monitoring" mapstructure:"monitoring"`
    // ... 现有字段 (instances, pushover)
}
```

**为什么不用配置验证库:**
- 现有 `viper` 已提供 YAML 解析 + mapstructure 标签
- 配置验证逻辑简单 (端口范围、URL 格式)，手写足够
- 避免引入 `go-playground/validator` 等额外依赖

## Integration with Existing Codebase

### 集成点 1: main.go 启动逻辑

**现有逻辑 (v0.2):**
```go
// 定时调度器
scheduler := scheduler.NewScheduler(cfg.Cron, logger)
scheduler.Schedule(updater.Update)
```

**v0.3 新逻辑:**
```go
// 1. 启动 HTTP API 服务器
if cfg.API.Enabled {
    apiServer := api.NewServer(cfg.API, updater, logger)
    go func() {
        logger.Info("启动 HTTP API 服务器", "port", cfg.API.Port)
        if err := apiServer.Start(); err != nil {
            logger.Error("API 服务器失败", "error", err)
        }
    }()
}

// 2. 启动监控服务
if cfg.Monitoring.Enabled {
    monitor := monitoring.NewMonitor(cfg.Monitoring, notifier, logger)
    go monitor.Start(context.Background())
}

// 3. 移除 cron 调度器 (不再需要)
```

### 集成点 2: config/config.go 扩展

**新增默认值:**
```go
func (c *Config) defaults() {
    // 现有默认值...
    c.API.Enabled = true
    c.API.Port = 8080
    c.API.AuthToken = "" // 必须在配置文件中设置

    c.Monitoring.Enabled = true
    c.Monitoring.CheckInterval = 15 * time.Minute
    c.Monitoring.TargetURL = "https://www.google.com"
    c.Monitoring.Timeout = 10 * time.Second
}
```

**新增验证:**
```go
func (c *Config) Validate() error {
    var errs []error

    // 现有验证...

    // API 配置验证
    if c.API.Enabled {
        if c.API.Port < 1 || c.API.Port > 65535 {
            errs = append(errs, fmt.Errorf("api.port must be 1-65535, got %d", c.API.Port))
        }
        if c.API.AuthToken == "" {
            errs = append(errs, fmt.Errorf("api.auth_token is required when api.enabled=true"))
        }
    }

    // 监控配置验证
    if c.Monitoring.Enabled {
        if c.Monitoring.CheckInterval < 1*time.Minute {
            errs = append(errs, fmt.Errorf("monitoring.check_interval must be >= 1m, got %v", c.Monitoring.CheckInterval))
        }
        if _, err := url.Parse(c.Monitoring.TargetURL); err != nil {
            errs = append(errs, fmt.Errorf("monitoring.target_url invalid: %w", err))
        }
    }

    return errors.Join(errs...)
}
```

### 集成点 3: notifier/notifier.go 变更

**现有逻辑 (v0.2):**
```go
// New() 从环境变量读取 PUSHOVER_TOKEN 和 PUSHOVER_USER
func New(logger *slog.Logger) *Notifier {
    token := os.Getenv("PUSHOVER_TOKEN")
    user := os.Getenv("PUSHOVER_USER")
    // ...
}
```

**v0.3 逻辑 (配置文件优先):**
```go
// NewWithConfig() 从配置文件读取，回退到环境变量
func NewWithConfig(cfg PushoverConfig, logger *slog.Logger) *Notifier {
    token := cfg.ApiToken
    user := cfg.UserKey

    // 回退到环境变量 (向后兼容)
    if token == "" {
        token = os.Getenv("PUSHOVER_TOKEN")
    }
    if user == "" {
        user = os.Getenv("PUSHOVER_USER")
    }

    if token == "" || user == "" {
        logger.Warn("Pushover 通知未配置")
        return &Notifier{enabled: false, logger: logger}
    }

    logger.Info("Pushover 通知已启用")
    return &Notifier{
        client:    pushover.New(token),
        recipient: pushover.NewRecipient(user),
        logger:    logger,
        enabled:   true,
    }
}
```

## Version Compatibility

| Package | Version | Compatible With | Notes |
|---------|---------|-----------------|-------|
| Go 1.24.11 | 标准库 | 项目当前版本 | `net/http`, `context`, `time.Ticker` 全部兼容 |
| `net/http` | Go 1.22+ | Go 1.24.11 | ServeMux 方法匹配语法 `POST /path` 在 Go 1.22 引入 |
| `context` | Go 1.0+ | Go 1.24.11 | `http.Request.WithContext()` 从 Go 1.0 支持 |
| `time.Ticker` | Go 1.0+ | Go 1.24.11 | 标准库定时器 |
| `encoding/json` | Go 1.0+ | Go 1.24.11 | JSON 编解码 |

## Dependencies to Remove (v0.3)

| Package | Why Remove | Replacement |
|---------|-----------|-------------|
| `github.com/robfig/cron/v3` | v0.3 不再需要 cron 表达式调度 | `time.Ticker` (固定 15 分钟间隔) |

## Sources

### HTTP Server & Standard Library

- [Choosing a Go Web Framework in 2026: A Minimalist's Guide](https://medium.com/@samayun_pathan/choosing-a-go-web-framework-in-2026-a-minimalists-guide-to-gin-fiber-chi-echo-and-beego-c79b31b8474d) — MEDIUM confidence (社区实践)
- [When You Shouldn't Use Frameworks in Go (Three Dots Labs)](https://threedots.tech/episode/when-you-should-not-use-frameworks/) — HIGH confidence (权威博客)
- [Why I Only Rely on the Go Standard Library for HTTP Services](https://blog.stackademic.com/why-i-only-rely-on-the-go-standard-library-for-http-services-a52868cdf816) — MEDIUM confidence (实践经验)
- [Go's http.ServeMux Is All You Need](https://dev.to/leapcell/gos-httpservemux-is-all-you-need-1mam) — MEDIUM confidence (Go 1.22+ 特性)
- [pkg.go.dev/net/http (官方文档)](https://pkg.go.dev/net/http) — HIGH confidence (Go 官方)

### Timeouts & Context

- [The complete guide to Go net/http timeouts (Cloudflare Blog)](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — HIGH confidence (权威指南)
- [Go http client timeout vs context timeout (Stack Overflow)](https://stackoverflow.com/questions/64129364/go-http-client-timeout-vs-context-timeout) — HIGH confidence (社区共识)
- [Timeouts in Go: A Comprehensive Guide (Better Stack)](https://betterstack.com/community/guides/scaling-go/golang-timeouts/) — MEDIUM confidence (最佳实践总结)
- [GitHub Issue: Client.Timeout not propagated to Request Context](https://github.com/golang/go/issues/31657) — HIGH confidence (已知问题)

### Middleware & Authentication

- [How to Implement Middleware in Go Web Applications (OneUptime, 2026-01)](https://oneuptime.com/blog/post/2026-01-26-go-middleware/view) — HIGH confidence (2026 最新文章)
- [OlegGorj/golang-bearer-token (GitHub)](https://github.com/oleggorj/golang-bearer-token) — MEDIUM confidence (代码示例)
- [Top 5 Authentication Solutions for Secure Go Apps in 2026 (WorkOS)](https://workos.com/blog/top-authentication-solutions-go-2026) — MEDIUM confidence (认证方案对比)

### Monitoring & Health Checks

- [core-go/health (GitHub)](https://github.com/core-go/health) — LOW confidence (可选库调研)
- [brpaz/go-healthcheck (GitHub)](https://github.com/brpaz/go-healthcheck) — LOW confidence (可选库调研)
- [How to Implement Health Checks in Go for Kubernetes (OneUptime)](https://oneuptime.com/blog/post/2026-01-07-go-health-checks-kubernetes/view) — MEDIUM confidence (健康检查最佳实践)

---
*Stack research for: v0.3 HTTP API 服务和监控服务*
*Researched: 2026-03-16*
*Confidence: HIGH (基于 Go 官方文档、Cloudflare 权威指南、社区共识)*
