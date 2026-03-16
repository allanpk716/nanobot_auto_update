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

- [x] **Phase 11: Configuration Extension** - 扩展配置支持 API 和监控服务参数 (completed 2026-03-16)
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
2. 用户可以在 config.yaml 中配置 API 端口、 Bearer Token、监控间隔、请求超时
3. 系统在启动时验证 Bearer Token 长度 >= 32 字符，否则拒绝启动
4. 系统在启动时验证所有必需配置项存在且有效，缺失时返回明确错误信息
5. 所有配置项有合理的默认值（除 token 外)，应用可以成功启动

**Plans:** 4/4 plans complete

Plans:
- [x] 11-01a-PLAN.md — Create Wave 0 unit test scaffolding for API and Monitor validation
- [x] 11-01b-PLAN.md — Create Wave 0 test data files and integration test stubs
- [x] 11-02-PLAN.md — Implement APIConfig and MonitorConfig validation with TDD
- [x] 11-03-PLAN.md — Integrate new configs into main Config struct and startup validation

---

### Phase 12: Monitoring Service

**Goal:** 系统每 15 分钟自动检查 Google 连通性，记录日志，失败时继续运行

**Depends on:** Phase 11

**Requirements:** MON-01, MON-04, MON-05, MON-08

**Success Criteria** (what must be TRUE when this phase completes):
1. 系统每 15 分钟自动发起 HTTP GET 请求到 https://www.google.com
2. 系统在每次检查时记录日志（包含时间、结果、连通性状态)
3. 系统使用 10 秒超时防止 HTTP 请求挂起，超时后继续运行
4. 系统在检查失败时不会崩溃或中断监控服务，继续等待下次周期
5. 监控服务响应 Ctrl+C 信号，优雅停止 ticker 和 goroutine

**Plans:** 2 plans

Plans:
- [ ] 12-01-PLAN.md — Checker: HTTP 连通性检查器实现
- [ ] 12-02-PLAN.md — Service: 监控服务主体实现

