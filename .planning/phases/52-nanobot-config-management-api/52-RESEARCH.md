# Phase 52: Nanobot Config Management API - Research

**Researched:** 2026-04-12
**Domain:** Go HTTP API - nanobot config.json 读写管理
**Confidence:** HIGH

## Summary

Phase 52 为每个 nanobot 实例提供 config.json 的读写 API，并将自动创建 nanobot 配置目录的功能集成到 Phase 50 的 create/copy 流程中。核心实现需要：(1) 从 InstanceConfig.StartCommand 解析 `--config` 参数以定位 nanobot 配置文件路径；(2) 新建 NanobotConfigManager 管理文件读写和默认配置生成；(3) 新增 GET/PUT nanobot-config 两个 API 端点；(4) 修改 InstanceConfigHandler 注入 nanobot 目录创建逻辑。

该 phase 的技术实现高度模式化——复用 Phase 50/51 已建立的 handler struct + 依赖注入模式、auth middleware、JSON 错误响应格式。唯一的非平凡技术点是 `--config` 路径解析（需要从 start_command 字符串中正确提取路径，处理引号、反斜杠、`~` 展开等边界情况）和默认配置模板的参数化生成。

**Primary recommendation:** 创建独立的 `internal/api/nanobot_config_handler.go` 文件实现 GET/PUT 端点，创建 `internal/nanobot/config_manager.go` 封装路径解析、文件读写和默认配置生成。通过在 InstanceConfigHandler 中注入回调函数（或接口）来集成 create/copy 流程，避免修改 NewServer() 签名。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 从 start_command 的 `--config` 参数解析 nanobot 配置路径（正则或字符串匹配）
- **D-02:** 无 `--config` 时 fallback 到 `~/.nanobot-{name}/config.json`
- **D-03:** `~` 解析为 `os.UserHomeDir()`
- **D-04:** 默认 config.json 含完整结构（agents/channels/providers/gateway/tools），敏感值留空
- **D-05:** 参数化 `gateway.port` 和 `agents.defaults.workspace`
- **D-06:** 默认值：model="glm-5-turbo", provider="zhipu", maxTokens=131072, temperature=0.7, gateway.host="0.0.0.0"
- **D-07:** telegram 频道默认 disabled=true, token="", allowFrom=[]
- **D-08:** providers 保留 zhipu/groq/aihubmix 结构，apiKey 全部留空
- **D-09:** 将 NanobotConfigManager 注入到 InstanceConfigHandler
- **D-10:** Create 流程：创建实例配置后 -> 创建 nanobot 目录 -> 写入默认 config.json
- **D-11:** Copy 流程：复制实例配置后 -> 创建新 nanobot 目录 -> 复制源 config.json -> 更新 port 和 workspace
- **D-12:** 修改 InstanceConfigHandler 和 NewServer() 构造函数
- **D-13:** PUT 只写文件，不自动重启实例
- **D-14:** 响应可含运行状态提示（informational），不触发操作
- **D-15:** 用户通过 Phase 51 lifecycle API 手动重启

### Claude's Discretion
- nanobot config 写入时的 JSON 格式验证
- 文件写入并发安全机制（mutex 保护）
- NanobotConfigManager 的具体 struct 设计和接口定义
- 目录创建时的错误处理
- --config 参数的具体解析实现
- 响应 JSON 的包装格式

### Deferred Ideas (OUT OF SCOPE)
- nanobot config schema 验证（ENC-01）— 未来里程碑
- nanobot config 模板库（ENC-02）— 未来里程碑
- 智能增量热重载 — 未来优化
- PUT 后自动重启实例 — 未来可选参数
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| NC-01 | Creating a new instance auto-creates nanobot config directory and default config.json | ParseConfigPath() + CreateDefaultConfig() 集成到 HandleCreate |
| NC-02 | User can read nanobot's config.json via GET API | HandleGetNanobotConfig 读取文件并返回 |
| NC-03 | User can update nanobot's config.json via PUT API | HandlePutNanobotConfig 写入文件，JSON 验证 + mutex |
| NC-04 | Copy instance clones nanobot config.json with port/name updated | CloneConfig() 集成到 HandleCopy，更新 port 和 workspace |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library `encoding/json` | Go 1.24 | JSON 编解码 | 项目已全局使用 `json.NewDecoder`/`json.NewEncoder` |
| Go standard library `os` | Go 1.24 | 文件读写和目录创建 | `os.ReadFile`/`os.WriteFile`/`os.MkdirAll` |
| Go standard library `regexp` | Go 1.24 | 解析 `--config` 参数 | 项目未引入第三方正则库，标准库足够 |
| Go standard library `sync` | Go 1.24 | Mutex 并发保护 | 项目已在 `config.go` 使用 `sync.Mutex` |
| `github.com/stretchr/testify` | v1.11.1 | 测试断言 | 项目已有 41 个测试文件全部使用此库 [VERIFIED: go.mod] |
| `log/slog` | Go 1.24 | 结构化日志 | 项目全局使用 slog |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os.UserHomeDir()` | Go 1.24 | `~` 路径展开 | D-03 要求 |
| `filepath.Join`/`filepath.Dir` | Go 1.24 | 路径拼接 | 构造 nanobot 目录路径 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `regexp` 解析 --config | `strings.Split` + 手动遍历 | 手动遍历更简单但不够健壮；正则可处理引号情况 |
| 全局 mutex | per-instance mutex (map[string]*sync.Mutex) | per-instance 粒度更细但复杂度更高，Phase 52 写入频率低，全局 mutex 足够 |

**Installation:** 无需安装新依赖，全部使用 Go 标准库和已有依赖。

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   ├── nanobot_config_handler.go    # 新文件：GET/PUT nanobot-config handler
│   ├── nanobot_config_handler_test.go  # 新文件：测试
│   ├── instance_config_handler.go   # 修改：注入 NanobotConfigManager 回调
│   ├── server.go                    # 修改：注册新路由 + 传递 handler
│   └── auth.go                      # 不变：复用 auth middleware
├── nanobot/                         # 新包：nanobot config 管理
│   ├── config_manager.go            # 新文件：路径解析、文件读写、默认配置
│   └── config_manager_test.go       # 新文件：测试
```

### Pattern 1: Handler Struct + 构造函数注入（项目已建立模式）
**What:** Handler struct 持有依赖引用，通过 `NewXxxHandler(deps, logger)` 构造
**When to use:** 所有新 handler
**Example:**
```go
// Source: internal/api/instance_lifecycle_handler.go (Phase 51 模式)
type InstanceLifecycleHandler struct {
    im     *instance.InstanceManager
    logger *slog.Logger
}

func NewInstanceLifecycleHandler(im *instance.InstanceManager, logger *slog.Logger) *InstanceLifecycleHandler {
    return &InstanceLifecycleHandler{
        im:     im,
        logger: logger.With("source", "api-instance-lifecycle"),
    }
}
```

### Pattern 2: 路由注册（Go 1.22+ ServeMux 模式）
**What:** 使用 `mux.Handle("METHOD /path/{param}", middleware(handler))` 注册路由
**When to use:** 所有新端点
**Example:**
```go
// Source: internal/api/server.go 第 111-116 行 (Phase 50 路由注册)
instanceConfigHandler := NewInstanceConfigHandler(config.GetCurrentConfig, logger)
mux.Handle("GET /api/v1/instance-configs", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleList)))
mux.Handle("POST /api/v1/instance-configs", authMiddleware(http.HandlerFunc(instanceConfigHandler.HandleCreate)))
```

### Pattern 3: 错误分发（自定义错误类型 + errors.As）
**What:** 定义自定义错误类型（validationError/notFoundError），handler 中用 `errors.As` 分发
**When to use:** 需要区分错误类型的 handler
**Example:**
```go
// Source: internal/api/instance_config_handler.go 第 237-244 行
var valErr *validationError
if errors.As(err, &valErr) {
    h.writeValidationError(w, "Validation failed", valErr.details)
    return
}
```

### Pattern 4: Callback 注入避免修改 NewServer 签名
**What:** 通过在 InstanceConfigHandler 上设置可选回调函数来扩展 create/copy 行为，不改变 NewServer() 签名
**When to use:** 需要在现有 handler 中注入新行为时
**Example:**
```go
// 新增到 InstanceConfigHandler
type InstanceConfigHandler struct {
    getConfig  func() *config.Config
    logger     *slog.Logger
    // Phase 52 注入：创建实例后的 nanobot 配置创建回调
    onCreateInstance func(name string, port uint32, startCommand string) error
    onCopyInstance   func(sourceName string, targetName string, targetPort uint32, targetStartCommand string) error
}
```

### Anti-Patterns to Avoid
- **直接在 InstanceConfigHandler 中操作文件系统:** 应将 nanobot config 逻辑封装到独立包（`internal/nanobot/`），保持 handler 职责单一
- **修改 config.go 的 UpdateConfig() 来写 nanobot config:** UpdateConfig 只写 auto-updater 的 config.yaml，nanobot config.json 应独立用 `os.WriteFile` 写入 [VERIFIED: config.go 源码]
- **在 handler 中硬编码路径模板:** 路径解析逻辑应集中在 NanobotConfigManager 中，handler 只做 HTTP 层处理
- **忽略 Windows 路径分隔符:** config.yaml 中的路径使用正斜杠（如 `C:/Users/allan716/.nanobot-work-helper/config.json`），`filepath.Join` 会正确处理，但 `--config` 解析需要兼顾两种分隔符 [VERIFIED: config.yaml 实际路径]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON 合法性验证 | 手写 JSON parser/validator | `json.Valid()` 或 `json.Unmarshal` 到 `map[string]interface{}` | 标准库已经完美处理边界情况 |
| 路径分隔符处理 | 手动拼接路径字符串 | `filepath.Join()` / `filepath.Dir()` | Windows 路径兼容性 |
| `~` 展开 | 手动字符串替换 | `os.UserHomeDir()` + `strings.Replace` | `os.UserHomeDir()` 处理平台差异 |
| 并发文件写入保护 | per-file lock 文件 | `sync.Mutex` | 项目已在 config.go 中使用此模式 [VERIFIED: config.go 第 179 行] |
| 目录创建 | 手动检查+创建 | `os.MkdirAll(path, 0755)` | 自动处理父目录不存在的情况 |

**Key insight:** 所有需要的功能都能用 Go 标准库实现，不需要引入任何新依赖。

## Common Pitfalls

### Pitfall 1: start_command 中 --config 路径解析失败
**What goes wrong:** start_command 格式多样，`--config` 后的路径可能含空格（引号包裹）、Windows 反斜杠、或没有 --config 参数
**Why it happens:** 字符串解析的边界情况复杂
**How to avoid:** 使用正则表达式 `--config\s+["']?([^"'\s]+)["']?` 匹配，并充分测试各种边界情况
**Warning signs:** 实际 config.yaml 中的 start_command 格式为 `nanobot gateway --config C:/Users/allan716/.nanobot-work-helper/config.json --port 18792`，无引号包裹 [VERIFIED: config.yaml]

### Pitfall 2: config.yaml 中 YAML key 是 `startcommand` 不是 `start_command`
**What goes wrong:** config.yaml 中 YAML key 为 `startcommand`（无下划线），但 mapstructure tag 为 `start_command`。InstanceConfig.Go struct 使用 `mapstructure:"start_command"` [VERIFIED: config.yaml 和 instance.go]
**Why it happens:** YAML key 和 mapstructure tag 名称不一致，viper 通过 mapstructure tag 映射
**How to avoid:** 代码中始终使用 `InstanceConfig.StartCommand`（Go struct field），不要直接读 YAML key
**Warning signs:** grep config.yaml 会看到 `startcommand:` 而非 `start_command:`

### Pitfall 3: 并发写入同一实例的 nanobot config.json
**What goes wrong:** 两个 PUT 请求同时写同一文件，导致数据损坏
**Why it happens:** HTTP handler 并发执行
**How to avoid:** 在 NanobotConfigManager 中使用 `sync.Mutex` 保护写入操作
**Warning signs:** 高并发 PUT 请求下文件内容异常

### Pitfall 4: HandleCreate/HandleCopy 中 nanobot 目录创建失败但 auto-updater 配置已写入
**What goes wrong:** config.yaml 更新成功但 nanobot 目录创建失败，导致不一致状态
**Why it happens:** Phase 50 的 HandleCreate 使用 `config.UpdateConfig()` 原子写入 config.yaml，但 nanobot 目录创建在 UpdateConfig 之后
**How to avoid:** 两种策略：(a) nanobot 目录创建失败时回滚 config.yaml（复杂），(b) nanobot 目录创建失败时只记录警告、标记实例为"配置不完整"（简单）。推荐 (b) 方案，因为 GET/PUT nanobot-config 端点可以后续修复
**Warning signs:** 新实例创建成功但 nanobot config 目录不存在

### Pitfall 5: 默认配置模板中 JSON 格式错误
**What goes wrong:** 硬编码的默认 JSON 模板有语法错误或缺少字段
**Why it happens:** 手写 JSON 模板容易出错
**How to avoid:** 使用 `map[string]interface{}` 或定义 Go struct 来生成 JSON，而非手写 JSON 字符串。在测试中验证生成的 JSON 能通过 `json.Unmarshal` + `json.Marshal` 往返
**Warning signs:** 生成的 config.json 无法被 nanobot 解析

### Pitfall 6: 实例不存在时访问 nanobot-config
**What goes wrong:** 请求一个不在 auto-updater config 中的实例的 nanobot config
**Why it happens:** nanobot-config API 依赖实例已存在于 config.yaml 中
**How to avoid:** GET/PUT nanobot-config 前先通过 `getCurrentConfig()` 检查实例是否存在，不存在返回 404
**Warning signs:** 直接用 name 拼路径读文件，不检查实例是否存在于配置中

## Code Examples

### 路径解析：从 start_command 提取 --config 路径
```go
// ParseConfigPath 从 start_command 中提取 --config 参数指定的路径
// D-01: 从 start_command 的 --config 参数解析
// D-02: 无 --config 时 fallback 到 ~/.nanobot-{name}/config.json
// D-03: ~ 解析为 os.UserHomeDir()
package nanobot

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
)

var configPathRegex = regexp.MustCompile(`--config\s+["']?([^"'\s]+)["']?`)

// ParseConfigPath 从 start_command 中提取 nanobot 配置文件路径
func ParseConfigPath(startCommand, instanceName string) (string, error) {
    matches := configPathRegex.FindStringSubmatch(startCommand)
    if len(matches) < 2 {
        // D-02: fallback 到 ~/.nanobot-{name}/config.json
        homeDir, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("failed to get home directory: %w", err)
        }
        return filepath.Join(homeDir, ".nanobot-"+instanceName, "config.json"), nil
    }

    configPath := matches[1]
    // D-03: 展开 ~ 为 home 目录
    if len(configPath) > 0 && configPath[0] == '~' {
        homeDir, err := os.UserHomeDir()
        if err != nil {
            return "", fmt.Errorf("failed to get home directory: %w", err)
        }
        configPath = filepath.Join(homeDir, configPath[1:])
    }

    return configPath, nil
}
```

### 默认配置生成
```go
// GenerateDefaultConfig 生成新实例的默认 nanobot config.json
// D-04: 完整结构，敏感值留空
// D-05: 参数化 port 和 workspace
// D-06: 保留默认值
func GenerateDefaultConfig(port uint32, workspace string) map[string]interface{} {
    return map[string]interface{}{
        "agents": map[string]interface{}{
            "defaults": map[string]interface{}{
                "workspace":         workspace, // D-05
                "model":             "glm-5-turbo", // D-06
                "provider":          "zhipu",
                "maxTokens":         131072,
                "temperature":       0.7,
                "maxToolIterations": 100,
                "memoryWindow":      50,
            },
        },
        "channels": map[string]interface{}{
            "telegram": map[string]interface{}{
                "enabled":   false, // D-07
                "token":     "",
                "allowFrom": []interface{}{},
                "proxy":     nil,
            },
        },
        "providers": map[string]interface{}{ // D-08
            "zhipu":    map[string]interface{}{"apiKey": "", "apiBase": "https://open.bigmodel.cn/api/coding/paas/v4/", "extraHeaders": nil},
            "groq":     map[string]interface{}{"apiKey": "", "apiBase": nil, "extraHeaders": nil},
            "aihubmix": map[string]interface{}{"apiKey": "", "apiBase": nil, "extraHeaders": nil},
        },
        "gateway": map[string]interface{}{
            "host": "0.0.0.0", // D-06
            "port": port,      // D-05
        },
        "tools": map[string]interface{}{
            "web": map[string]interface{}{
                "search": map[string]interface{}{
                    "apiKey":     "",
                    "maxResults": 5,
                },
            },
            "exec":              map[string]interface{}{"timeout": 60},
            "restrictToWorkspace": false,
            "mcpServers":        map[string]interface{}{},
        },
    }
}
```

### Handler 模式（参考 Phase 51）
```go
// Source: 项目已建立模式
type NanobotConfigHandler struct {
    manager      *nanobot.ConfigManager
    getConfig    func() *config.Config
    logger       *slog.Logger
}

func NewNanobotConfigHandler(manager *nanobot.ConfigManager, getConfig func() *config.Config, logger *slog.Logger) *NanobotConfigHandler {
    return &NanobotConfigHandler{
        manager:   manager,
        getConfig: getConfig,
        logger:    logger.With("source", "api-nanobot-config"),
    }
}

// HandleGet handles GET /api/v1/instance-configs/{name}/nanobot-config
func (h *NanobotConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    // 1. 检查实例是否存在
    // 2. 解析配置路径
    // 3. 读取文件
    // 4. 返回 JSON
}

// HandlePut handles PUT /api/v1/instance-configs/{name}/nanobot-config
func (h *NanobotConfigHandler) HandlePut(w http.ResponseWriter, r *http.Request) {
    name := r.PathValue("name")
    // 1. 检查实例是否存在
    // 2. 解析请求体 (JSON 验证)
    // 3. mutex 保护写入文件
    // 4. 返回成功响应
}
```

### 路由注册（参考 Phase 50 模式）
```go
// 在 server.go 的 NewServer() 中添加
nanobotConfigManager := nanobot.NewConfigManager(logger)
nanobotConfigHandler := NewNanobotConfigHandler(nanobotConfigManager, config.GetCurrentConfig, logger)
mux.Handle("GET /api/v1/instance-configs/{name}/nanobot-config", authMiddleware(http.HandlerFunc(nanobotConfigHandler.HandleGet)))
mux.Handle("PUT /api/v1/instance-configs/{name}/nanobot-config", authMiddleware(http.HandlerFunc(nanobotConfigHandler.HandlePut)))

// 注入到 InstanceConfigHandler（回调模式）
instanceConfigHandler.SetOnCreateInstance(func(name string, port uint32, startCommand string) error {
    return nanobotConfigManager.CreateDefaultConfig(name, port, startCommand)
})
instanceConfigHandler.SetOnCopyInstance(func(sourceName, targetName string, targetPort uint32, targetStartCommand string) error {
    return nanobotConfigManager.CloneConfig(sourceName, targetName, targetPort, targetStartCommand)
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Go <1.22 http.ServeMux 路由 | Go 1.22+ 方法+路径模式路由 | Go 1.22 (2024-02) | `mux.Handle("GET /path/{name}", handler)` 替代手动路由分发 |
| 手动 JSON 验证 | `json.Valid()` / `json.Unmarshal` to interface{} | Go 1.x | 标准库内置验证 |

**Deprecated/outdated:**
- 无已弃用模式。项目已使用 Go 1.22+ 路由模式和标准库 JSON 处理。

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | nanobot 读取 config.json 时使用 `~` 展开 | 路径解析 | 需要确认 nanobot 实际的路径解析行为 |
| A2 | Copy 流程中源实例的 nanobot config.json 一定存在 | Copy 集成 | 如果源实例从未启动过，config.json 可能不存在 |
| A3 | nanobot 不会在运行时锁定 config.json 文件 | PUT 写入 | Windows 上文件可能被锁定 |

**需要用户确认的项目：**
- A1: nanobot 的 `--config` 路径中 `~` 是否会被 nanobot 自身展开？还是传给 nanobot 的路径必须是绝对路径？
- A2: Copy 时源实例的 nanobot config.json 不存在时是否应创建默认配置再复制？

## Open Questions

1. **NewServer() 签名是否需要修改？**
   - What we know: D-12 要求修改 NewServer 构造函数。但通过回调注入模式可以避免。
   - What's unclear: CONTEXT.md 的 D-12 是否是硬性要求，还是"如果需要就改"。
   - Recommendation: 优先使用回调注入避免签名变更。如果回调模式不够清晰，再修改签名。NewServer 当前已有 9 个参数 [VERIFIED: server.go 第 29 行]，再加参数会增加复杂度。替代方案：将 `NanobotConfigManager` 作为 InstanceConfigHandler 的可选依赖注入。

2. **InstanceConfigHandler 的 HandleCreate/HandleCopy 是否需要回滚机制？**
   - What we know: Create 分两步——先写 config.yaml（原子操作），再创建 nanobot 目录。
   - What's unclear: 如果 nanobot 目录创建失败，是否需要回滚 config.yaml。
   - Recommendation: 不回滚。nanobot 目录创建失败只记录警告，GET/PUT nanobot-config 端点可后续修复。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none (Go conventions) |
| Quick run command | `go test ./internal/api/... ./internal/nanobot/... -count=1 -v -timeout 30s` |
| Full suite command | `go test ./... -count=1 -timeout 60s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| NC-01 | Create instance auto-creates nanobot config dir + default config.json | integration | `go test ./internal/api/... -run TestHandleCreate_NanobotConfig -v` | Wave 0 |
| NC-02 | GET nanobot-config returns config.json content | unit | `go test ./internal/api/... -run TestHandleGetNanobotConfig -v` | Wave 0 |
| NC-03 | PUT nanobot-config updates config.json | unit | `go test ./internal/api/... -run TestHandlePutNanobotConfig -v` | Wave 0 |
| NC-04 | Copy instance clones nanobot config with updated port/workspace | integration | `go test ./internal/api/... -run TestHandleCopy_NanobotConfig -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/api/... ./internal/nanobot/... -count=1 -timeout 30s`
- **Per wave merge:** `go test ./... -count=1 -timeout 60s`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/nanobot/config_manager.go` — 新包，含路径解析、文件读写、默认配置生成
- [ ] `internal/nanobot/config_manager_test.go` — 路径解析、默认配置生成、文件读写测试
- [ ] `internal/api/nanobot_config_handler.go` — GET/PUT handler
- [ ] `internal/api/nanobot_config_handler_test.go` — handler 测试
- [ ] `internal/api/instance_config_handler.go` — 修改：注入 onCreateInstance/onCopyInstance 回调
- [ ] `internal/api/instance_config_handler_test.go` — 修改：测试回调调用

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer token via AuthMiddleware（复用 Phase 50） |
| V3 Session Management | no | 无 session |
| V4 Access Control | no | 单 admin token，无 RBAC |
| V5 Input Validation | yes | JSON body 验证（json.Unmarshal），路径参数验证 |
| V6 Cryptography | no | 无加密需求 |

### Known Threat Patterns for Go HTTP API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via instance name | Tampering | 验证 name 不含 `..` 或路径分隔符（InstanceConfig.Validate 已验证 name 非空） |
| JSON injection via PUT body | Tampering | `json.Valid()` 或 `json.Unmarshal` 验证 |
| Concurrent write corruption | Tampering | `sync.Mutex` 保护文件写入 |
| Unauthorized config read/write | Information Disclosure | AuthMiddleware（constant-time comparison） |

## Sources

### Primary (HIGH confidence)
- 源码 `internal/api/server.go` — NewServer 构造函数、路由注册模式
- 源码 `internal/api/instance_config_handler.go` — CRUD handler 模式、HandleCreate/HandleCopy 注入点
- 源码 `internal/api/instance_lifecycle_handler.go` — Phase 51 handler 模式参考
- 源码 `internal/config/instance.go` — InstanceConfig struct 定义
- 源码 `internal/config/config.go` — UpdateConfig 原子操作模式
- 源码 `config.yaml` — 实际配置格式和 start_command 路径写法
- `go.mod` — 项目依赖版本

### Secondary (MEDIUM confidence)
- CONTEXT.md 中的 nanobot config.json 示例 — 默认配置模板基础

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 全部使用 Go 标准库和已有依赖，源码已验证
- Architecture: HIGH — 复用 Phase 50/51 已建立模式，注入点已定位到具体代码行
- Pitfalls: HIGH — 基于 config.yaml 实际格式和源码分析

**Research date:** 2026-04-12
**Valid until:** 2026-05-12（稳定项目，架构不会频繁变化）
