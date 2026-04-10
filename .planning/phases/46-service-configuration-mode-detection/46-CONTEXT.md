# Phase 46: Service Configuration & Mode Detection - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

用户通过 config.yaml 的 service 子段控制服务模式开关，程序启动时通过 svc.IsWindowsService() 自动检测运行环境并选择服务模式或控制台模式。这是 v0.11 的第一个阶段，为后续 Phase 47（svc.Handler）、Phase 48（服务注册）、Phase 49（代码适配）奠定基础。

</domain>

<decisions>
## Implementation Decisions

### 配置结构设计
- **D-01:** 新增 `ServiceConfig` 子段到 `Config` 结构体，字段：`auto_start *bool`、`service_name string`、`display_name string`
- **D-02:** `auto_start` 默认 `false`，未配置时行为与当前完全一致（控制台模式）
- **D-03:** 预留 `service_name` 和 `display_name` 字段（Phase 48 服务注册需要），默认值分别为 `"NanobotAutoUpdater"` 和 `"Nanobot Auto Updater"`
- **D-04:** 使用 `mapstructure:"service"` 标签，遵循项目现有 viper+mapstructure 模式（与 `self_update`、`api` 等子段一致）
- **D-05:** 创建独立文件 `internal/config/service.go`，与 `api.go`、`selfupdate.go` 等同结构

### 模式检测策略
- **D-06:** 启动时序：先调用 `svc.IsWindowsService()` 检测环境，再加载配置。服务模式路径不需要先读 config.yaml
- **D-07:** SCM 启动 + `auto_start: false` 时：以服务模式正常运行，但记录 WARN 日志提醒配置已变更（Phase 48 会处理自动卸载）
- **D-08:** 控制台运行 + `auto_start: true` 时：检测管理员权限，自动注册 Windows 服务后以退出码 `2` 退出
- **D-09:** 退出码 `2` 表示"注册服务后退出"，与正常退出 `0` 区分

### 配置解析验证
- **D-10:** `service_name` 格式校验：仅允许字母数字，无空格（SCM 要求）
- **D-11:** `display_name` 长度限制：最大 256 字符（SCM 限制）
- **D-12:** 仅当 `auto_start: true` 时校验 `service_name` 和 `display_name`；`auto_start: false` 时跳过
- **D-13:** 测试范围：只测 ServiceConfig 的配置解析、默认值、验证逻辑。svc.IsWindowsService() 的集成测试留给 Phase 47

### Claude's Discretion
- ServiceConfig 具体的 Go struct 定义细节
- 验证错误消息的具体措辞
- WARN 日志的格式和内容

</decisions>

<specifics>
## Specific Ideas

- 退出码 `2` 是用户明确选择的，用于脚本中区分"注册服务后退出"和"正常运行后退出"
- config.go 已有子段模式（SelfUpdateConfig、APIConfig 等），ServiceConfig 遵循同样模式即可

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 配置系统
- `internal/config/config.go` — Config 根结构体、Load() 函数、viper 初始化流程
- `internal/config/selfupdate.go` — 子段配置的参考实现（SelfUpdateConfig 结构体 + 默认值 + 验证）
- `internal/config/api.go` — 另一个子段配置参考（APIConfig + 验证逻辑）

### 入口点
- `cmd/nanobot-auto-updater/main.go` — 程序入口，需要在此添加 svc.IsWindowsService() 检测和分支逻辑

### 进程管理
- `internal/lifecycle/daemon.go` — 现有守护进程代码（build tag: windows），MakeDaemon() 目前未被 main.go 调用

### 测试参考
- `internal/config/instance_test.go` — 表格驱动测试模式参考
- `internal/config/api_test.go` — 配置验证测试模式参考

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` Load() 函数：viper.New() + SetDefault + ReadInConfig + Unmarshal 模式，ServiceConfig 直接复用
- `internal/config/selfupdate.go`：最小的子段配置实现（2 个字段 + 默认值），可作为 ServiceConfig 模板
- `golang.org/x/sys v0.41.0`：已在 go.mod 中，`windows/svc` 子包可直接导入，无需新增依赖

### Established Patterns
- 子段配置模式：每个子配置类型独立文件（api.go、selfupdate.go 等），结构体 + New() 默认值 + Validate()
- viper 标签使用 `mapstructure`（非 `yaml`），这是项目已有决策（Key Decisions 表）
- 验证使用 `errors.Join` 聚合所有错误
- 测试使用表格驱动 + `t.Run()` 子测试

### Integration Points
- `Config` 结构体：需新增 `Service ServiceConfig` 字段
- `config.Load()`：需添加 `ServiceConfig` 的 `SetDefault()` 调用
- `Config.Validate()`：需添加 `cfg.Service.Validate()` 调用
- `cmd/nanobot-auto-updater/main.go`：初始化流程最前面插入 svc.IsWindowsService() 检测

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---
*Phase: 46-service-configuration-mode-detection*
*Context gathered: 2026-04-10*
