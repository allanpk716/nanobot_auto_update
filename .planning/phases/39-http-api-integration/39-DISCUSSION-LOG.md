# Phase 39: HTTP API Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-30
**Phase:** 39-http-api-integration
**Areas discussed:** 自更新执行模型, 互斥锁策略, 响应格式细节, 错误场景处理

---

## 自更新执行模型

| Option | Description | Selected |
|--------|-------------|----------|
| 异步 + 202 Accepted | 返回 202，后台 goroutine 执行更新，客户端轮询 check 端点 | ✓ |
| 同步等待 | 调用 Update() 后尝试返回 200 OK | |
| 异步 + 自动重启 | 后台执行更新，完成后 os.Exit 或 exec.Command 重启（属于 Phase 40） | |

**User's choice:** 异步 + 202 Accepted (Recommended)
**Notes:** Phase 40 处理重启机制。Phase 39 只负责 API 层：触发更新、检查状态。

---

## 互斥锁策略

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 isUpdating | 自更新和 trigger-update 共享同一个 atomic.Bool 锁 | ✓ |
| 独立锁 | 新增 isSelfUpdating 锁 | |
| 统一锁管理器 | 引入 UpdateManager 统一管理 | |

**User's choice:** 复用 isUpdating (Recommended)
**Notes:** 符合 API-02 互斥要求。任何一个更新进行中，另一个返回 409。

---

## Check 响应格式

| Option | Description | Selected |
|--------|-------------|----------|
| 详细模式 | 返回 current_version, latest_version, needs_update, release_notes, published_at, download_url, self_update_status | ✓ |
| 精简模式 | 只返回 current_version, latest_version, needs_update | |

**User's choice:** 详细模式 (Recommended)
**Notes:** 第三方程序可自行决定是否更新，获取完整 Release 信息。

---

## 状态轮询方式

| Option | Description | Selected |
|--------|-------------|----------|
| 复用 check 端点 | GET /api/v1/self-update/check 加 self_update_status 字段 | ✓ |
| 专用状态端点 | 新增 GET /api/v1/self-update/status | |

**User's choice:** 复用 check 端点 (Recommended)
**Notes:** 避免新增端点，客户端只需轮询一个 URL。

---

## 错误码策略

| Option | Description | Selected |
|--------|-------------|----------|
| 标准复用 | 401/409/500/503，与 trigger-update 一致 | ✓ |
| 细粒度扩展 | 增加更多错误码如 422/502 | |

**User's choice:** 标准复用 (Recommended)
**Notes:** 401 认证失败, 409 更新进行中, 500 更新失败, 503 自更新未配置。

---

## Help 端点集成

| Option | Description | Selected |
|--------|-------------|----------|
| 直接添加 | 在 getEndpoints() 中添加 self_update_check 和 self_update 两个端点 | ✓ |
| 扩展配置区 | Help 端点增加自更新配置 section | |

**User's choice:** 直接添加 (Recommended)
**Notes:** 简单直接，与现有端点保持相同格式。

---

## Claude's Discretion

- SelfUpdateHandler struct 设计和字段
- 状态管理实现方式
- 日志字段命名
- 测试策略

## Deferred Ideas

None — discussion stayed within phase scope
