# Phase 30: Log Structure and Recording - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-27
**Phase:** 30-log-structure-and-recording
**Areas discussed:** 数据结构, 时间戳, 状态定义, 实例详情, UUID生成, 响应集成, 记录时机, 存储位置, 日志引用, 耗时计算, 失败处理, 实现范围

---

## 数据结构设计

| Option | Description | Selected |
|--------|-------------|----------|
| 独立结构 (UpdateLog) | 创建独立的 UpdateLog 结构包含 ID、时间戳、状态、实例详情数组,与 UpdateResult 分离,职责清晰,易于扩展 | ✓ |
| 扩展现有结构 (UpdateResult) | 在现有 UpdateResult 基础上添加 ID、时间戳、耗时字段,复用现有结构,减少代码重复 | |
| 组合模式 | 创建 UpdateLog 作为外层包装,内部引用 UpdateResult 作为实例结果,两层结构,保持现有代码不变 | |

**User's choice:** 独立结构 (UpdateLog)
**Notes:** 职责分离清晰,UpdateLog 专注于审计日志,UpdateResult 专注于更新结果

---

## 时间戳格式

| Option | Description | Selected |
|--------|-------------|----------|
| RFC 3339 字符串 | 使用 "2026-03-27T10:30:00.123Z" 格式,人类可读,与 JSON 序列化兼容,Go 标准库支持良好 | ✓ |
| Unix 毫秒时间戳 | 使用 int64 毫秒时间戳,占用空间小,计算性能好,但可读性差 | |
| 两者都存储 | 同时存储字符串和时间戳,兼顾可读性和性能,但增加存储空间 | |

**User's choice:** RFC 3339 字符串
**Notes:** 人类可读,JSON 兼容,Go 标准库支持良好

---

## 状态定义

| Option | Description | Selected |
|--------|-------------|----------|
| 三态分类 | success: 所有实例成功 \| partial_success: 部分成功部分失败 \| failed: 所有实例失败或 UV 更新失败,三态分类清晰完整 | ✓ |
| 简化二态 | 只区分 success/failed,partial_success 归为 failed,简化状态管理,但丢失部分成功的语义 | |
| 详细状态码 | 在 success/failed 基础上添加 warning 状态表示部分成功,三态但语义不同,与 HTTP 状态码映射更复杂 | |

**User's choice:** 三态分类 (success/partial_success/failed)
**Notes:** 清晰区分完全成功、部分成功和完全失败的场景

---

## 实例详情

| Option | Description | Selected |
|--------|-------------|----------|
| 基本信息 | 记录实例名称、端口、更新状态、错误消息 (来自 InstanceError),与 REQUIREMENTS.md 一致 | ✓ |
| 日志引用 | 添加字段引用 LogBuffer 历史记录 (如缓冲区偏移量),Phase 33 集成时可直接定位日志,但需要 LogBuffer API 扩展 | ✓ |
| 耗时明细 | 添加实例的启动耗时、停止耗时单独统计,便于性能分析,增加数据复杂度 | ✓ |

**User's choice:** 基本信息 + 日志引用 + 耗时明细
**Notes:** 完整的实例信息,支持审计、日志定位和性能分析

---

## UUID 生成时机

| Option | Description | Selected |
|--------|-------------|----------|
| HTTP Handler 生成 | 在 TriggerHandler.Handle() 开始时生成 UUID,在调用 TriggerUpdate 前就确定 ID,可以在错误日志中使用 | ✓ |
| Manager 内部生成 | 在 InstanceManager.TriggerUpdate() 内部生成,更新流程开始时才创建,更接近更新操作本身 | |
| Logger 组件生成 | 在专门的 UpdateLogger 组件生成,记录日志时才创建,解耦更新逻辑和日志记录 | |

**User's choice:** HTTP Handler 生成
**Notes:** 在 Handler 开始时生成,可在错误日志中使用 UUID 关联整个更新流程

---

## HTTP 响应集成

| Option | Description | Selected |
|--------|-------------|----------|
| 扩展现有响应 | 在现有 APIUpdateResult 结构上添加 update_id 字段,保持向后兼容,客户端可选择使用新字段 | ✓ |
| 新响应结构 | 创建新的 APIUpdateResponse 结构包装 APIUpdateResult,添加 update_id 等元数据,结构清晰但破坏现有响应格式 | |
| 响应头返回 | 在 HTTP 响应头 X-Update-ID 中返回 UUID,响应体保持不变,最简单的集成方式 | |

**User's choice:** 扩展现有响应 (APIUpdateResult)
**Notes:** 保持向后兼容,客户端可选择使用新字段

---

## 记录时机

| Option | Description | Selected |
|--------|-------------|----------|
| Handler 后记录 | 在 TriggerHandler.Handle() 中,更新完成后调用 UpdateLogger.Record(),Handler 负责记录,职责清晰 | ✓ |
| Manager 内部记录 | 在 InstanceManager.TriggerUpdate() 内部,更新完成后自动记录,Manager 负责记录,解耦 HTTP 层 | |
| 异步队列记录 | 使用 channel 异步记录,Handler 发送结果到 channel,后台 goroutine 负责记录,不阻塞响应 | |

**User's choice:** Handler 后记录
**Notes:** Handler 负责记录,职责清晰,同步记录确保日志完整性

---

## 存储位置

| Option | Description | Selected |
|--------|-------------|----------|
| 内存 slice | 在内存中使用 slice 存储,Phase 31 实现文件持久化时再写入文件,简单实现 | ✓ |
| 直接文件 | 直接写入文件 (./logs/updates.jsonl),Phase 30 就实现文件写入,提前完成 Phase 31 的部分工作 | |
| 数据库存储 | 使用数据库 (SQLite) 存储,更结构化的存储,但增加复杂度和依赖 | |

**User's choice:** 内存 slice
**Notes:** Phase 30 专注数据模型和记录机制,Phase 31 再实现持久化

---

## 日志引用实现

| Option | Description | Selected |
|--------|-------------|----------|
| 索引引用 | 存储实例名和 LogBuffer 中的日志范围 (start_index, end_index),Phase 33 可通过偏移量直接定位,性能好但需要 LogBuffer API 扩展 | ✓ |
| 完整复制 | 在记录时复制实例的 stdout/stderr 内容到日志中,日志自包含,但占用大量空间,可能与 LogBuffer 重复 | |
| 动态引用 | 只存储实例名,查询时通过 InstanceManager 获取对应 LogBuffer 的历史记录,节省空间但需要运行时访问 | |

**User's choice:** 索引引用 (LogBuffer 索引)
**Notes:** 避免复制大量日志内容,节省内存,提高性能

---

## 耗时计算

| Option | Description | Selected |
|--------|-------------|----------|
| 开始结束差值 | 在 UpdateLog 中记录开始时间和结束时间,计算耗时 = 结束 - 开始,精度到毫秒,简单直接 | ✓ |
| 分段计时 | 在 Handler 中使用 time.Since(start) 计算,分别记录每个实例的耗时,更详细但更复杂 | |
| Manager 内部统计 | 在 InstanceManager 内部统计,使用 time.Duration 类型,更精确的类型安全 | |

**User's choice:** 开始结束差值
**Notes:** 简单直接,毫秒级精度,实例级别耗时单独统计

---

## 记录失败处理

| Option | Description | Selected |
|--------|-------------|----------|
| 非阻塞记录 | 日志记录失败不影响更新操作,只记录 ERROR 日志,不返回错误给客户端,确保更新流程稳定 | ✓ |
| 警告客户端 | 日志记录失败时返回警告给客户端,但更新操作本身成功,增加客户端复杂度 | |
| 严格失败 | 日志记录失败时整个更新操作标记为失败,严格的审计策略,但可能影响正常更新流程 | |

**User's choice:** 非阻塞记录
**Notes:** 确保更新流程稳定性,日志记录失败不影响主流程

---

## 实现范围

| Option | Description | Selected |
|--------|-------------|----------|
| 最小范围 | 只实现 UpdateLog 数据结构和 UpdateLogger 组件,在 Handler 中生成 UUID 和记录日志,内存存储,Phase 31 再实现持久化 | ✓ |
| 提前实现文件 | Phase 30 就实现 JSONL 文件写入,提前完成 Phase 31 的部分工作,减少后续阶段工作量 | |
| 完整实现 | 同时实现查询 API (GET /api/v1/update-logs),Phase 30 完成所有日志功能,Phase 31-32 基本跳过 | |

**User's choice:** 最小范围
**Notes:** 专注于 Phase 30 的核心目标,为后续阶段打好基础

---

## Claude's Discretion

- 日志字段的具体命名 (duration_ms vs duration_ms)
- 错误消息的中英文选择
- UpdateLogger 组件的具体实现细节
- InstanceUpdateDetail 的 JSON 标签命名
- LogBuffer 索引的获取方式 (需要 Phase 33 扩展 API)

## Deferred Ideas

None — 讨论保持在阶段范围内
