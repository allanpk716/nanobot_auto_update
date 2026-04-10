# Phase 46: Service Configuration & Mode Detection - Research

**Researched:** 2026-04-10
**Domain:** Go 配置系统扩展 + Windows 服务环境检测
**Confidence:** HIGH

## Summary

Phase 46 是 v0.11 的第一阶段，需要完成两件事：(1) 在现有 viper 配置系统中新增 `ServiceConfig` 子段（`auto_start`、`service_name`、`display_name`）；(2) 在 `main.go` 入口处调用 `svc.IsWindowsService()` 检测运行环境，决定走服务模式还是控制台模式。

项目已有成熟的子段配置模式（`SelfUpdateConfig`、`APIConfig`、`MonitorConfig`、`HealthCheckConfig`），ServiceConfig 完全遵循同一模式即可。`golang.org/x/sys/windows/svc` 包已在 go.mod 中（v0.41.0），`IsWindowsService()` 是稳定 API（903 个导入者），无需新增任何依赖。

**Primary recommendation:** 创建 `internal/config/service.go` 遵循 `selfupdate.go` 的最小模式，在 `main.go` 的 flag 解析之后、配置加载之前插入 `svc.IsWindowsService()` 检测。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 新增 `ServiceConfig` 子段到 `Config` 结构体，字段：`auto_start *bool`、`service_name string`、`display_name string`
- **D-02:** `auto_start` 默认 `false`，未配置时行为与当前完全一致（控制台模式）
- **D-03:** 预留 `service_name` 和 `display_name` 字段（Phase 48 服务注册需要），默认值分别为 `"NanobotAutoUpdater"` 和 `"Nanobot Auto Updater"`
- **D-04:** 使用 `mapstructure:"service"` 标签，遵循项目现有 viper+mapstructure 模式
- **D-05:** 创建独立文件 `internal/config/service.go`，与 `api.go`、`selfupdate.go` 等同结构
- **D-06:** 启动时序：先调用 `svc.IsWindowsService()` 检测环境，再加载配置。服务模式路径不需要先读 config.yaml
- **D-07:** SCM 启动 + `auto_start: false` 时：以服务模式正常运行，但记录 WARN 日志提醒配置已变更（Phase 48 会处理自动卸载）
- **D-08:** 控制台运行 + `auto_start: true` 时：检测管理员权限，自动注册 Windows 服务后以退出码 `2` 退出
- **D-09:** 退出码 `2` 表示"注册服务后退出"，与正常退出 `0` 区分
- **D-10:** `service_name` 格式校验：仅允许字母数字，无空格（SCM 要求）
- **D-11:** `display_name` 长度限制：最大 256 字符（SCM 限制）
- **D-12:** 仅当 `auto_start: true` 时校验 `service_name` 和 `display_name`；`auto_start: false` 时跳过
- **D-13:** 测试范围：只测 ServiceConfig 的配置解析、默认值、验证逻辑。svc.IsWindowsService() 的集成测试留给 Phase 47

### Claude's Discretion
- ServiceConfig 具体的 Go struct 定义细节
- 验证错误消息的具体措辞
- WARN 日志的格式和内容

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MGR-01 | config.yaml 新增 `auto_start: true/false` 配置项 | ServiceConfig 结构体 + viper 集成模式（见"架构模式"） |
| SVC-01 | 程序启动时通过 svc.IsWindowsService() 检测运行模式，自动选择服务模式或控制台模式 | svc 包 API + main.go 插入点（见"代码示例"） |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sys/windows/svc` | v0.41.0 (go.mod) | Windows 服务环境检测 + 服务运行 | Go 官方扩展库，903 个导入者，已存在于项目依赖中 |
| `github.com/spf13/viper` | v1.20.1 (go.mod) | YAML 配置解析 | 项目已有依赖，所有配置子段使用同一模式 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `regexp` (stdlib) | Go 1.24 | service_name 格式校验 | Validate() 中校验仅字母数字 [D-10] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `svc.IsWindowsService()` | `svc.IsAnInteractiveSession()` | 后者已 deprecated [VERIFIED: pkg.go.dev] |
| 自定义环境检测 | WMI 查询父进程 | 复杂度高且不可靠，官方 API 已足够 |

**Installation:**
无需安装新依赖 — `golang.org/x/sys v0.41.0` 已在 go.mod 中。

**Version verification:**
```
go.mod 中 golang.org/x/sys v0.41.0 — 已确认存在
pkg.go.dev 最新版 v0.43.0 (2026-03-27) — 项目版本略旧但完全可用
```

## Architecture Patterns

### Recommended Project Structure
```
internal/config/
├── config.go          # Config 根结构体 — 新增 Service 字段
├── service.go         # NEW: ServiceConfig + Validate()
├── service_test.go    # NEW: ServiceConfig 表格驱动测试
├── selfupdate.go      # 参考: 最小子段模式
└── api.go             # 参考: 带验证的子段模式

cmd/nanobot-auto-updater/
└── main.go            # 入口点 — 新增 svc.IsWindowsService() 检测
```

### Pattern 1: 子段配置模式 (Sub-config Pattern)
**What:** 每个配置子段 = 独立文件 + 结构体 + Validate() 方法
**When to use:** 所有新配置子段
**Example:**
```go
// Source: internal/config/selfupdate.go (项目实际代码)
package config

import "fmt"

type ServiceConfig struct {
    AutoStart   *bool  `yaml:"auto_start" mapstructure:"auto_start"`
    ServiceName string `yaml:"service_name" mapstructure:"service_name"`
    DisplayName string `yaml:"display_name" mapstructure:"display_name"`
}

func (s *ServiceConfig) Validate() error {
    // 仅 auto_start 为 true 时校验 [D-12]
    if s.AutoStart != nil && *s.AutoStart {
        // service_name 校验 [D-10]
        // display_name 校验 [D-11]
    }
    return nil
}
```

### Pattern 2: *bool 指针用于区分"未设置"和"false"
**What:** `*bool` 允许区分 nil（未配置，用默认值）、true、false 三种状态
**When to use:** 配置项需要"未配置"语义时
**Example:**
```go
// Source: internal/config/instance.go (项目实际代码)
type InstanceConfig struct {
    AutoStart *bool `mapstructure:"auto_start"` // nil = default true
}

func (ic *InstanceConfig) ShouldAutoStart() bool {
    if ic.AutoStart == nil {
        return true // default
    }
    return *ic.AutoStart
}
```

### Pattern 3: viper 集成（defaults + Unmarshal）
**What:** 在 `config.Load()` 中设置 viper defaults，然后 Unmarshal 到结构体
**When to use:** 所有新增子段
**Example:**
```go
// Source: internal/config/config.go Load() 函数 (项目实际代码)
// 模式: 在 defaults() 中设置结构体默认值
func (c *Config) defaults() {
    c.Service.AutoStart = nil // nil 表示 false [D-02]
    c.Service.ServiceName = "NanobotAutoUpdater"
    c.Service.DisplayName = "Nanobot Auto Updater"
}

// 在 Load() 中设置 viper defaults
v.SetDefault("service.auto_start", nil)     // 不设默认，viper 会用零值
v.SetDefault("service.service_name", cfg.Service.ServiceName)
v.SetDefault("service.display_name", cfg.Service.DisplayName)

// Validate 链式调用
func (c *Config) Validate() error {
    // ...
    if err := c.Service.Validate(); err != nil {
        errs = append(errs, err)
    }
    return errors.Join(errs...)
}
```

### Pattern 4: svc.IsWindowsService() 入口检测
**What:** 在 main() 最前面检测是否运行在 Windows 服务上下文
**When to use:** 程序启动时模式选择
**Example:**
```go
// Source: golang.org/x/sys/windows/svc/example/main.go [VERIFIED: pkg.go.dev]
inService, err := svc.IsWindowsService()
if err != nil {
    log.Fatalf("failed to determine if we are running in service: %v", err)
}
if inService {
    // 服务模式路径 [D-06]: 不需要加载 config.yaml
    runService(svcName, false)
    return
}
// 控制台模式: 正常加载配置 + 运行
```

### Anti-Patterns to Avoid
- **在服务模式路径中先加载配置**: 按 D-06 决策，服务模式路径在 IsWindowsService() 检测后直接进入 svc.Run()，不先读 config.yaml
- **使用 `IsAnInteractiveSession()`**: 已 deprecated [VERIFIED: pkg.go.dev svc 包文档]
- **`service_name` 允许特殊字符**: SCM 只接受字母数字和有限特殊字符，D-10 限制为仅字母数字
- **auto_start 默认 true**: D-02 明确默认 false，与 InstanceConfig.AutoStart 的默认 true 不同

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Windows 服务环境检测 | 自己检查父进程或 session ID | `svc.IsWindowsService()` | 官方 API，处理了所有边缘情况 |
| 配置解析 | 手动解析 YAML | viper + mapstructure | 项目已有模式，处理类型转换和默认值 |
| 服务名验证的正则 | 手写循环检查 | `regexp.MustCompile` | 标准库，可读性好 |

**Key insight:** `svc.IsWindowsService()` 内部通过检查进程 session ID 是否为 0 来判断是否在服务上下文中运行。这个检测是可靠的（不在容器中运行的情况下，本项目目标就是原生 Windows）。

## Common Pitfalls

### Pitfall 1: *bool 与 viper 默认值的交互
**What goes wrong:** viper 的 `SetDefault("service.auto_start", nil)` 不会将零值 bool 映射为 nil 指针；当 YAML 中未写 `auto_start` 时，viper Unmarshal 会产生 `*bool = nil`（正确），但如果写 `auto_start: false` 则为 `*bool = &false`
**Why it happens:** viper 对 pointer 类型有特殊处理：YAML 中不存在的 key 映射为 nil，存在的 key（包括 false）映射为指针
**How to avoid:** 测试覆盖三种情况：未配置(nil)、true、false。参考 instance_test.go 中 `TestInstanceConfigAutoStart` 的 `ptrBool` 辅助函数模式 [VERIFIED: internal/config/instance_test.go]
**Warning signs:** 默认值行为与预期不符

### Pitfall 2: svc.IsWindowsService() 在容器中返回 false
**What goes wrong:** 在 Windows 容器中（如 servercore:ltsc2019），IsWindowsService() 始终返回 false [CITED: github.com/golang/go/issues/56335]
**Why it happens:** 容器内的 session 隔离与原生 Windows 不同
**How to avoid:** 本项目目标运行环境是原生 Windows，不是容器。此 pitfall 仅在测试/开发时需注意
**Warning signs:** 在 Docker 中测试服务模式时检测失败

### Pitfall 3: main.go 中 os.Exit(2) 跳过 defer
**What goes wrong:** D-08 要求在注册服务后 `os.Exit(2)` 退出，但 `os.Exit` 会跳过所有 defer
**Why it happens:** `os.Exit` 直接终止进程，不执行 defer
**How to avoid:** 确保注册服务 + 退出逻辑在 main() 中不依赖 defer 清理（注册时还没有初始化任何资源）
**Warning signs:** 资源泄漏在 os.Exit 路径上

### Pitfall 4: svc 包仅在 Windows 可用
**What goes wrong:** 直接 `import "golang.org/x/sys/windows/svc"` 会导致跨平台编译失败
**Why it happens:** svc 包内部使用 Windows syscall
**How to avoid:** 方案一（推荐）：由于整个项目已限定 Windows（main.go 无 build tag 但 internal 包大量使用 `//go:build windows`），main.go 也应有条件编译；方案二：将 svc 检测封装到 `internal/lifecycle/` 包中，使用 `//go:build windows` build tag。注意 D-06 决策的时序要求
**Warning signs:** `go build` 在非 Windows 平台失败

### Pitfall 5: service_name 校验正则过于宽松或严格
**What goes wrong:** SCM 对服务名的限制是：字母、数字、斜杠(/)、反斜杠(\)和点(.)，但 D-10 决策限制为仅字母数字
**Why it happens:** D-10 是用户明确决策，比 SCM 限制更严格
**How to avoid:** 使用 `^[a-zA-Z0-9]+$` 正则，严格遵循 D-10
**Warning signs:** 用户配置了含特殊字符的服务名被拒绝

## Code Examples

### Example 1: ServiceConfig 结构体定义 (遵循 selfupdate.go 模式)
```go
// Source: 模式参照 internal/config/selfupdate.go [VERIFIED]
// internal/config/service.go
package config

import (
    "fmt"
    "regexp"
)

// ServiceConfig holds configuration for Windows service mode.
type ServiceConfig struct {
    AutoStart   *bool  `yaml:"auto_start" mapstructure:"auto_start"`
    ServiceName string `yaml:"service_name" mapstructure:"service_name"`
    DisplayName string `yaml:"display_name" mapstructure:"display_name"`
}

var serviceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Validate validates the ServiceConfig values.
func (s *ServiceConfig) Validate() error {
    // 仅当 auto_start 为 true 时校验 [D-12]
    if s.AutoStart == nil || !*s.AutoStart {
        return nil
    }

    // service_name 格式校验 [D-10]
    if !serviceNameRegex.MatchString(s.ServiceName) {
        return fmt.Errorf("service.service_name must contain only alphanumeric characters, got %q", s.ServiceName)
    }

    // display_name 长度校验 [D-11]
    if len(s.DisplayName) > 256 {
        return fmt.Errorf("service.display_name must be at most 256 characters, got %d", len(s.DisplayName))
    }
    if len(s.DisplayName) == 0 {
        return fmt.Errorf("service.display_name is required when auto_start is true")
    }

    return nil
}
```

### Example 2: Config 集成
```go
// Source: 模式参照 internal/config/config.go [VERIFIED]

// Config 结构体新增字段
type Config struct {
    // ... existing fields ...
    Service ServiceConfig `yaml:"service" mapstructure:"service"`
}

// defaults() 新增
func (c *Config) defaults() {
    // ... existing defaults ...
    // Service defaults [D-02, D-03]
    c.Service.AutoStart = nil // nil = false, 未配置时行为与当前一致
    c.Service.ServiceName = "NanobotAutoUpdater"
    c.Service.DisplayName = "Nanobot Auto Updater"
}

// Load() 新增 viper defaults
v.SetDefault("service.service_name", cfg.Service.ServiceName)
v.SetDefault("service.display_name", cfg.Service.DisplayName)

// Validate() 新增
if err := c.Service.Validate(); err != nil {
    errs = append(errs, err)
}
```

### Example 3: main.go 入口检测 (参照 svc/example/main.go [VERIFIED: pkg.go.dev])
```go
// Source: golang.org/x/sys/windows/svc/example/main.go [VERIFIED: pkg.go.dev]
// 插入位置: flag.Parse() 之后、config.Load() 之前

import "golang.org/x/sys/windows/svc"

// 在 main() 中，flag.Parse() 之后:
inService, err := svc.IsWindowsService()
if err != nil {
    fmt.Fprintf(os.Stderr, "Failed to detect service mode: %v\n", err)
    os.Exit(1)
}
if inService {
    // 服务模式路径 [D-06]
    // Phase 47 会在此处调用 svc.Run()
    // Phase 46 仅记录日志后退出（服务模式完整实现留给 Phase 47）
    log.Println("Running as Windows service")
    // 临时: 直接运行控制台逻辑，Phase 47 会替换为 svc.Run()
    // 注意: 这是 Phase 46 的临时行为，Phase 47 实现完整 svc.Handler 后会改变
}
```

### Example 4: 表格驱动测试 (参照 instance_test.go [VERIFIED])
```go
// Source: 模式参照 internal/config/instance_test.go [VERIFIED]
// internal/config/service_test.go

func TestServiceConfigValidate(t *testing.T) {
    tests := []struct {
        name        string
        config      ServiceConfig
        expectError bool
        errorMatch  string
    }{
        {
            name:        "auto_start nil skips validation",
            config:      ServiceConfig{AutoStart: nil, ServiceName: "", DisplayName: ""},
            expectError: false,
        },
        {
            name:        "auto_start false skips validation",
            config:      ServiceConfig{AutoStart: ptrBool(false), ServiceName: "", DisplayName: ""},
            expectError: false,
        },
        {
            name:        "auto_start true valid config",
            config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "MyService", DisplayName: "My Service"},
            expectError: false,
        },
        {
            name:        "auto_start true invalid service_name with space",
            config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "My Service", DisplayName: "My Service"},
            expectError: true,
            errorMatch:  "service_name",
        },
        {
            name:        "auto_start true display_name too long",
            config:      ServiceConfig{AutoStart: ptrBool(true), ServiceName: "MyService", DisplayName: strings.Repeat("a", 257)},
            expectError: true,
            errorMatch:  "display_name",
        },
    }
    // ... t.Run() 循环 ...
}

func ptrBool(v bool) *bool { return &v }
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `svc.IsAnInteractiveSession()` | `svc.IsWindowsService()` | golang.org/x/sys 早期版本 | IsAnInteractiveSession 已 deprecated [VERIFIED: pkg.go.dev] |

**Deprecated/outdated:**
- `svc.IsAnInteractiveSession()`: deprecated，使用 `svc.IsWindowsService()` 替代 [VERIFIED: pkg.go.dev svc 包文档]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `auto_start` 为 nil 时 viper Unmarshal 会产生 `*bool = nil`（与 InstanceConfig.AutoStart 行为一致） | Architecture Patterns | 如果 viper 不这样处理，需要额外逻辑判断"未配置" |
| A2 | main.go 不需要 `//go:build windows` tag（因为项目本身就是 Windows-only，lifecycle 包使用了 tag 但 cmd 入口未用） | Pitfall 4 | 如果需要跨平台编译 main.go，需要条件编译封装 |
| A3 | Phase 46 的 main.go 修改不需要实际调用 svc.Run()，只需添加检测逻辑，svc.Run() 留给 Phase 47 | Code Examples | 如果 Phase 46 需要完整的服务模式运行，工作量会增加 |

## Open Questions

1. **main.go 的 build tag**
   - What we know: `internal/lifecycle/daemon.go` 使用 `//go:build windows`，但 `cmd/nanobot-auto-updater/main.go` 没有使用
   - What's unclear: main.go 中新增 `import "golang.org/x/sys/windows/svc"` 后，是否需要添加 build tag 或条件编译
   - Recommendation: 由于 main.go 已经导入了 `internal/lifecycle`（其子文件有 build tag），且项目在 Go 1.24 上运行，最简方案是将 svc 检测封装到 `internal/lifecycle/` 中的一个新文件（如 `servicedetect.go`），使用 `//go:build windows` tag，并提供一个非 Windows 的 stub 文件。或者，考虑到项目完全是 Windows-only，直接在 main.go 中导入也可以

2. **Phase 46 的服务模式路径具体行为**
   - What we know: D-06 决策说服务模式路径不需要先读 config.yaml
   - What's unclear: Phase 46 是否需要在服务模式路径中做任何事，还是只做检测 + 分支标记
   - Recommendation: Phase 46 的服务模式路径应该是：检测到 inService=true 后记录日志，然后继续执行（或按 Phase 47 的需要调整）。因为 Phase 47 才实现 svc.Handler

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go runtime | 编译和测试 | Yes | go1.24.11 | -- |
| golang.org/x/sys | svc.IsWindowsService() | Yes (go.mod) | v0.41.0 | -- |
| regexp (stdlib) | service_name 校验 | Yes | Go 1.24 | -- |

**Missing dependencies with no fallback:**
None

**Missing dependencies with fallback:**
None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (assert/require) |
| Config file | none |
| Quick run command | `go test ./internal/config/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MGR-01 | ServiceConfig 解析 auto_start 字段 | unit | `go test ./internal/config/... -run TestServiceConfig -v` | Wave 0 |
| MGR-01 | ServiceConfig 默认值正确 | unit | `go test ./internal/config/... -run TestServiceConfig_Defaults -v` | Wave 0 |
| MGR-01 | ServiceConfig.Validate() 校验 service_name 格式 | unit | `go test ./internal/config/... -run TestServiceConfigValidate -v` | Wave 0 |
| MGR-01 | ServiceConfig.Validate() 校验 display_name 长度 | unit | `go test ./internal/config/... -run TestServiceConfigValidate -v` | Wave 0 |
| MGR-01 | ServiceConfig 仅 auto_start=true 时校验 | unit | `go test ./internal/config/... -run TestServiceConfigValidate -v` | Wave 0 |
| MGR-01 | ServiceConfig 通过 viper Load() 集成 | unit | `go test ./internal/config/... -run TestServiceConfig_ViperLoad -v` | Wave 0 |
| SVC-01 | svc.IsWindowsService() 检测逻辑插入 main.go | manual-only | N/A (Phase 47 集成测试) | N/A |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/config/service_test.go` -- covers MGR-01 ServiceConfig 解析、默认值、验证逻辑
- [ ] SVC-01 的集成测试标记为 manual-only，Phase 47 实现 svc.Handler 时补充

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | yes | D-08: 控制台运行 + auto_start=true 时需检测管理员权限 |
| V5 Input Validation | yes | service_name 正则校验 + display_name 长度校验 [D-10, D-11] |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for Go + Windows Service

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| 配置文件篡改 | Tampering | 文件权限控制（config.yaml 仅管理员可写） |
| 服务名注入 | Tampering | 正则校验仅允许字母数字 [D-10] |
| 未授权服务注册 | Spoofing | 检测管理员权限后注册 [D-08] |

## Sources

### Primary (HIGH confidence)
- `internal/config/config.go` -- Config 根结构体、Load() 函数、viper 模式 [VERIFIED: 代码读取]
- `internal/config/selfupdate.go` -- 最小子段配置参考 [VERIFIED: 代码读取]
- `internal/config/api.go` -- 带验证的子段配置参考 [VERIFIED: 代码读取]
- `internal/config/instance.go` -- *bool 指针模式参考 [VERIFIED: 代码读取]
- `cmd/nanobot-auto-updater/main.go` -- 程序入口 [VERIFIED: 代码读取]
- `go.mod` -- 依赖确认 golang.org/x/sys v0.41.0 [VERIFIED: 代码读取]
- [pkg.go.dev/golang.org/x/sys/windows/svc](https://pkg.go.dev/golang.org/x/sys/windows/svc) -- svc 包官方 API 文档 [VERIFIED]

### Secondary (MEDIUM confidence)
- [svc/example/main.go](https://docs-go.hexacode.org/src/golang.org/x/sys/windows/svc/example/main.go) -- 官方 IsWindowsService() 使用示例 [CITED]
- [GitHub Issue #56335](https://github.com/golang/go/issues/56335) -- IsWindowsService() 在容器中的已知限制 [CITED]

### Tertiary (LOW confidence)
None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 所有依赖已存在于 go.mod，API 已通过 pkg.go.dev 确认
- Architecture: HIGH - 项目已有 5+ 个子段配置的成熟模式，ServiceConfig 完全复用
- Pitfalls: HIGH - *bool 模式在 InstanceConfig 中已有验证，svc 包限制已知
- Testing: HIGH - 表格驱动测试模式在项目中广泛使用（api_test.go、instance_test.go）

**Research date:** 2026-04-10
**Valid until:** 2026-05-10 (Go 配置模式和 svc API 非常稳定)
