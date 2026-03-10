# Phase 6: 配置扩展 - Research

**Researched:** 2026-03-10
**Domain:** Go YAML configuration extension, validation patterns, backward compatibility
**Confidence:** HIGH

## Summary

Phase 6 需要扩展现有的 YAML 配置结构以支持多实例定义,同时保持与 v1.0 配置文件的向后兼容性。核心挑战在于:(1) 设计清晰的配置结构支持实例数组;(2) 实现高效的唯一性验证(名称和端口);(3) 提供详细的错误消息帮助用户快速定位问题;(4) 确保新旧配置格式的互斥检测。

**Primary recommendation:** 使用 mapstructure 标签定义 InstanceConfig 结构体,通过 map-based 去重检查唯一性,使用 Go 1.20+ 的 errors.Join 聚合验证错误,采用"新配置优先,旧配置后备"的向后兼容模式。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- 新增 `instances` 数组字段到 Config 结构,每个元素包含 name、port、start_command、startup_timeout 字段
- **配置模式选择**: 新配置优先,旧配置作为后备
  - 如果 `instances` 数组存在,使用多实例模式
  - 如果 `instances` 不存在,使用现有的 `nanobot` section (单实例模式)
  - 两种模式互斥,配置验证时检查并报错
- 保持现有 `nanobot` section 不变,用于向后兼容 v1.0 配置文件

**必填字段:**
- `name`: 实例名称 (string),用于标识实例和日志输出
- `port`: 实例端口号 (uint32),必须在 1-65535 范围内
- `start_command`: 实例启动命令 (string),用户指定完整的启动命令

**可选字段:**
- `startup_timeout`: 实例启动超时时间 (duration),不配置时使用全局的 nanobot.startup_timeout

**验证时机**: 程序启动时,配置加载后立即验证
**验证内容**:
  1. 检查 `instances` 和 `nanobot` section 是否同时存在(不允许)
  2. 检查所有实例的 `name` 字段唯一性
  3. 检查所有实例的 `port` 字段唯一性
  4. 验证每个实例的必填字段是否存在
  5. 验证 `port` 在有效范围内 (1-65535)
  6. 验证 `startup_timeout` 格式和最小值(5秒)

**错误消息格式**: 详细错误消息,列出所有重复项及其位置
- 名称重复: "配置验证失败: 实例名称重复 - \"instance1\" 出现在第 2 和第 5 个实例配置中"
- 端口重复: "配置验证失败: 端口重复 - 18790 出现在实例 \"instance1\" 和 \"instance2\" 中"
- 字段缺失: "配置验证失败: 实例 \"instance1\" 缺少必填字段 \"start_command\""

**验证失败处理**: 检测到配置验证错误时,打印错误消息并退出(exit code 1)

### Claude's Discretion

- InstanceConfig 结构体的具体命名(例如 InstanceConfig vs NanobotInstance)
- 错误消息的具体措辞(只要保持详细和清晰)
- 验证逻辑的实现方式(循环遍历 vs 使用 map 去重)

### Deferred Ideas (OUT OF SCOPE)

None — 讨论保持在阶段范围内
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CONF-01 | Instance configuration (YAML) - Users can define multiple instances using instances array in config.yaml, each instance contains name, port, start_command fields | Standard Stack: viper + mapstructure tags 支持 YAML 数组结构;<br>Architecture Patterns: InstanceConfig 结构定义见"Code Examples" |
| CONF-02 | Instance name validation - Detect duplicate instance names on startup, fail fast with clear error message | Standard Stack: Go map 去重模式;<br>Architecture Patterns: 见"Code Examples" validateUniqueNames 函数 |
| CONF-03 | Port validation - Detect duplicate ports on startup, fail fast with clear error message | Standard Stack: Go map 去重模式;<br>Architecture Patterns: 见"Code Examples" validateUniquePorts 函数 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/viper | v1.21.0 | Configuration management | 项目已使用,成熟的配置解决方案,支持 YAML/mapstructure |
| gopkg.in/yaml.v3 | (indirect) | YAML parsing | viper 底层使用,yaml.v3 支持 YAML 1.2 标准 |
| github.com/go-viper/mapstructure/v2 | v2.4.0 | Struct field mapping | viper 必需,支持 struct tags 和类型转换 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| errors (stdlib) | Go 1.20+ | Error aggregation | 使用 `errors.Join` 聚合多个验证错误 |
| fmt (stdlib) | - | Error message formatting | 所有错误消息格式化 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| mapstructure tags | yaml tags | yaml tags 不被 viper 支持,会导致 unmarshal 失败 |
| errors.Join | hashicorp/go-multierror | go-multierror 功能更丰富,但增加依赖;errors.Join (Go 1.20+) 足够本场景使用 |
| map-based duplicate check | go-playground/validator unique tag | validator 库更重,引入学习成本;map-based 简单直接,易于控制错误消息格式 |

**Installation:**
```bash
# 无需安装新依赖,使用现有依赖
# 现有依赖已足够支持此阶段实现
```

## Architecture Patterns

### Recommended Project Structure
```
internal/config/
├── config.go           # 扩展 Config 结构,添加 Instances 字段
├── instance.go         # 新增: InstanceConfig 定义和验证逻辑
└── config_test.go      # 扩展: 添加实例配置测试用例
```

### Pattern 1: InstanceConfig Structure Definition
**What:** 定义实例配置结构体,使用 mapstructure 标签支持 viper unmarshal
**When to use:** CONF-01 实现的基础结构
**Example:**
```go
// Source: 项目现有模式 (internal/config/config.go) + viper 官方推荐
type InstanceConfig struct {
    Name           string        `mapstructure:"name"`
    Port           uint32        `mapstructure:"port"`
    StartCommand   string        `mapstructure:"start_command"`
    StartupTimeout time.Duration `mapstructure:"startup_timeout"`
}

// Config 扩展
type Config struct {
    Cron      string           `mapstructure:"cron"`
    Nanobot   NanobotConfig    `mapstructure:"nanobot"`    // 保留向后兼容
    Instances []InstanceConfig `mapstructure:"instances"`  // 新增多实例支持
    Pushover  PushoverConfig   `mapstructure:"pushover"`
}
```

### Pattern 2: Backward Compatibility Check
**What:** 检测新旧配置模式互斥性,确保用户不会同时使用两种配置
**When to use:** Load() 函数中,Validate() 之前
**Example:**
```go
// Source: CONTEXT.md 锁定决策 + 项目现有验证模式
func (c *Config) ValidateModeCompatibility() error {
    hasLegacy := c.Nanobot.Port != 0 || c.Nanobot.StartupTimeout != 0
    hasInstances := len(c.Instances) > 0

    if hasLegacy && hasInstances {
        return fmt.Errorf("配置错误: 不能同时使用 'nanobot' section 和 'instances' 数组,请选择其中一种配置模式")
    }
    return nil
}
```

### Pattern 3: Unique Field Validation with Detailed Errors
**What:** 使用 map 检测重复值,构建详细的错误消息
**When to use:** CONF-02 和 CONF-03 实现
**Example:**
```go
// Source: Go 社区最佳实践 (map-based duplicate detection)
// https://stackoverflow.com/questions/34111476/finding-unique-items-in-a-go-slice-or-array

func validateUniqueNames(instances []InstanceConfig) error {
    seen := make(map[string]int) // name -> first index

    for i, inst := range instances {
        if inst.Name == "" {
            return fmt.Errorf("配置验证失败: 第 %d 个实例缺少必填字段 \"name\"", i+1)
        }

        if firstIdx, exists := seen[inst.Name]; exists {
            return fmt.Errorf("配置验证失败: 实例名称重复 - %q 出现在第 %d 和第 %d 个实例配置中",
                inst.Name, firstIdx+1, i+1)
        }
        seen[inst.Name] = i
    }
    return nil
}

func validateUniquePorts(instances []InstanceConfig) error {
    portToInstance := make(map[uint32]string) // port -> instance name

    for _, inst := range instances {
        if inst.Port == 0 || inst.Port > 65535 {
            return fmt.Errorf("配置验证失败: 实例 %q 的端口必须在 1-65535 范围内,当前值: %d",
                inst.Name, inst.Port)
        }

        if existingName, exists := portToInstance[inst.Port]; exists {
            return fmt.Errorf("配置验证失败: 端口重复 - %d 出现在实例 %q 和 %q 中",
                inst.Port, existingName, inst.Name)
        }
        portToInstance[inst.Port] = inst.Name
    }
    return nil
}
```

### Pattern 4: InstanceConfig Validation Method
**What:** 单个实例配置的验证方法,遵循项目现有 Validate() 模式
**When to use:** 扩展项目现有的验证链
**Example:**
```go
// Source: 项目现有模式 (internal/config/config.go:43-51)
func (ic *InstanceConfig) Validate() error {
    if ic.Name == "" {
        return fmt.Errorf("实例 name 字段不能为空")
    }
    if ic.Port == 0 || ic.Port > 65535 {
        return fmt.Errorf("实例 %q 的端口必须 > 0 且 <= 65535,当前值: %d", ic.Name, ic.Port)
    }
    if ic.StartCommand == "" {
        return fmt.Errorf("实例 %q 缺少必填字段 \"start_command\"", ic.Name)
    }
    if ic.StartupTimeout != 0 && ic.StartupTimeout < 5*time.Second {
        return fmt.Errorf("实例 %q 的 startup_timeout 必须至少 5 秒,当前值: %v",
            ic.Name, ic.StartupTimeout)
    }
    return nil
}
```

### Pattern 5: Aggregated Validation with errors.Join
**What:** 聚合所有验证错误,一次性报告所有问题
**When to use:** Config.Validate() 扩展,支持多实例验证
**Example:**
```go
// Source: Go 1.20+ 标准库 errors.Join
// https://medium.com/@virtualik/error-handling-patterns-in-go-every-developer-should-know-8962777c935b

func (c *Config) Validate() error {
    var errs []error

    // 验证 cron 表达式
    if err := ValidateCron(c.Cron); err != nil {
        errs = append(errs, err)
    }

    // 检查配置模式兼容性
    if err := c.ValidateModeCompatibility(); err != nil {
        errs = append(errs, err)
    }

    // 根据模式选择验证路径
    if len(c.Instances) > 0 {
        // 多实例模式验证
        if err := validateUniqueNames(c.Instances); err != nil {
            errs = append(errs, err)
        }
        if err := validateUniquePorts(c.Instances); err != nil {
            errs = append(errs, err)
        }
        for _, inst := range c.Instances {
            if err := inst.Validate(); err != nil {
                errs = append(errs, err)
            }
        }
    } else {
        // 单实例模式验证 (v1.0 向后兼容)
        if err := c.Nanobot.Validate(); err != nil {
            errs = append(errs, err)
        }
    }

    // 聚合所有错误
    return errors.Join(errs...)
}
```

### Anti-Patterns to Avoid

- **使用 yaml 标签而非 mapstructure 标签**: viper 使用 mapstructure 进行 unmarshal,yaml 标签会被忽略导致字段丢失
  - **正确做法**: 统一使用 `mapstructure` 标签
  - **来源**: [Stack Overflow - Viper not considering yaml tags](https://stackoverflow.com/questions/56773979)

- **验证失败时返回第一个错误就停止**: 用户需要修复一个错误后才能看到下一个,效率低
  - **正确做法**: 使用 errors.Join 聚合所有错误,一次性报告
  - **来源**: [Medium - Error Aggregation Pattern](https://medium.com/@virtualik/error-handling-patterns-in-go-every-developer-should-know-8962777c935b)

- **模糊的错误消息**: 只说"配置验证失败"而不提供具体位置和原因
  - **正确做法**: 错误消息包含实例名称、位置索引、具体字段和期望值
  - **来源**: [JetBrains - Go Error Handling Best Practices](https://blog.jetbrains.com/go/2026/03/02/secure-go-error-handling-best-practices/)

- **使用第三方验证库过度设计**: 引入 go-playground/validator 只为 unique 检查
  - **正确做法**: 简单场景使用 map-based 去重,保持代码可读性和可控性

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML unmarshal 到 struct | 自定义 YAML parser | viper + mapstructure | viper 处理了 defaults、环境变量、类型转换等复杂场景 |
| 唯一性检查 | 手写循环嵌套 O(n²) | map[T]struct{} 或 map[T]int | map 查找 O(1),总体 O(n),性能更好 |
| 错误聚合 | 自定义 Error type | errors.Join (Go 1.20+) | 标准库实现,与 errors.Is/As 兼容性好 |
| Duration 字段解析 | 自定义 duration parser | time.ParseDuration + viper | viper 内置支持 duration string (如 "30s") |

**Key insight:** Go 1.20+ 的 errors.Join 已足够应对配置验证的错误聚合需求,无需引入 hashicorp/go-multierror 增加依赖复杂度。map-based 去重算法简单高效,错误消息可控性强。

## Common Pitfalls

### Pitfall 1: 使用 yaml 标签导致字段丢失
**What goes wrong:** 在 struct 字段上使用 `yaml:"field_name"` 标签,期望 viper 能识别
**Why it happens:** viper 使用 mapstructure 包进行 unmarshal,不是 yaml.v3,因此 yaml 标签被忽略
**How to avoid:** 统一使用 `mapstructure:"field_name"` 标签
**Warning signs:** 配置文件中有字段,但 unmarshal 后 struct 字段为零值
**Source:** [Stack Overflow - Viper yaml tags issue](https://stackoverflow.com/questions/56773979/viper-is-not-considering-the-yaml-tags-in-my-structs-on-unmarshalling)

### Pitfall 2: 向后兼容性破坏 v1.0 配置
**What goes wrong:** 新版本程序无法加载旧的 v1.0 配置文件
**Why it happens:** 验证逻辑强制要求 instances 字段存在,或者 Nanobot section 缺失时报错
**How to avoid:** 实现配置模式兼容性检查,两种模式互斥但都有效
**Warning signs:** 测试中使用旧 config.yaml 文件,程序启动失败
**Source:** CONTEXT.md 锁定决策 - "新配置优先,旧配置作为后备"

### Pitfall 3: 错误消息不够详细
**What goes wrong:** 错误消息只说"端口重复",但不告诉用户哪两个实例冲突
**Why it happens:** 验证逻辑使用简单的 duplicate 检查,没有记录冲突位置
**How to avoid:** 使用 map[T]string 或 map[T]int 记录值到位置的映射,构建详细错误消息
**Warning signs:** 用户需要手动逐个检查配置才能找到冲突
**Source:** [JetBrains - User-Friendly Error Messages](https://blog.jetbrains.com/go/2026/03/02/secure-go-error-handling-best-practices/)

### Pitfall 4: 验证逻辑执行顺序不当
**What goes wrong:** 先检查字段格式,再检查互斥性,导致同时有两种配置时报多个错误
**Why it happens:** 没有按照"结构 -> 语义 -> 业务"的顺序组织验证
**How to avoid:** 先检查配置模式兼容性,再进行字段格式和唯一性验证
**Warning signs:** 配置文件同时有 nanobot 和 instances,报出一堆字段验证错误

### Pitfall 5: 未处理 startup_timeout 默认值
**What goes wrong:** 实例的 startup_timeout 为 0,导致后续 Phase 7 生命周期管理使用 0 超时
**Why it happens:** InstanceConfig.Validate() 只检查 < 5s,不处理零值情况
**How to avoid:** 在验证或使用时,如果 startup_timeout 为 0,使用全局默认值 (30s)
**Warning signs:** 配置文件未设置 startup_timeout,程序行为异常
**Source:** CONTEXT.md - startup_timeout "不配置时使用全局的 nanobot.startup_timeout"

## Code Examples

Verified patterns from official sources and project code:

### InstanceConfig Structure Definition
```go
// Source: 项目现有模式 (internal/config/config.go) + viper 官方文档
// https://github.com/spf13/viper

type InstanceConfig struct {
    Name           string        `mapstructure:"name"`             // 必填
    Port           uint32        `mapstructure:"port"`             // 必填, 1-65535
    StartCommand   string        `mapstructure:"start_command"`    // 必填
    StartupTimeout time.Duration `mapstructure:"startup_timeout"`  // 可选, 最小 5s
}

// Config 扩展 - 添加 Instances 字段
type Config struct {
    Cron      string           `mapstructure:"cron"`
    Nanobot   NanobotConfig    `mapstructure:"nanobot"`    // 保留 v1.0 兼容
    Instances []InstanceConfig `mapstructure:"instances"`  // 新增 v0.2
    Pushover  PushoverConfig   `mapstructure:"pushover"`
}
```

### YAML Configuration Example
```yaml
# 新配置格式 (v0.2 多实例模式)
cron: "0 3 * * *"

instances:
  - name: "instance1"
    port: 18790
    start_command: "C:\\path\\to\\nanobot.exe"
    startup_timeout: 30s

  - name: "instance2"
    port: 18791
    start_command: "C:\\another\\path\\nanobot.exe --port 18791"
    # startup_timeout 可选,使用全局默认

# 旧配置格式 (v1.0 单实例模式) - 仍然支持
# nanobot:
#   port: 18790
#   startup_timeout: 30s
#   repo_path: "C:\\Users\\allan716\\.nanobot\\nanobot-repo"

pushover:
  api_token: "aqquyv31y73mzh9k3qfptpd1zyi73z"
  user_key: "uw3b9cbopa5jn843xqxwknzcbjzoe5"
```

### Complete Validation Implementation
```go
// Source: Go 标准库 + 项目现有模式

func (ic *InstanceConfig) Validate() error {
    if ic.Name == "" {
        return fmt.Errorf("实例 name 字段不能为空")
    }
    if ic.Port == 0 || ic.Port > 65535 {
        return fmt.Errorf("实例 %q 的端口必须 > 0 且 <= 65535,当前值: %d", ic.Name, ic.Port)
    }
    if ic.StartCommand == "" {
        return fmt.Errorf("实例 %q 缺少必填字段 \"start_command\"", ic.Name)
    }
    // startup_timeout 允许为 0,使用时回退到全局默认值
    if ic.StartupTimeout != 0 && ic.StartupTimeout < 5*time.Second {
        return fmt.Errorf("实例 %q 的 startup_timeout 必须至少 5 秒,当前值: %v",
            ic.Name, ic.StartupTimeout)
    }
    return nil
}

func (c *Config) ValidateModeCompatibility() error {
    // 检测 Nanobot section 是否有非零值 (用户显式配置)
    hasLegacy := c.Nanobot.Port != 0

    if hasLegacy && len(c.Instances) > 0 {
        return fmt.Errorf("配置错误: 不能同时使用 'nanobot' section 和 'instances' 数组,请选择其中一种配置模式")
    }
    return nil
}

func validateUniqueNames(instances []InstanceConfig) error {
    seen := make(map[string]int)

    for i, inst := range instances {
        if firstIdx, exists := seen[inst.Name]; exists {
            return fmt.Errorf("配置验证失败: 实例名称重复 - %q 出现在第 %d 和第 %d 个实例配置中",
                inst.Name, firstIdx+1, i+1)
        }
        seen[inst.Name] = i
    }
    return nil
}

func validateUniquePorts(instances []InstanceConfig) error {
    portToInstance := make(map[uint32]string)

    for _, inst := range instances {
        if existingName, exists := portToInstance[inst.Port]; exists {
            return fmt.Errorf("配置验证失败: 端口重复 - %d 出现在实例 %q 和 %q 中",
                inst.Port, existingName, inst.Name)
        }
        portToInstance[inst.Port] = inst.Name
    }
    return nil
}

func (c *Config) Validate() error {
    var errs []error

    // 1. 验证 cron 表达式
    if err := ValidateCron(c.Cron); err != nil {
        errs = append(errs, err)
    }

    // 2. 检查配置模式兼容性
    if err := c.ValidateModeCompatibility(); err != nil {
        errs = append(errs, err)
    }

    // 3. 根据模式选择验证路径
    if len(c.Instances) > 0 {
        // 多实例模式验证
        if err := validateUniqueNames(c.Instances); err != nil {
            errs = append(errs, err)
        }
        if err := validateUniquePorts(c.Instances); err != nil {
            errs = append(errs, err)
        }
        for i := range c.Instances {
            if err := c.Instances[i].Validate(); err != nil {
                errs = append(errs, err)
            }
        }
    } else {
        // 单实例模式验证 (v1.0 向后兼容)
        if err := c.Nanobot.Validate(); err != nil {
            errs = append(errs, err)
        }
    }

    return errors.Join(errs...)
}
```

### Load Function Extension
```go
// Source: 项目现有模式 (internal/config/config.go:85-123)

func Load(configPath string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(configPath)
    v.SetConfigType("yaml")

    cfg := New()

    // 设置 defaults (保持现有逻辑)
    v.SetDefault("cron", cfg.Cron)
    v.SetDefault("nanobot.port", cfg.Nanobot.Port)
    v.SetDefault("nanobot.startup_timeout", cfg.Nanobot.StartupTimeout.String())
    v.SetDefault("nanobot.repo_path", cfg.Nanobot.RepoPath)
    v.SetDefault("pushover.api_token", cfg.Pushover.ApiToken)
    v.SetDefault("pushover.user_key", cfg.Pushover.UserKey)

    // 读取配置文件
    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("failed to read config file: %w", err)
        }
    }

    // Unmarshal 到 struct (自动处理 instances 数组)
    if err := v.Unmarshal(cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    // 验证 (包含新的多实例验证逻辑)
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }

    return cfg, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 手动循环嵌套检查唯一性 (O(n²)) | map-based 去重 (O(n)) | Go 1.0+ | 性能提升,代码简洁 |
| 返回第一个错误即停止 | errors.Join 聚合所有错误 | Go 1.20 (2023) | 用户体验提升,一次看到所有问题 |
| 使用 yaml 标签 | 使用 mapstructure 标签 | viper 1.0+ | 兼容 viper unmarshal 机制 |
| 单实例配置 | 多实例配置数组 | Phase 6 (本项目) | 支持多实例管理需求 |

**Deprecated/outdated:**
- **使用 hashicorp/go-multierror**: Go 1.20+ 的 errors.Join 已足够,无需额外依赖
- **自定义 error type 用于配置验证**: 过度设计,fmt.Errorf + errors.Join 足够
- **interface{} 类型配置字段**: 类型不安全,应使用具体类型 (string, uint32, time.Duration)

## Open Questions

1. **startup_timeout 默认值处理**
   - What we know: startup_timeout 字段可选,CONTEXT.md 说"不配置时使用全局的 nanobot.startup_timeout"
   - What's unclear: 在哪里处理默认值回退?是在 Validate() 中设置,还是在使用时 (Phase 7) 检查?
   - Recommendation: 在使用时检查,避免在 config 包中硬编码业务逻辑。Phase 7 的 lifecycle.Manager 初始化时,如果 startup_timeout 为 0,使用 cfg.Nanobot.StartupTimeout (全局默认)

2. **实例配置位置索引从 0 还是从 1 开始**
   - What we know: Go 数组索引从 0 开始,但错误消息面向用户
   - What's unclear: 错误消息中的"第 X 个实例"应该从 0 还是从 1 计数?
   - Recommendation: 面向用户的错误消息从 1 开始计数 (第 1 个实例),更符合用户直觉

## Validation Architecture

> workflow.nyquist_validation 未在 .planning/config.json 中显式设置,根据规则视为启用

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — 使用 `*_test.go` 文件 |
| Quick run command | `go test ./internal/config -v -run TestInstance` |
| Full suite command | `go test ./internal/config -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONF-01 | instances 数组 YAML 加载 | unit | `go test ./internal/config -v -run TestLoadInstancesYAML` | ❌ Wave 0 |
| CONF-01 | InstanceConfig 结构 unmarshal | unit | `go test ./internal/config -v -run TestInstanceConfigUnmarshal` | ❌ Wave 0 |
| CONF-02 | 名称唯一性验证 - 检测重复 | unit | `go test ./internal/config -v -run TestValidateUniqueNames_Duplicate` | ❌ Wave 0 |
| CONF-02 | 名称唯一性验证 - 通过 | unit | `go test ./internal/config -v -run TestValidateUniqueNames_Valid` | ❌ Wave 0 |
| CONF-02 | 错误消息包含位置信息 | unit | `go test ./internal/config -v -run TestValidateUniqueNames_ErrorMessage` | ❌ Wave 0 |
| CONF-03 | 端口唯一性验证 - 检测重复 | unit | `go test ./internal/config -v -run TestValidateUniquePorts_Duplicate` | ❌ Wave 0 |
| CONF-03 | 端口唯一性验证 - 通过 | unit | `go test ./internal/config -v -run TestValidateUniquePorts_Valid` | ❌ Wave 0 |
| CONF-03 | 错误消息包含实例名称 | unit | `go test ./internal/config -v -run TestValidateUniquePorts_ErrorMessage` | ❌ Wave 0 |
| CONF-01/02/03 | 向后兼容 - 旧配置可加载 | unit | `go test ./internal/config -v -run TestLoadLegacyConfig` | ❌ Wave 0 |
| CONF-01/02/03 | 模式互斥检测 | unit | `go test ./internal/config -v -run TestValidateModeCompatibility` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/config -v -run TestInstance` (运行当前任务相关的测试)
- **Per wave merge:** `go test ./internal/config -v` (完整 config 包测试套件)
- **Phase gate:** `go test ./...` (全项目测试,Phase 6 之前需全部通过)

### Wave 0 Gaps
- [ ] `internal/config/instance_test.go` — InstanceConfig 验证测试
- [ ] `internal/config/instance.go` — InstanceConfig 结构和验证逻辑
- [ ] `internal/config/config_test.go` — 扩展: 添加 TestLoadInstancesYAML, TestLoadLegacyConfig, TestValidateModeCompatibility
- [ ] `testutil/testdata/` — 测试配置文件 (instances_valid.yaml, instances_duplicate_name.yaml, instances_duplicate_port.yaml, legacy_v1.yaml, mixed_mode.yaml)

*(If no gaps: "None — existing test infrastructure covers all phase requirements")*

**Wave 0 Required:**
1. 创建 `internal/config/instance.go` — 定义 InstanceConfig 结构
2. 扩展 `internal/config/config.go` — 添加 Instances 字段和验证逻辑
3. 创建 `internal/config/instance_test.go` — InstanceConfig 单元测试
4. 扩展 `internal/config/config_test.go` — 添加集成测试用例
5. 准备测试数据文件 — 各种配置场景的 YAML 文件

## Sources

### Primary (HIGH confidence)
- [go-yaml/yaml GitHub](https://github.com/go-yaml/yaml) - YAML 1.2 支持,向后兼容 YAML 1.1
- [spf13/viper GitHub](https://github.com/spf13/viper) - Viper 配置管理库官方文档
- [go-viper/mapstructure/v2](https://pkg.go.dev/github.com/go-viper/mapstructure/v2) - mapstructure 官方 API 文档
- 项目现有代码: `internal/config/config.go` - 验证模式、配置加载流程
- CONTEXT.md - Phase 6 锁定决策

### Secondary (MEDIUM confidence)
- [Stack Overflow - Viper yaml tags](https://stackoverflow.com/questions/56773979/viper-is-not-considering-the-yaml-tags-in-my-structs-on-unmarshalling) - Viper 使用 mapstructure 标签,非 yaml 标签
- [Stack Overflow - Finding Unique Items](https://stackoverflow.com/questions/34111476/finding-unique-items-in-a-go-slice-or-array) - map-based 去重模式
- [Medium - Error Aggregation Pattern](https://medium.com/@virtualik/error-handling-patterns-in-go-every-developer-should-know-8962777c935b) - errors.Join 使用模式
- [Go Blog - unique package](https://go.dev/blog/unique) - Go 1.23 unique 包 (本阶段不使用,但了解技术趋势)

### Tertiary (LOW confidence)
- [JetBrains - Go Error Handling](https://blog.jetbrains.com/go/2026/03/02/secure-go-error-handling-best-practices/) - 错误消息最佳实践 (需验证适用性)
- [Medium - Validator Guide](https://leapcell.medium.com/validator-complex-structs-arrays-and-maps-validation-for-go-6c5f8d440ed3) - go-playground/validator (本阶段不使用)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - 项目已使用 viper + mapstructure,无需新依赖;Go 标准库 errors.Join 稳定可用
- Architecture: HIGH - 验证模式遵循项目现有代码风格;唯一性检查使用 Go 社区成熟模式;向后兼容方案明确
- Pitfalls: HIGH - 基于 viper 官方文档、Stack Overflow 高票回答和项目实际情况总结

**Research date:** 2026-03-10
**Valid until:** 2027-03-10 (Go 标准库和 viper API 稳定,验证模式成熟,有效期 1 年)
