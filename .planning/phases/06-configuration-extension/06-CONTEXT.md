# Phase 6: 配置扩展 - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

用户可以在 YAML 配置文件中定义多个 nanobot 实例,程序启动时自动验证配置的有效性(名称和端口唯一性),同时保持与 v1.0 配置文件的向后兼容性。此阶段专注于配置结构和验证,不涉及实例的生命周期管理(Phase 7)。

</domain>

<decisions>
## Implementation Decisions

### 配置结构设计
- 新增 `instances` 数组字段到 Config 结构,每个元素包含 name、port、start_command、startup_timeout 字段
- **配置模式选择**: 新配置优先,旧配置作为后备
  - 如果 `instances` 数组存在,使用多实例模式
  - 如果 `instances` 不存在,使用现有的 `nanobot` section (单实例模式)
  - 两种模式互斥,配置验证时检查并报错
- 保持现有 `nanobot` section 不变,用于向后兼容 v1.0 配置文件

### 实例字段定义
每个实例包含以下字段:

**必填字段:**
- `name`: 实例名称 (string),用于标识实例和日志输出
- `port`: 实例端口号 (uint32),必须在 1-65535 范围内
- `start_command`: 实例启动命令 (string),用户指定完整的启动命令

**可选字段:**
- `startup_timeout`: 实例启动超时时间 (duration),不配置时使用全局的 nanobot.startup_timeout

### 验证行为
- **验证时机**: 程序启动时,配置加载后立即验证
- **验证内容**:
  1. 检查 `instances` 和 `nanobot` section 是否同时存在(不允许)
  2. 检查所有实例的 `name` 字段唯一性
  3. 检查所有实例的 `port` 字段唯一性
  4. 验证每个实例的必填字段是否存在
  5. 验证 `port` 在有效范围内 (1-65535)
  6. 验证 `startup_timeout` 格式和最小值(5秒)

### 错误消息格式
- **详细错误消息**: 列出所有重复项及其位置
- **示例格式**:
  - 名称重复: "配置验证失败: 实例名称重复 - \"instance1\" 出现在第 2 和第 5 个实例配置中"
  - 端口重复: "配置验证失败: 端口重复 - 18790 出现在实例 \"instance1\" 和 \"instance2\" 中"
  - 字段缺失: "配置验证失败: 实例 \"instance1\" 缺少必填字段 \"start_command\""

### 验证失败处理
- **立即退出**: 检测到配置验证错误时,打印错误消息并退出(exit code 1)
- 不加载配置,不启动程序
- 用户必须修复配置文件才能启动程序

### Claude's Discretion
- InstanceConfig 结构体的具体命名(例如 InstanceConfig vs NanobotInstance)
- 错误消息的具体措辞(只要保持详细和清晰)
- 验证逻辑的实现方式(循环遍历 vs 使用 map 去重)

</decisions>

<specifics>
## Specific Ideas

- 配置验证应该快速失败(fail-fast),避免启动后才发现配置错误
- 详细错误消息帮助用户快速定位和修复配置问题
- 启动命令必填确保每个实例的启动方式明确,避免意外行为
- startup_timeout 可选保持配置简洁,大多数实例可以使用全局超时设置

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- **internal/config/config.go**: 现有配置加载和验证逻辑,使用 viper 库
  - 可以扩展 `Config` 结构添加 `Instances []InstanceConfig` 字段
  - 复用现有的 `Validate()` 模式进行配置验证
  - 复用 `viper.New()` 创建独立 viper 实例的模式

- **internal/config/config.go**: 现有验证函数
  - `ValidateCron()` 函数模式可复用于实例配置验证
  - `NanobotConfig.Validate()` 方法模式可复用于 InstanceConfig 验证

### Established Patterns
- **配置加载模式**: defaults() → ReadInConfig() → Unmarshal() → Validate()
- **验证模式**: 每个配置结构有独立的 Validate() 方法
- **错误处理**: 使用 fmt.Errorf 包装错误,提供上下文信息

### Integration Points
- **配置加载入口**: `internal/config.Load()` 函数需要扩展支持 instances 验证
- **生命周期管理**: Phase 7 将使用 InstanceConfig 创建多个 lifecycle.Manager
- **主程序**: main.go 需要根据配置模式(单实例 vs 多实例)选择不同的执行路径

</code_context>

<deferred>
## Deferred Ideas

None — 讨论保持在阶段范围内

</deferred>

---

*Phase: 06-configuration-extension*
*Context gathered: 2026-03-10*
