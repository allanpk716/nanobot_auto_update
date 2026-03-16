# Roadmap: Nanobot Auto Updater v0.3

**Milestone:** v0.3 监控服务和 HTTP API
**Created:** 2026-03-16
**Granularity:** Standard

## Overview

v0.3 里程碑将项目从定时更新工具转变为持续运行的监控服务 + HTTP API 触发模式。核心变化包括:

1. **架构转变**: 移除 cron 定时调度，改为 HTTP API + 后台监控服务双服务并发模式
2. **配置迁移**: Pushover credentials 从环境变量迁移到 YAML 配置文件
3. **新增服务**: HTTP API 服务器 (POST /api/v1/trigger-update) + Google 连通性监控服务
4. **并发控制**: 共享更新锁防止 API 和监控服务并发触发更新
5. **安全增强**: Bearer Token 认证，常量时间比较，防止时序攻击

**Requirements Coverage:** 31/31 requirements mapped

## Phases

- [ ] **Phase 11: Configuration Extension** - 扩展配置支持 API 和监控服务参数
- [ ] **Phase 12: Monitoring Service** - 实现后台连通性监控 goroutine
- [ ] **Phase 13: HTTP API Server** - 实现 HTTP API 服务器和认证中间件
- [ ] **Phase 14: Shared Update Lock** - 实现并发更新控制和集成
- [ ] **Phase 15: Notification Enhancements** - 扩展通知支持失败/恢复场景
- [ ] **Phase 16: Main Coordination** - 主函数集成所有服务，实现优雅停机
- [ ] **Phase 17: Legacy Removal** - 移除 cron 调度和相关命令行参数
- [ ] **Phase 18: End-to-End Validation** - 完整系统测试和陷阱验证

---

## Phase Details

### Phase 11: Configuration Extension

**Goal:** 用户可以在 YAML 配置文件中配置所有新增参数，系统在启动时验证配置有效性

**Depends on:** v0.2 (Phase 10)

**Requirements:** CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, CONF-06, SEC-03

**Success Criteria** (what must be TRUE when this phase completes):

1. 用户可以在 config.yaml 中配置 Pushover token/user，无需环境变量
2. 用户可以在 config.yaml 中配置 API 端口、Bearer Token、监控间隔、请求超时
3. 系统在启动时验证 Bearer Token 长度 ≥ 32 字符，否则拒绝启动
4. 系统在启动时验证所有必需配置项存在且有效，缺失时返回明确错误信息
5. 所有配置项有合理的默认值（除 token 外），应用可以成功启动

**Plans:** 3/4 plans executed

Plans:
- [ ] 11-01a-PLAN.md — Create Wave 0 unit test scaffolding for API and Monitor validation
- [ ] 11-01b-PLAN.md — Create Wave 0 test data files and integration test stubs
- [ ] 11-02-PLAN.md — Implement APIConfig and MonitorConfig validation with TDD
- [ ] 11-03-PLAN.md — Integrate new configs into main Config struct and startup validation

---

### Phase 12: Monitoring Service

**Goal:** 系统每 15 分钟自动检查 Google 连通性，记录日志，失败时继续运行

**Depends on:** Phase 11

**Requirements:** MON-01, MON-04, MON-05, MON-08

**Success Criteria** (what must be TRUE when this phase completes):

1. 系统每 15 分钟自动发起 HTTP GET 请求到 https://www.google.com
2. 系统在每次检查时记录日志（包含时间、结果、连通性状态）
3. 系统使用 10 秒超时防止 HTTP 请求挂起，超时后继续运行
4. 系统在检查失败时不会崩溃或中断监控服务，继续等待下次周期
5. 监控服务响应 Ctrl+C 信号，优雅停止 ticker 和 goroutine

**Plans:** TBD

---

### Phase 13: HTTP API Server

**Goal:** 用户可以通过 HTTP API 触发更新，系统验证认证并返回结构化响应

**Depends on:** Phase 11

**Requirements:** API-01, API-02, API-03, API-04, API-08, SEC-01, SEC-02

**Success Criteria** (what must be TRUE when this phase completes):

1. 用户可以发送 POST 请求到 /api/v1/trigger-update 触发更新
2. 用户必须在请求头提供有效 Bearer Token，否则返回 HTTP 401 Unauthorized
3. 系统返回 JSON 格式响应，包含 status 和 message 字段
4. 系统使用常量时间比较验证 Token，不在日志中记录完整 Token
5. 系统记录所有 API 请求日志（包含请求时间、认证结果）

**Plans:** TBD

---

### Phase 14: Shared Update Lock

**Goal:** HTTP API 和监控服务可以安全共享更新触发能力，避免并发冲突

**Depends on:** Phase 12, Phase 13

**Requirements:** ARCH-04, API-05, API-06, API-07, API-09

**Success Criteria** (what must be TRUE when this phase completes):

1. 系统使用共享锁控制更新操作，同时只能有一个更新流程运行
2. HTTP API 在更新成功时返回 HTTP 200 OK + 成功详情 JSON
3. HTTP API 在更新失败时返回 HTTP 500 + 错误详情 JSON，并发送 Pushover 通知
4. HTTP API 在已有更新进行中时返回 HTTP 409 Conflict + 提示信息
5. 监控服务在更新进行中时跳过触发，等待下次检查周期

**Plans:** TBD

---

### Phase 15: Notification Enhancements

**Goal:** 监控服务在连通性失败和恢复时发送通知，并尝试触发更新

**Depends on:** Phase 12, Phase 14

**Requirements:** MON-02, MON-03, MON-06, MON-07

**Success Criteria** (what must be TRUE when this phase completes):

1. 系统在首次检测到 Google 连通性失败时发送 Pushover 失败通知
2. 系统在连通性从失败恢复时发送 Pushover 恢复通知
3. 系统在检测到连通性失败时尝试触发 nanobot 更新
4. 系统在更新进行中时跳过监控触发的更新，避免重复通知

**Plans:** TBD

---

### Phase 16: Main Coordination

**Goal:** 系统启动时自动启动所有服务，响应 Ctrl+C 优雅停机

**Depends on:** Phase 11, Phase 12, Phase 13, Phase 14, Phase 15

**Requirements:** ARCH-03, ARCH-05

**Success Criteria** (what must be TRUE when this phase completes):

1. 系统启动时自动启动 HTTP API 服务器和监控服务
2. 系统响应 Ctrl+C 信号，停止接受新请求，等待处理中请求完成
3. 所有服务在停机时正确清理资源（HTTP 服务器、监控 ticker、goroutines）

**Plans:** TBD

---

### Phase 17: Legacy Removal

**Goal:** 移除旧的 cron 调度功能和 --update-now 命令行参数

**Depends on:** Phase 16

**Requirements:** ARCH-01, ARCH-02

**Success Criteria** (what must be TRUE when this phase completes):

1. 系统不再包含 cron 定时调度功能代码
2. 系统不再接受 --update-now 命令行参数
3. 所有旧配置项（cron）从配置结构中移除
4. 现有功能（HTTP API + 监控）完全替代旧功能

**Plans:** TBD

---

### Phase 18: End-to-End Validation

**Goal:** 完整系统测试，验证所有组件协同工作，无资源泄漏

**Depends on:** Phase 17

**Requirements:** ARCH-06

**Success Criteria** (what must be TRUE when this phase completes):

1. E2E 测试覆盖 HTTP API 触发更新流程（成功、失败、并发冲突）
2. E2E 测试覆盖监控服务触发更新流程（首次失败、恢复）
3. E2E 测试验证优雅停机无 goroutine 泄漏
4. E2E 测试验证 24 小时长期运行稳定性
5. 所有 6 个 critical pitfalls 已验证避免（goroutine 泄漏、并发冲突、超时、停机、认证、端口）

**Plans:** TBD

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 11. Configuration Extension | 3/4 | In Progress|  |
| 12. Monitoring Service | 0/0 | Not started | - |
| 13. HTTP API Server | 0/0 | Not started | - |
| 14. Shared Update Lock | 0/0 | Not started | - |
| 15. Notification Enhancements | 0/0 | Not started | - |
| 16. Main Coordination | 0/0 | Not started | - |
| 17. Legacy Removal | 0/0 | Not started | - |
| 18. End-to-End Validation | 0/0 | Not started | - |

---

## Coverage Map

| Requirement | Phase | Category |
|-------------|-------|----------|
| CONF-01 | Phase 11 | Configuration |
| CONF-02 | Phase 11 | Configuration |
| CONF-03 | Phase 11 | Configuration |
| CONF-04 | Phase 11 | Configuration |
| CONF-05 | Phase 11 | Configuration |
| CONF-06 | Phase 11 | Configuration |
| SEC-03 | Phase 11 | Security |
| MON-01 | Phase 12 | Monitoring Service |
| MON-04 | Phase 12 | Monitoring Service |
| MON-05 | Phase 12 | Monitoring Service |
| MON-08 | Phase 12 | Monitoring Service |
| API-01 | Phase 13 | HTTP API Service |
| API-02 | Phase 13 | HTTP API Service |
| API-03 | Phase 13 | HTTP API Service |
| API-04 | Phase 13 | HTTP API Service |
| API-08 | Phase 13 | HTTP API Service |
| SEC-01 | Phase 13 | Security |
| SEC-02 | Phase 13 | Security |
| ARCH-04 | Phase 14 | Architecture Changes |
| API-05 | Phase 14 | HTTP API Service |
| API-06 | Phase 14 | HTTP API Service |
| API-07 | Phase 14 | HTTP API Service |
| API-09 | Phase 14 | HTTP API Service |
| MON-02 | Phase 15 | Monitoring Service |
| MON-03 | Phase 15 | Monitoring Service |
| MON-06 | Phase 15 | Monitoring Service |
| MON-07 | Phase 15 | Monitoring Service |
| ARCH-03 | Phase 16 | Architecture Changes |
| ARCH-05 | Phase 16 | Architecture Changes |
| ARCH-01 | Phase 17 | Architecture Changes |
| ARCH-02 | Phase 17 | Architecture Changes |
| ARCH-06 | Phase 18 | Architecture Changes |

**Total:** 31/31 requirements mapped ✓

---

## Dependencies

```
Phase 11 (Config)
    ├─→ Phase 12 (Monitoring)
    └─→ Phase 13 (HTTP API)
           └─→ Phase 14 (Lock + Integration)
                   └─→ Phase 15 (Notifications)
                           └─→ Phase 16 (Main Coord)
                                   └─→ Phase 17 (Legacy Removal)
                                           └─→ Phase 18 (E2E Validation)
```

**Critical Path:** Phase 11 → Phase 12/13 (parallel) → Phase 14 → Phase 15 → Phase 16 → Phase 17 → Phase 18

---

## Architecture Impact

### New Components

- **internal/config/** (扩展): 新增 API、监控、安全配置字段
- **internal/monitor/** (新): Google 连通性监控服务
- **internal/api/** (新): HTTP API 服务器 + Bearer Token 认证
- **internal/lock/** (新): 共享更新锁 (sync.Mutex + TryLock)

### Modified Components

- **cmd/nanobot-auto-updater/** (重大修改): 主函数从 cron 调度改为 errgroup 协调双服务
- **internal/notifier/** (扩展): 新增 NotifyRecovery() 方法

### Removed Components

- **internal/scheduler/** (删除): 移除 cron 调度器包
- **cmd/nanobot-auto-updater/** (删除): 移除 --update-now 参数处理

---

## Risk Mitigation

Based on research, 6 critical pitfalls to avoid:

1. **Goroutine 泄漏** (Phase 12, 18): 所有 ticker 使用 `defer ticker.Stop()`，select 监听 `ctx.Done()`
2. **并发更新冲突** (Phase 14): 使用 `sync.Mutex.TryLock()` 非阻塞模式
3. **HTTP 超时不当** (Phase 12): 设置显式超时（连接 5s，总请求 10s）
4. **优雅停机不完整** (Phase 16): 使用 errgroup + context 分阶段停机
5. **Bearer Token 安全** (Phase 13): 使用 `crypto/hmac.Equal` 常量时间比较
6. **端口冲突启动失败** (Phase 13): 使用 `net.Listen()` 先绑定端口再启动服务器

---

## Research Notes

See `research/SUMMARY.md` for detailed research findings:

- **Stack**: Go 标准库优先，无新增框架依赖
- **Architecture**: 双服务并发模型 + 共享锁协调
- **Pitfalls**: 6 个 critical pitfalls，12 个总陷阱
- **Confidence**: HIGH (基于官方文档 + 生产验证模式)

**Research flags for planning:**
- Phase 12 (Monitoring): Google 连通性检查细节需在计划时深入研究
- Phase 14 (Integration): 并发边缘情况需要实际测试验证

---

*Roadmap created: 2026-03-16*
*Phase 11 planned: 2026-03-16*
*Phase 11 revised: 2026-03-16 (split 11-01 into 11-01a and 11-01b)*
*Next step: `/gsd:execute-phase 11`*
