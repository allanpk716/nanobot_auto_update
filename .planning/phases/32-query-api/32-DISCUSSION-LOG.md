# Phase 32: Query API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-28
**Phase:** 32-query-api
**Areas discussed:** 数据源和启动恢复, 响应 JSON 结构, 日志排序方向

---

## 数据源和启动恢复

| Option | Description | Selected |
|--------|-------------|----------|
| 启动时从文件恢复 | 读取 JSONL 文件，解析加载到内存 slice。查询走内存。 | ✓ |
| 仅返回本次运行日志 | 不恢复文件，只返回当前 session 的日志。 | |
| 查询时直接读文件 | 每次查询从 JSONL 文件读取。内存占用小但性能差。 | |

**User's choice:** 启动时从文件恢复
**Notes:** 配合 Phase 31 启动时清理，先清理再恢复，确保只加载 7 天内的记录。使用 bufio.Scanner 流式读取。

---

## 响应 JSON 结构

| Option | Description | Selected |
|--------|-------------|----------|
| 嵌套结构 {data, meta} | 标准REST分页模式，数据和元数据分离。 | ✓ |
| 扁平结构 {logs, total, offset, limit} | 列表和分页信息在同一层级。 | |

**User's choice:** 嵌套结构 {data: [...], meta: {total, offset, limit}}
**Notes:** data 字段包含完整 UpdateLog 对象列表，meta 字段包含分页元数据。

---

## 日志排序方向

| Option | Description | Selected |
|--------|-------------|----------|
| 最新优先 (descending) | offset=0 返回最新日志，用户通常关心最近的更新。 | ✓ |
| 最早优先 (ascending) | offset=0 返回最旧日志，传统分页模式。 | |

**User's choice:** 最新优先
**Notes:** 类似 GitHub API 的分页行为。内存 slice 按记录顺序存储（旧到新），查询时反向遍历。

---

## Claude's Discretion

- 启动恢复的具体实现细节（bufio 缓冲区大小、错误恢复策略）
- 分页参数校验的具体错误消息措辞
- UpdateLogger 中添加恢复方法的签名和位置
- 查询 handler 的代码组织
- meta 字段中是否包含 page 计算

## Deferred Ideas

None — 讨论保持在阶段范围内
