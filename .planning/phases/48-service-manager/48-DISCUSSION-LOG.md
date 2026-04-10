# Phase 48: Service Manager - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 48-service-manager
**Areas discussed:** 服务注册细节, 服务卸载策略, SCM 恢复策略, 权限与跨平台, 边界情况

---

## 服务注册细节

| Option | Description | Selected |
|--------|-------------|----------|
| LocalSystem | 权限最高，无需密码，适合需要访问文件系统和网络的场景 | ✓ |
| NetworkService | 权限最低，仅能访问网络资源。可能无法访问用户文件目录 | |
| 用户配置账户 | 用户在 config.yaml 中配置服务账户名和密码。更灵活但配置更复杂 | |

**User's choice:** LocalSystem (推荐)
**Notes:** 大多数 Windows 后台服务使用 LocalSystem，适合本项目需要访问文件系统和网络的场景

| Option | Description | Selected |
|--------|-------------|----------|
| 自动启动 | Windows 启动时自动启动服务，无需用户登录 | ✓ |
| 手动启动 | 服务注册但不自动启动，需要手动 sc start 或依赖其他触发器 | |

**User's choice:** 自动启动 (推荐)
**Notes:** 匹配 ROADMAP "系统启动即运行，无需用户登录桌面" 核心诉求

| Option | Description | Selected |
|--------|-------------|----------|
| 固定描述 | 硬编码中文描述"自动保持 nanobot 处于最新版本" | ✓ |
| 可配置描述 | 在 config.yaml 中新增 service.description 字段，用户可自定义 | |

**User's choice:** 固定描述 (推荐)
**Notes:** 保持配置简洁，描述固定即可

---

## 服务卸载策略

| Option | Description | Selected |
|--------|-------------|----------|
| 先停止再卸载 | 检测到已注册服务后，先调用 Control(svc.Stop) 停止，等停止完成后再 DeleteService | ✓ |
| 仅标记删除 | 直接 DeleteService 标记删除，SCM 在服务停止后真正删除 | |
| 运行中跳过 | 如果服务正在运行则跳过卸载，仅日志提示用户手动停止后重试 | |

**User's choice:** 先停止再卸载 (推荐)
**Notes:** 安全且完整，确保服务完全停止后再删除

| Option | Description | Selected |
|--------|-------------|----------|
| 继续运行 | 卸载服务后继续以控制台模式运行 | ✓ |
| 退出（退出码 3） | 卸载服务后以退出码 3 退出，区分"注册后退出(2)"和"卸载后退出(3)" | |

**User's choice:** 继续运行 (推荐)
**Notes:** 适合"关掉服务但保留手动运行"场景

---

## SCM 恢复策略

| Option | Description | Selected |
|--------|-------------|----------|
| 无限重启 | 第一次/第二次/后续失败均重启服务，60秒间隔，24小时重置计数器 | ✓ |
| 最多重启 3 次 | 三次失败后不再重启，24小时重置计数 | |
| 可配置策略 | 用户可在 config.yaml 中配置重启次数、间隔、重置时间 | |

**User's choice:** 无限重启 (推荐)
**Notes:** 简单有效，确保后台监控服务持续运行

---

## 权限与跨平台

| Option | Description | Selected |
|--------|-------------|----------|
| 检测并报错 | 使用 OpenProcessToken + TokenElevation 检测。非管理员时输出错误提示并退出（退出码 1） | ✓ |
| UAC 提升提示 | 非管理员时提示用户并尝试通过 ShellExecuteEx runas 触发 UAC 提升 | |

**User's choice:** 检测并报错 (推荐)
**Notes:** 简洁明了，不引入 UAC 弹窗复杂性

---

## 边界情况

| Option | Description | Selected |
|--------|-------------|----------|
| 跳过注册 | 检测到已注册服务时跳过注册，日志提示服务已存在 | ✓ |
| 重新注册 | 先卸载旧服务再重新注册 | |
| 更新配置 | 保留现有注册，仅更新配置（恢复策略等） | |

**User's choice:** 跳过注册 (推荐)
**Notes:** 幂等操作，安全简单

---

## Claude's Discretion

- ServiceManager 文件组织（放在 internal/lifecycle/）
- 错误处理细节（SCM API 错误包装、日志格式）
- 测试策略（SCM 操作的 mock 设计）
- 具体的 Go API 调用方式和参数
- 恢复策略的实现方式（svc/mgr API vs 调用 sc.exe）

## Deferred Ideas

None — discussion stayed within phase scope
