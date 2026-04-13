# Phase 50: Instance Config CRUD API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 50-instance-config-crud-api
**Areas discussed:** API 路由设计, 配置持久化与热重载, 复制实例策略, 验证与错误处理

---

## API 路由设计

| Option | Description | Selected |
|--------|-------------|----------|
| 方案 A: instance-configs | 独立资源路径 /api/v1/instance-configs，RESTful 语义清晰，避免与现有 /instances 混淆 | ✓ |
| 方案 B: 扩展 /instances | 复用现有 /instances 路径，Go 1.22 mux 支持同路径不同方法，路径简洁但混在一起 | |

**User's choice:** 方案 A: instance-configs
**Notes:** 使用独立资源路径更清晰。复制用 POST .../copy 子资源。JSON 字段与 YAML 一致（snake_case）。startup_timeout API 层用秒数。

---

## 配置持久化与热重载

| Option | Description | Selected |
|--------|-------------|----------|
| A: 直接写文件 + 热重载 | CRUD 直接写 config.yaml，依赖 500ms debounce 热重载。简单可靠但每次全量重启 | ✓ |
| B: 智能更新 + 绕过热重载 | 精确更新单个实例，需要屏蔽 fsnotify。复杂但更精确 | |

| Option | Description | Selected |
|--------|-------------|----------|
| viper.WriteConfig() | 使用 viper 已有全局实例直接写。可能丢失注释但简单 | ✓ |
| 直接 yaml.Marshal | 完全控制输出格式。需要额外处理与 viper 同步 | |

**User's choice:** 方案 A + viper.WriteConfig()
**Notes:** 全量替换在 CRUD 场景可接受。写入使用 viper.WriteConfig()。

---

## 复制实例策略

| Option | Description | Selected |
|--------|-------------|----------|
| 后缀 -copy | 如 gateway → gateway-copy。用户可覆盖 name | ✓ |
| 用户必须指定名称 | 不自动生成，必须由用户提供 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 原端口+1 递增 | 18790 → 18791，已占用则继续递增。用户可覆盖 port | ✓ |
| 用户必须指定端口 | 不自动分配，必须由用户提供 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 不在 Phase 50 处理 | nanobot 配置目录留给 Phase 52 | ✓ |
| Phase 50 创建目录 | 同时创建目录和默认 config.json | |

**User's choice:** -copy 后缀 + 端口递增 + 不处理 nanobot 目录
**Notes:** 复制只克隆 auto-updater 配置。nanobot 目录和配置留给 Phase 52。

---

## 验证与错误处理

| Option | Description | Selected |
|--------|-------------|----------|
| 422 + 详细字段错误 | 返回 errors 数组，含每个字段的具体错误信息 | ✓ |
| 400 + 合并错误文本 | 返回合并的错误消息字符串 | |

| Option | Description | Selected |
|--------|-------------|----------|
| 自动停止后删除 | 调用现有 StopAllNanobots 停止实例后删除配置 | ✓ |
| 拒绝删除运行中实例 | 返回 409 Conflict，要求用户先 stop | |

**User's choice:** 422 详细字段错误 + 自动停止后删除
**Notes:** 复用现有 InstanceConfig.Validate() 和 Config.Validate()（含唯一性检查）。实例不存在返回 404。

---

## Claude's Discretion

- Handler 结构设计
- 配置读写并发安全机制
- CRUD handler 与 viper/热重载的具体集成方式
- copy 端点请求体结构
- 列表端点响应格式
- 写入失败时错误恢复策略

## Deferred Ideas

- nanobot 配置目录创建和默认 config.json — Phase 52
- 智能增量热重载（只重启被修改的实例）— 未来优化
- 批量操作 API — 未来里程碑
