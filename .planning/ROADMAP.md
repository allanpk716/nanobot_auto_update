# Roadmap: Nanobot Auto Updater

## Milestones

- ✅ **v1.0 Single Instance Auto-Update** - Phases 01-04 (shipped 2026-02-18)
- ✅ **v0.2 Multi-Instance Support** - Phases 05-18 (shipped 2026-03-16)
- ✅ **v0.4 Real-time Log Viewing** - Phases 19-23 (shipped 2026-03-20)
- ✅ **v0.5 Core Monitoring and Automation** - Phases 24-29 (shipped 2026-03-24)
- 🚧 **v0.6 Update Log Recording and Query System** - Phases 30-33 (in progress)

## Overview

v0.6 里程碑在现有 HTTP API 触发更新功能基础上,添加持久化更新日志记录和查询能力。系统将使用 JSON Lines 格式记录每次更新操作的元数据和实例详情,提供分页查询 API,并自动清理 7 天前的历史记录。

## Phases

<details>
<summary>✅ v1.0 Single Instance Auto-Update (Phases 01-04) - SHIPPED 2026-02-18</summary>

基础自动更新功能已交付。

</details>

<details>
<summary>✅ v0.2 Multi-Instance Support (Phases 05-18) - SHIPPED 2026-03-16</summary>

多实例管理功能已交付。

</details>

<details>
<summary>✅ v0.4 Real-time Log Viewing (Phases 19-23) - SHIPPED 2026-03-20</summary>

实时日志查看功能已交付。

</details>

<details>
<summary>✅ v0.5 Core Monitoring and Automation (Phases 24-29) - SHIPPED 2026-03-24</summary>

核心监控和自动化功能已交付。

</details>

### 🚧 v0.6 Update Log Recording and Query System (In Progress)

**Milestone Goal:** 记录每次 HTTP API 触发的更新操作,并提供查询接口获取更新历史日志

#### Phase 30: Log Structure and Recording
**Goal**: 系统能够记录每次更新操作的元数据和实例详情
**Depends on**: Phase 29 (HTTP help 接口完成)
**Requirements**: LOG-01, LOG-02, LOG-03, LOG-04
**Success Criteria** (what must be TRUE):
  1. 每次更新操作生成唯一的 UUID v4 标识符并在 trigger-update 响应中返回
  2. 系统记录更新的开始时间戳、结束时间戳和整体状态 (success/partial_success/failed)
  3. 系统记录每个实例的更新详情 (名称、端口、状态、错误消息)
  4. 系统计算并存储从开始到结束的总耗时 (毫秒级精度)
  5. 所有时间戳使用 UTC 时区存储
**Plans**: 2 plans

Plans:
- [x] 30-01-PLAN.md — Create UpdateLog data model and UpdateLogger component
- [x] 30-02-PLAN.md — Integrate UpdateLogger into TriggerHandler with UUID generation

#### Phase 31: File Persistence
**Goal**: 系统能够持久化更新日志到 JSON Lines 文件并自动清理旧记录
**Depends on**: Phase 30 (日志结构和记录完成)
**Requirements**: STORE-01, STORE-02
**Success Criteria** (what must be TRUE):
  1. 更新日志以 JSON Lines 格式持久化到 ./logs/updates.jsonl 文件
  2. 文件写入使用原子追加和 sync.Mutex 保护避免并发冲突
  3. 应用启动时自动删除 7 天前的日志记录
  4. 清理过程不阻塞正常的读写操作
  5. 日志文件不存在时自动创建
**Plans**: TBD

#### Phase 32: Query API
**Goal**: 用户能够通过 HTTP API 查询更新历史日志
**Depends on**: Phase 31 (文件持久化完成)
**Requirements**: QUERY-01, QUERY-02, QUERY-03
**Success Criteria** (what must be TRUE):
  1. 用户可以通过 GET /api/v1/update-logs 查询更新日志列表
  2. 查询接口使用 Bearer Token 认证保护 (复用 Phase 28 的 AuthMiddleware)
  3. 查询结果包含分页元数据 (总数、当前页、每页数量)
  4. 用户可以通过 limit 参数控制每页数量 (默认 20,最大 100)
  5. 用户可以通过 offset 参数控制分页偏移 (默认 0,最小 0)
  6. 查询使用流式读取避免内存问题,offset 超出范围时返回空列表
**Plans**: TBD

#### Phase 33: Integration and Testing
**Goal**: 日志记录集成到现有更新流程并通过端到端测试验证
**Depends on**: Phase 32 (查询 API 完成)
**Requirements**: None (集成和测试阶段)
**Success Criteria** (what must be TRUE):
  1. POST /api/v1/trigger-update 触发更新后自动记录日志到文件
  2. GET /api/v1/update-logs 能够查询到最近的更新记录
  3. 日志记录失败不影响更新操作本身 (非阻塞记录)
  4. 1000+ 条记录的查询响应时间 < 500ms
  5. 更新 ID 在响应和查询结果中一致
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 30 → 31 → 32 → 33

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 30. Log Structure and Recording | v0.6 | 2/2 | Complete   | 2026-03-27 |
| 31. File Persistence | v0.6 | 0/3 | Not started | - |
| 32. Query API | v0.6 | 0/3 | Not started | - |
| 33. Integration and Testing | v0.6 | 0/3 | Not started | - |

## Coverage Map

| Requirement | Phase | Category |
|-------------|-------|----------|
| LOG-01 | Phase 30 | Core Logging |
| LOG-02 | Phase 30 | Core Logging |
| LOG-03 | Phase 30 | Core Logging |
| LOG-04 | Phase 30 | Core Logging |
| STORE-01 | Phase 31 | Storage |
| STORE-02 | Phase 31 | Storage |
| QUERY-01 | Phase 32 | Query API |
| QUERY-02 | Phase 32 | Query API |
| QUERY-03 | Phase 32 | Query API |

**Total v1 requirements:** 9
**Mapped to phases:** 9
**Coverage:** 100%

## Dependencies

```
Phase 29 (v0.5)
    ↓
Phase 30 (Log Structure)
    ↓
Phase 31 (File Persistence)
    ↓
Phase 32 (Query API)
    ↓
Phase 33 (Integration)
```

## Notes

- 所有阶段遵循研究建议,使用 Go 标准库实现
- Phase 30 建立数据模型,Phase 31 实现持久化,Phase 32 提供查询接口,Phase 33 集成验证
- Phase 31 和 Phase 33 可能需要额外研究 (文件清理原子性、LogBuffer 快照读取)

---

*Last updated: 2026-03-27*
