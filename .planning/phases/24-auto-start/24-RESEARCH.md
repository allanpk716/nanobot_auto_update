# Phase 24: Auto-start - Research

**Researched:** 2026-03-20
**Domain:** Golang 应用启动自动化、并发模式、结构化日志
**Confidence:** HIGH

## Summary

Phase 24 需要在应用启动时自动启动所有配置的实例，实现无用户干预的自动化启动流程。研究涵盖 Golang 应用启动模式、并发启动的最佳实践、结构化日志模式以及配置验证。

**Primary recommendation:** 复用现有 InstanceManager 和 InstanceLifecycle 基础设施，添加 StartAllInstances() 方法，在 main.go 的 API 服务器启动后通过 goroutine 异步执行串行启动流程。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### 配置选项
- **实例级别 auto_start 开关**
  - 在 `InstanceConfig` 中添加 `AutoStart bool` 字段（默认 `true`）
  - 实例配置示例：
    ```yaml
    instances:
      - name: "gateway"
        port: 18790
        start_command: "python -m nanobot.gateway"
        auto_start: false  # 跳过自动启动
      - name: "worker"
        port: 18791
        start_command: "python -m nanobot.worker"
        # auto_start 默认 true，无需显式配置
    ```
  - `auto_start: false` 的实例在启动阶段被跳过
  - 无全局开关，简化实现

#### 启动时机
- **API 服务器启动后启动**
  - 启动顺序：配置加载 → InstanceManager 创建 → API 服务器启动 → 自动启动实例
  - API 服务器先准备好，用户可以通过 `/api/v1/status` 等端点查看启动状态
  - 实例启动在 goroutine 中执行，不阻塞 API 服务器启动
  - 实现位置：在 `main.go` 中，API 服务器启动 goroutine 之后启动实例

#### 失败重试策略
- **无重试机制**
  - 实例启动失败后直接记录错误，继续启动其他实例
  - 简单实现，避免复杂的重试逻辑
  - 失败信息通过日志记录（Phase 25 健康监控会持续检查实例状态）
  - 用户可以通过手动触发更新（Phase 28 HTTP API）重新启动失败的实例

#### 日志输出格式
- **每个实例启动时记录 INFO**
  - 启动前：`Starting instance "gateway" (port=18790)...`
  - 启动后：`Instance "gateway" started successfully (duration=2.3s)`
  - 包含实例名、端口、耗时
- **失败实例的 ERROR 详情**
  - 错误日志：`Failed to start instance "worker" (port=18791): <InstanceError details>`
  - 包含实例名、端口、错误原因、底层错误
  - 使用 `InstanceError` 结构化错误（继承 Phase 7）
- **汇总日志**
  - 所有实例启动完成后：`Auto-start completed: 2 started, 1 failed (failed: [worker])`
  - 包含成功数量、失败数量、失败实例名称列表

#### 失败后的应用行为
- **继续运行应用**
  - 即使部分实例启动失败，应用仍然继续运行
  - 已启动的实例正常提供服务（如 API 访问、SSE 流式日志）
  - 失败信息通过日志记录，不退出应用
  - Phase 25 健康监控会定期检查实例状态并记录日志

### Claude's Discretion
- 日志消息的具体措辞（中文/英文）
- 汇总日志的格式（图标、对齐、颜色）
- 耗时的格式（秒 vs 毫秒）
- 实例启动 goroutine 的错误处理（如何记录 panic 等异常）

### Deferred Ideas (OUT OF SCOPE)
None — 讨论保持在阶段范围内

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTOSTART-01 | 应用启动时自动启动所有配置的实例 | InstanceManager.StartAllInstances() 方法实现，main.go 在 API 启动后调用 |
| AUTOSTART-02 | 每个实例按配置顺序依次启动 | 串行启动模式（参考 Phase 8 startAll 方法），遍历 instances slice |
| AUTOSTART-03 | 实例启动失败时记录错误并继续启动其他实例 | 优雅降级模式（参考 Phase 8 stopAll/startAll），失败记录到 StartFailed 数组 |
| AUTOSTART-04 | 所有实例启动完成后记录汇总状态 | AutoStartResult 结构体（复用 UpdateResult 模式），汇总日志格式 |

</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.13 | 运行时环境 | 项目标准，支持最新 log/slog 和 context 特性 |
| log/slog | 1.24+ | 结构化日志 | 标准库，已有自定义 handler 实现（logging.go） |
| context | 1.24+ | 超时和取消 | 标准库模式，现有代码已全面采用 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time | 标准库 | 启动耗时统计 | 每个实例启动前后记录时间戳 |
| fmt | 标准库 | 日志格式化 | 汇总日志格式化输出 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| 串行启动 | 并发启动（errgroup） | 并发启动更复杂，需要额外同步机制；串行启动简单可靠，符合 Phase 8 确定的模式 |

**Installation:**
无新增依赖，全部使用标准库。

**Version verification:**
```
go version go1.24.13 windows/amd64
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── instance/
│   ├── manager.go          # InstanceManager 添加 StartAllInstances() 方法
│   ├── lifecycle.go        # InstanceLifecycle 现有 StartAfterUpdate() 方法
│   ├── errors.go           # InstanceError 复用现有结构
│   └── result.go           # 复用 UpdateResult 或创建 AutoStartResult
├── config/
│   └── instance.go         # InstanceConfig 添加 AutoStart 字段
cmd/
└── nanobot-auto-updater/
    └── main.go             # 在 API 服务器启动后调用自动启动
```

### Pattern 1: 串行启动 + 优雅降级
**What:** 按配置顺序依次启动实例，失败不中断流程
**When to use:** 应用启动时自动启动所有实例
**Example:**
```go
// Source: internal/instance/manager.go (参考 startAll 方法)
func (m *InstanceManager) StartAllInstances(ctx context.Context) *AutoStartResult {
	m.logger.Info("Starting auto-start phase", "instance_count", len(m.instances))

	result := &AutoStartResult{}
	startTime := time.Now()

	for _, inst := range m.instances {
		// Skip instances with auto_start: false
		if !inst.config.AutoStart {
			m.logger.Info("Skipping instance (auto_start=false)",
				"instance", inst.config.Name,
				"port", inst.config.Port)
			result.Skipped = append(result.Skipped, inst.config.Name)
			continue
		}

		// Record start time for this instance
		instStart := time.Now()
		m.logger.Info("Starting instance",
			"instance", inst.config.Name,
			"port", inst.config.Port)

		if err := inst.StartAfterUpdate(ctx); err != nil {
			m.logger.Error("Failed to start instance",
				"error", err,
				"port", inst.config.Port)
			// 记录失败但不返回,继续启动其他实例
			result.StartFailed = append(result.StartFailed, err.(*InstanceError))
		} else {
			duration := time.Since(instStart)
			m.logger.Info("Instance started successfully",
				"instance", inst.config.Name,
				"port", inst.config.Port,
				"duration", duration)
			result.Started = append(result.Started, inst.config.Name)
		}
	}

	// Log summary
	totalDuration := time.Since(startTime)
	m.logger.Info("Auto-start completed",
		"started", len(result.Started),
		"failed", len(result.StartFailed),
		"skipped", len(result.Skipped),
		"failed_instances", extractNames(result.StartFailed),
		"total_duration", totalDuration)

	return result
}
```

### Pattern 2: 异步启动不阻塞主流程
**What:** 在 goroutine 中执行实例启动，不阻塞 API 服务器启动
**When to use:** main.go 中 API 服务器启动后
**Example:**
```go
// Source: cmd/nanobot-auto-updater/main.go (API 服务器启动后添加)
// Start API server in goroutine
go func() {
	logger.Info("Starting API server", "port", cfg.API.Port)
	if err := apiServer.Start(); err != nil {
		logger.Error("API server error", "error", err)
	}
}()

// Auto-start instances in goroutine (non-blocking)
go func() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in auto-start goroutine",
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	// Create context with timeout for auto-start
	autoStartCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	instanceManager.StartAllInstances(autoStartCtx)
}()
```

### Pattern 3: 配置验证和默认值
**What:** 在配置加载时验证 auto_start 字段并设置默认值
**When to use:** InstanceConfig 验证
**Example:**
```go
// Source: internal/config/instance.go (修改 Validate 方法)
type InstanceConfig struct {
	Name           string        `mapstructure:"name"`
	Port           uint32        `mapstructure:"port"`
	StartCommand   string        `mapstructure:"start_command"`
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`
	AutoStart      bool          `mapstructure:"auto_start"` // 新增字段
}

// 在 config.Load() 中设置默认值
v.SetDefault("instances[].auto_start", true) // 默认 true

// Validate 无需额外验证，AutoStart 是布尔类型
```

### Anti-Patterns to Avoid
- **并发启动**: 使用 errgroup 或 sync.WaitGroup 并发启动 - 增加复杂度，串行启动已足够（Phase 8 确定的模式）
- **全局开关**: 添加全局 auto_start 配置 - 增加配置复杂度，实例级别控制更灵活
- **重试机制**: 失败后自动重试 - 增加复杂度，依赖健康监控和手动触发更新
- **阻塞主流程**: 在主 goroutine 中同步启动实例 - 会阻塞 API 服务器启动，影响用户体验

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 实例启动逻辑 | 自定义启动代码 | InstanceLifecycle.StartAfterUpdate() | 已有完整实现，支持超时、日志、错误处理 |
| 错误结构 | 自定义错误类型 | InstanceError | 已有结构化错误，支持 InstanceName/Port/Operation/Err |
| 日志格式 | 自定义日志格式化 | slog.With() + 现有 logging handler | 已有自定义 handler，支持 "timestamp - [LEVEL]: message" 格式 |
| 结果汇总 | 自定义结果结构 | 复用 UpdateResult 或创建 AutoStartResult | UpdateResult 已有 Stopped/Started/StopFailed/StartFailed 结构 |

**Key insight:** Phase 7-8 已建立完善的实例生命周期管理基础设施，Phase 24 只需扩展配置和添加启动入口，无需重新设计核心逻辑。

## Common Pitfalls

### Pitfall 1: 配置字段未设置默认值
**What goes wrong:** 如果 auto_start 字段未在配置文件中指定，viper 可能解析为零值 false，导致实例意外跳过自动启动
**Why it happens:** viper 的 mapstructure 不会自动设置默认值，除非显式配置 SetDefault
**How to avoid:** 在 config.Load() 中使用 `v.SetDefault("instances[].auto_start", true)` 设置默认值
**Warning signs:** 实例未自动启动，日志显示 "Skipping instance (auto_start=false)" 但配置文件未显式设置 auto_start

### Pitfall 2: goroutine panic 未捕获
**What goes wrong:** 自动启动 goroutine 中的 panic 会导致整个应用崩溃，无任何日志记录
**Why it happens:** goroutine 中的 panic 不会传播到主 goroutine，需要显式 recover
**How to avoid:** 在自动启动 goroutine 中添加 defer recover() 并记录 panic 和 stack trace
**Warning signs:** 应用启动后立即退出，无错误日志

### Pitfall 3: 启动超时未设置
**What goes wrong:** 如果某个实例启动卡死，整个自动启动流程会无限期阻塞
**Why it happens:** StartAfterUpdate() 使用实例的 startup_timeout，但如果上下文未设置超时，可能永远等待
**How to avoid:** 在 goroutine 中创建 context.WithTimeout() 设置总体超时（建议 5 分钟，根据实例数量调整）
**Warning signs:** 自动启动流程长时间未完成，日志停留在 "Starting instance" 无后续输出

### Pitfall 4: 日志顺序混乱
**What goes wrong:** 多个实例的启动日志可能交错，难以阅读
**Why it happens:** 串行启动本身不会交错，但如果未来改为并发启动可能有问题
**How to avoid:** 保持串行启动模式，日志自然按顺序输出
**Warning signs:** 日志中实例启动信息无序，同一实例的 "Starting" 和 "started" 日志不连续

### Pitfall 5: 配置验证遗漏
**What goes wrong:** 添加 AutoStart 字段后未更新 Validate() 方法，可能接受无效配置
**Why it happens:** 布尔类型无需验证，容易被忽略
**How to avoid:** 虽然布尔类型无需验证，但要确保配置加载时设置了正确的默认值
**Warning signs:** 配置文件中的 auto_start 字段被忽略或解析为错误的值

## Code Examples

Verified patterns from official sources and existing codebase:

### 实例启动方法（复用现有代码）
```go
// Source: internal/instance/lifecycle.go (L86-113)
func (il *InstanceLifecycle) StartAfterUpdate(ctx context.Context) error {
	il.logger.Info("Starting instance after update")

	// INST-05: Clear LogBuffer on restart (fresh start)
	il.logBuffer.Clear()

	// Handle default startup timeout
	startupTimeout := il.config.StartupTimeout
	if startupTimeout == 0 {
		startupTimeout = 30 * time.Second // Default: 30 seconds
		il.logger.Debug("Using default startup timeout", "timeout", startupTimeout)
	}

	// Start the instance using lifecycle package with instance-specific command and port
	if err := lifecycle.StartNanobotWithCapture(ctx, il.config.StartCommand, il.config.Port, startupTimeout, il.logger, il.logBuffer); err != nil {
		il.logger.Error("Failed to start instance", "error", err)
		return &InstanceError{
			InstanceName: il.config.Name,
			Operation:    "start",
			Port:         il.config.Port,
			Err:          fmt.Errorf("failed to start instance: %w", err),
		}
	}

	il.logger.Info("Instance started successfully with log capture")
	return nil
}
```

### 配置默认值设置（参考现有模式）
```go
// Source: internal/config/config.go (L125-141) - 扩展模式
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	cfg := New()

	// Set defaults for instances (需要针对 array of struct 的默认值)
	// 注意: viper 对 array of struct 的默认值处理有限，需要在 Unmarshal 后处理
	// 或者在 InstanceConfig 中添加 GetAutoStart() 方法返回默认值

	// ... existing defaults ...

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal to struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Post-process: Set default for AutoStart (因为 viper 不支持 array of struct 的默认值)
	for i := range cfg.Instances {
		// 如果配置文件未指定 auto_start，viper 会解析为零值 false
		// 我们需要区分 "未指定" 和 "显式设置为 false"
		// 解决方案: 使用 *bool 或在 mapstructure 中使用自定义解码器
		// 简单方案: 默认 true，只有显式 false 才跳过
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}
```

### 日志模式（参考现有实现）
```go
// Source: internal/logging/logging.go (L64-86) - 日志格式: "2024-01-01 12:00:00.123 - [INFO]: message"
func NewLogger(logDir string) *slog.Logger {
	dailyWriter, err := newDailyRotateWriter(logDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create daily rotate writer: %v\n", err)
		return slog.New(&simpleHandler{w: os.Stdout})
	}

	multiWriter := io.MultiWriter(dailyWriter, os.Stdout)
	handler := &simpleHandler{w: multiWriter}

	return slog.New(handler)
}

// 使用模式（现有代码已采用）
logger.Info("Auto-start completed",
	"started", len(result.Started),
	"failed", len(result.StartFailed),
	"skipped", len(result.Skipped),
	"failed_instances", extractNames(result.StartFailed),
	"total_duration", totalDuration)
```

### Context 超时模式（参考 WebSearch 最佳实践）
```go
// Source: Context, Graceful Shutdown, and Retry/Timeout Patterns (dev.to)
// 创建带超时的 context，自动取消长时间运行的操作
autoStartCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel() // Always pair with defer to prevent context leak

// Pass context down the call chain
instanceManager.StartAllInstances(autoStartCtx)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| fmt.Printf 日志 | slog 结构化日志 | Go 1.21+ (2023) | 支持日志级别、结构化字段、自定义 handler |
| context.TODO() | context.WithTimeout() | Go 1.7+ (2016) | 支持超时控制，防止无限阻塞 |
| 手动启动实例 | 自动启动（Phase 24） | Phase 24 (2026-03) | 无用户干预，应用启动即可用 |

**Deprecated/outdated:**
- log.Printf: 不支持结构化日志，已被 slog 替代（项目已全面采用 slog）
- 无 context 的启动方法: 无法控制超时，所有启动方法已支持 context 参数

## Open Questions

1. **AutoStart 默认值处理**
   - What we know: viper 对 array of struct 的默认值处理有限，需要在 Unmarshal 后处理
   - What's unclear: 如何区分 "未指定 auto_start" 和 "显式设置为 false"
   - Recommendation: 使用指针类型 `*bool`，nil 表示未指定（默认 true），false 表示显式跳过；或在 config 包添加 post-process 函数

2. **自动启动超时总时长**
   - What we know: 每个实例有 startup_timeout（默认 30s），但总体启动流程需要上限
   - What's unclear: 总体超时应该设置为多少（取决于实例数量）
   - Recommendation: 建议设置为 5 分钟（`5 * len(cfg.Instances)` 个实例 * 30s 每实例 ≈ 2.5 分钟，留缓冲）

3. **日志消息语言**
   - What we know: 项目使用中文日志（参考 InstanceError.Error() 方法）
   - What's unclear: 自动启动日志是否使用中文还是英文
   - Recommendation: 保持与现有代码一致，使用中文日志（但实例名、端口等字段保持英文）

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (标准库) |
| Config file | none - 使用测试代码构造配置 |
| Quick run command | `go test ./internal/instance -run TestStartAllInstances -v` |
| Full suite command | `go test ./internal/instance -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTOSTART-01 | 应用启动时自动启动所有配置的实例 | unit | `go test ./internal/instance -run TestStartAllInstances -v` | ❌ Wave 0 |
| AUTOSTART-02 | 每个实例按配置顺序依次启动 | unit | `go test ./internal/instance -run TestStartAllInstancesOrder -v` | ❌ Wave 0 |
| AUTOSTART-03 | 实例启动失败时记录错误并继续启动其他实例 | unit | `go test ./internal/instance -run TestStartAllInstancesGracefulDegradation -v` | ❌ Wave 0 |
| AUTOSTART-04 | 所有实例启动完成后记录汇总状态 | unit | `go test ./internal/instance -run TestStartAllInstancesSummary -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/instance -run TestStartAllInstances -v`
- **Per wave merge:** `go test ./internal/instance -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/instance/manager_test.go` — 添加 TestStartAllInstances（覆盖 AUTOSTART-01, 02, 03, 04）
- [ ] `internal/instance/manager_test.go` — 添加 TestStartAllInstancesOrder（验证启动顺序）
- [ ] `internal/instance/manager_test.go` — 添加 TestStartAllInstancesGracefulDegradation（验证失败不中断）
- [ ] `internal/instance/manager_test.go` — 添加 TestStartAllInstancesSummary（验证汇总日志）
- [ ] `internal/config/instance_test.go` — 添加 TestAutoStartDefaultValue（验证默认值 true）
- [ ] `cmd/nanobot-auto-updater/main_test.go` — 添加 TestAutoStartIntegration（集成测试）

*(现有测试基础设施完善，只需添加 Phase 24 相关测试)*

## Sources

### Primary (HIGH confidence)
- Existing codebase (internal/instance/manager.go, lifecycle.go) - 实例管理模式和优雅降级策略
- Existing codebase (internal/config/config.go, instance.go) - 配置加载和验证模式
- Existing codebase (internal/logging/logging.go) - 日志格式和 handler 实现

### Secondary (MEDIUM confidence)
- [Goroutine Patterns: Building Efficient Concurrent Code in Go](https://dev.to/shrsv/goroutine-patterns-building-efficient-concurrent-code-in-go-26i3) - 并发模式最佳实践
- [Context, Graceful Shutdown, and Retry/Timeout Patterns](https://dev.to/serifcolakel/building-resilient-go-services-context-graceful-shutdown-and-retrytimeout-patterns-21g3) - Context 超时和优雅关闭模式
- [Logging in Go with Slog: A Practitioner's Guide](https://www.dash0.com/guides/logging-in-go-with-slog) - 结构化日志模式和高级特性

### Tertiary (LOW confidence)
- [How to Use Context in Go for Cancellation and Timeouts](https://oneuptime.com/blog/post/2026-01-23-go-context/view) - Context 最佳实践（2026-01，需验证）
- [Gracefully Cancel Long-Running Goroutines with Context](https://oneuptime.com/blog/post/2026-01-25-gracefully-cancel-goroutines-context-go/view) - Goroutine 取消模式（2026-01，需验证）

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 全部使用现有标准库和项目依赖，无新增包
- Architecture: HIGH - 复用 Phase 7-8 确定的模式，串行启动 + 优雅降级
- Pitfalls: HIGH - 基于现有代码模式和 WebSearch 最佳实践

**Research date:** 2026-03-20
**Valid until:** 3 个月（Go 1.24 稳定，log/slog 和 context 模式成熟）
