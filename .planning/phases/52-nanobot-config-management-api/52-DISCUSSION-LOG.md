# Phase 52: Nanobot Config Management API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 52-nanobot-config-management-api
**Areas discussed:** 目录路径解析, 默认配置模板, Create/copy 集成, 运行中实例行为

---

## 目录路径解析

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcode ~/.nanobot-{name} | 固定规则映射实例名到目录路径 | |
| Configurable per-instance | InstanceConfig 新增 nanobot_dir 字段 | |
| Parse from start_command | 从 --config 参数自动解析路径 | ✓ |

**User's choice:** Parse from start_command
**Notes:** start_command 中已包含 --config 路径，无需额外字段。Fallback 到 ~/.nanobot-{name}/config.json 用于没有 --config 的情况。

---

## 默认配置模板

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal skeleton | 只保留必要字段（workspace/port/gateway），去掉敏感内容 | |
| Full structure with empty secrets | 保留所有段落结构，apiKey/token 留空字符串 | ✓ |
| Clone from template instance | 从现有实例完整复制 | |

**User's choice:** Full structure with empty secrets
**Notes:** 用户提供了完整 nanobot config.json 示例作为模板。自动参数化 gateway.port 和 workspace，敏感值清空。

---

## Create/copy 集成

| Option | Description | Selected |
|--------|-------------|----------|
| Inject into Phase 50 handler | 注入 NanobotConfigManager 到 InstanceConfigHandler | ✓ |
| Route-level middleware chain | 路由层链式调用 | |
| Separate API calls | 用户单独调用 nanobot-config API | |

**User's choice:** Inject into Phase 50 handler
**Notes:** 需修改 Phase 50 的 InstanceConfigHandler 构造函数和 NewServer() 传参。Create 创建默认配置，Copy 克隆并更新 port/workspace。

---

## 运行中实例行为

| Option | Description | Selected |
|--------|-------------|----------|
| Write only, no restart | PUT 只写文件，用户手动重启 | ✓ |
| Auto-restart after write | 写入后自动重启实例 | |
| Write + optional restart param | 提供可选 restart 参数 | |

**User's choice:** Write only, no restart
**Notes:** 最简单最安全。用户通过 Phase 51 lifecycle API 手动重启。

---

## Claude's Discretion

- nanobot config 写入时的 JSON 格式验证（至少确保是合法 JSON）
- 文件写入并发安全机制（mutex 保护同实例的并发写入）
- NanobotConfigManager 的具体 struct 设计和接口定义
- 目录创建时的错误处理
- 响应 JSON 的包装格式

## Deferred Ideas

- PUT 后自动重启实例（可选参数）— 未来考虑
- nanobot config schema 验证 — 未来里程碑
