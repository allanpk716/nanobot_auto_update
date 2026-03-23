# Phase 28: HTTP API Trigger - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-23
**Phase:** 28-http-api-trigger
**Areas discussed:** 认证机制, 并发控制, 响应格式, 更新范围和超时, HTTP 状态码, 日志记录

---

## 认证机制

### Bearer Token 传递方式

| Option | Description | Selected |
|--------|-------------|----------|
| 标准 Bearer Token | 标准 HTTP 认证方式，符合 RFC 6750。Authorization: Bearer \<token\>。与大多数 HTTP 客户端和工具兼容。 | ✓ |
| 自定义 Header | 自定义 header，如 X-API-Key。更灵活，但不遵循 HTTP 标准。某些 HTTP 客户端可能需要额外配置。 | |

**User's choice:** 标准 Bearer Token (Recommended)
**Notes:** 用户选择标准的 RFC 6750 方式，确保与所有 HTTP 客户端兼容。

### 认证失败响应格式

| Option | Description | Selected |
|--------|-------------|----------|
| JSON 格式 | 返回 JSON 错误响应，符合 RFC 7807 Problem Details。示例：{"error": "unauthorized", "message": "Invalid or missing Bearer token"}。与 REST API 最佳实践一致。 | ✓ |
| 纯文本格式 | 返回纯文本错误消息，如 "Unauthorized: Invalid Bearer token"。简单，但不适合 API 客户端解析。 | |

**User's choice:** JSON 格式 (Recommended)
**Notes:** 用户选择 JSON 格式，便于客户端解析和错误处理。

---

## 并发控制

### 并发策略

| Option | Description | Selected |
|--------|-------------|----------|
| 状态标志 + 立即拒绝 | 使用 atomic.Bool 标志跟踪更新状态。如果正在更新，立即返回 409 Conflict + JSON 错误。实现简单，性能开销低，适合单服务器场景。 | ✓ |
| 互斥锁 + 排队等待 | 使用 sync.Mutex 互斥锁。第二个请求会等待第一个完成，可能导致超时。适合需要排队执行的场景。 | |
| 信号量 + 限流 | 使用带容量的 channel 作为信号量。可以限制并发更新数量（如最多 3 个同时更新）。适合需要限流而非完全禁止并发的场景。 | |

**User's choice:** 状态标志 + 立即拒绝 (Recommended)
**Notes:** 用户选择简单的 atomic.Bool 方案，适合单服务器场景，避免复杂的排队机制。

### 重复请求处理

| Option | Description | Selected |
|--------|-------------|----------|
| 409 Conflict + JSON | 返回 409 Conflict 状态码，符合 HTTP 语义。JSON 响应包含 error: "conflict" 和 message: "Update already in progress"。客户端可以稍后重试。 | ✓ |
| 429 Too Many Requests | 返回 429 Too Many Requests，通常用于限流场景。虽然不是严格意义上的限流，但某些 API 网关可能期望这个状态码。 | |

**User's choice:** 409 Conflict + JSON (Recommended)
**Notes:** 用户选择 409 Conflict，更符合 HTTP 语义（资源状态冲突而非限流）。

---

## 响应格式

### JSON 响应字段

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 CLI 格式 | 与 Phase 05 CLI 的 JSON 格式保持一致。成功：{"success": true, "version": "1.2.3", "source": "github", "message": "Update completed"}。失败：{"success": false, "error": "Network timeout", "exit_code": 1}。客户端可以复用现有的解析逻辑。 | ✓ |
| 新的 API 专用格式 | 设计新的 API 专用格式，可能包含更多字段如 instances_updated、duration、detailed_errors。更详细，但与 CLI 格式不一致。 | |

**User's choice:** 复用 CLI 格式 (Recommended)
**Notes:** 用户选择复用 Phase 05 的 JSON 格式，保持一致性，减少客户端集成成本。

---

## 更新范围和超时

### 更新范围

| Option | Description | Selected |
|--------|-------------|----------|
| 更新所有实例 | 触发所有配置实例的更新，与 Phase 24 自动启动行为一致。使用 InstanceManager.UpdateAllInstances() 方法。简单直接，确保所有实例保持同步。 | ✓ |
| 选择性更新实例 | 在请求中指定实例名称，如 {"instances": ["gateway", "worker"]}。更灵活，但增加 API 复杂度。适合需要独立更新某些实例的场景。 | |

**User's choice:** 更新所有实例 (Recommended)
**Notes:** 用户选择更新所有实例，与 Phase 24 的自动启动行为保持一致。

### 超时设置

| Option | Description | Selected |
|--------|-------------|----------|
| 使用配置的 api.timeout | 使用配置文件的 api.timeout (默认 30s) 作为整个更新流程的超时。如果更新未在 30s 内完成，返回 504 Gateway Timeout。客户端可以调整配置文件中的超时时间。 | ✓ |
| 动态计算超时 | 根据实例数量动态计算超时（如每个实例 45s）。更智能，但可能导致超时时间过长（如 5 个实例 = 225s）。适合实例数量不固定的场景。 | |

**User's choice:** 使用配置的 api.timeout (Recommended)
**Notes:** 用户选择使用配置文件的超时设置，简单直接，客户端可以灵活调整。

---

## HTTP 状态码

### 成功状态码

| Option | Description | Selected |
|--------|-------------|----------|
| 200 OK | 同步更新完成后返回 200 OK + JSON 结果。客户端等待时间可能较长（根据实例数量和超时设置），但可以立即获得更新结果。 | ✓ |
| 202 Accepted (异步) | 立即返回 202 Accepted，更新在后台执行。客户端需要通过其他方式（如查询状态端点）获取更新结果。适合异步模式，但需要额外的状态查询机制。 | |

**User's choice:** 200 OK (Recommended)
**Notes:** 用户选择同步模式返回 200 OK，简化客户端实现，无需轮询状态。

---

## 日志记录

### 日志格式

| Option | Description | Selected |
|--------|-------------|----------|
| 统一日志格式 | API 触发的更新与定时更新使用相同的日志格式和级别（INFO/ERROR）。可以通过日志字段（如 source=api-trigger）区分触发来源。保持一致性，便于日志分析。 | ✓ |
| 专用 API 日志 | API 触发的更新使用专门的日志格式或更详细的日志级别。便于调试 API 调用，但可能导致日志不一致。 | |

**User's choice:** 统一日志格式 (Recommended)
**Notes:** 用户选择统一日志格式，通过 source 字段区分触发来源，便于日志分析。

---

## Claude's Discretion

以下实现细节由 Claude 在规划和执行阶段决定：
- Bearer Token 验证的具体实现（如何提取 header、如何比较 token）
- atomic.Bool 的初始化位置（在 InstanceManager 还是专用的 UpdateHandler）
- 错误消息的具体措辞（中文/英文）
- 日志字段的具体命名（source vs trigger vs api_call）
- 更新流程的具体实现（是否需要新增 UpdateAllInstances 方法）

## Deferred Ideas

讨论过程中未出现超出阶段范围的想法。
