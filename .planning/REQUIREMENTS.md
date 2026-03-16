# Requirements: Nanobot Auto Updater v0.3

**Defined:** 2026-03-16
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预

## v0.3 Requirements

v0.3 里程碑将项目从定时更新工具转变为持续运行的监控服务 + HTTP API 触发模式。

### Configuration

- [ ] **CONF-01**: 用户可以在 YAML 配置文件中配置 Pushover credentials (token/user)，不再依赖环境变量
- [ ] **CONF-02**: 用户可以在 YAML 配置文件中配置 HTTP API 端口号
- [ ] **CONF-03**: 用户可以在 YAML 配置文件中配置 Bearer Token (API 认证)
- [ ] **CONF-04**: 用户可以在 YAML 配置文件中配置 Google 监控间隔时间
- [ ] **CONF-05**: 用户可以在 YAML 配置文件中配置 HTTP 请求超时时间
- [ ] **CONF-06**: 系统在启动时验证所有必需配置项，缺失或无效配置时拒绝启动并返回明确错误

### HTTP API Service

- [ ] **API-01**: 用户可以通过 POST /api/v1/trigger-update 触发 nanobot 更新
- [ ] **API-02**: 用户需要在 HTTP 请求中提供 Bearer Token 进行认证
- [ ] **API-03**: 系统拒绝无效或缺失 Token 的请求，返回 HTTP 401 Unauthorized
- [ ] **API-04**: 系统返回 JSON 格式响应，包含 status 和 message 字段
- [ ] **API-05**: 系统在更新成功时返回 HTTP 200 OK + 成功详情
- [ ] **API-06**: 系统在更新失败时返回 HTTP 500 Internal Server Error + 错误详情
- [ ] **API-07**: 系统在已有更新进行中时返回 HTTP 409 Conflict + 提示信息
- [ ] **API-08**: 系统记录所有 API 请求日志 (包含请求时间、结果)
- [ ] **API-09**: 系统在 API 触发更新失败时通过 Pushover 发送通知

### Monitoring Service

- [ ] **MON-01**: 系统每 15 分钟自动检查 Google 连通性 (HTTP GET https://www.google.com)
- [ ] **MON-02**: 系统在首次检测到连通性失败时通过 Pushover 发送失败通知
- [ ] **MON-03**: 系统在连通性恢复时通过 Pushover 发送恢复通知
- [ ] **MON-04**: 系统记录所有监控检查日志 (包含检查时间、结果、连通性状态)
- [ ] **MON-05**: 系统在监控检查失败时继续运行，不中断监控服务
- [ ] **MON-06**: 系统在 Google 连通性失败时尝试触发 nanobot 更新
- [ ] **MON-07**: 系统在已有更新进行中时跳过监控触发的更新，等待下次检查周期
- [ ] **MON-08**: 系统使用超时机制 (10秒) 防止 HTTP 请求无限期挂起

### Architecture Changes

- [ ] **ARCH-01**: 系统移除 cron 定时调度功能
- [ ] **ARCH-02**: 系统移除 --update-now 命令行参数
- [ ] **ARCH-03**: 系统启动时自动启动 HTTP API 服务器和监控服务
- [ ] **ARCH-04**: 系统实现共享更新锁，防止 HTTP API 和监控服务并发触发更新
- [ ] **ARCH-05**: 系统实现优雅停机 (处理 Ctrl+C)，停止接受新请求，等待处理中请求完成
- [ ] **ARCH-06**: 系统确保所有 goroutine 在停机时正确清理，无资源泄漏

### Security

- [ ] **SEC-01**: 系统使用常量时间比较验证 Bearer Token，防止时序攻击
- [ ] **SEC-02**: 系统不在日志中记录完整的 Bearer Token
- [ ] **SEC-03**: 系统在启动时验证 Bearer Token 长度至少 32 字符

## v0.4 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Enhanced Monitoring

- **MON-09**: 支持配置多个监控目标 (Google + 其他服务)
- **MON-10**: 支持自定义监控间隔 (不同目标不同间隔)
- **MON-11**: 支持连续失败阈值触发 (N 次失败后触发更新)

### Enhanced API

- **API-10**: 支持 GET /api/v1/status 返回系统状态
- **API-11**: 支持 GET /api/v1/health 健康检查端点
- **API-12**: 支持更详细的 JSON 响应 (包含每个实例的状态)
- **API-13**: 支持 API 请求速率限制

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| JWT 认证 | 单用户内部服务，静态 Bearer Token 足够，避免过度复杂化 |
| GUI 管理界面 | 命令行工具，无需图形界面 |
| 多监控目标 | v0.3 聚焦单目标 (Google)，多目标增加指数级复杂度 |
| 响应体内容检查 | Google 首页内容因地区变化，仅检查 HTTP 状态码更可靠 |
| 自动重试逻辑 | 隐藏真实连通性问题，延迟故障检测，下次周期会重试 |
| 数据库历史记录 | 超出项目范围，日志文件提供足够的历史记录 |
| Prometheus metrics | 内部服务无需复杂监控，日志 + Pushover 足够 |
| 配置热重载 | 增加实现复杂度，重启服务可接受 |
| API 版本控制 | 单端点无需版本管理，未来需要时再引入 |
| 开机自启动 | 用户手动启动，保持用户控制 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 1 | Pending |
| CONF-02 | Phase 1 | Pending |
| CONF-03 | Phase 1 | Pending |
| CONF-04 | Phase 1 | Pending |
| CONF-05 | Phase 1 | Pending |
| CONF-06 | Phase 1 | Pending |
| MON-01 | Phase 2 | Pending |
| MON-04 | Phase 2 | Pending |
| MON-05 | Phase 2 | Pending |
| MON-08 | Phase 2 | Pending |
| API-01 | Phase 3 | Pending |
| API-02 | Phase 3 | Pending |
| API-03 | Phase 3 | Pending |
| API-04 | Phase 3 | Pending |
| API-08 | Phase 3 | Pending |
| ARCH-04 | Phase 4 | Pending |
| API-05 | Phase 4 | Pending |
| API-06 | Phase 4 | Pending |
| API-07 | Phase 4 | Pending |
| API-09 | Phase 4 | Pending |
| MON-02 | Phase 5 | Pending |
| MON-03 | Phase 5 | Pending |
| MON-06 | Phase 5 | Pending |
| MON-07 | Phase 5 | Pending |
| ARCH-01 | Phase 7 | Pending |
| ARCH-02 | Phase 7 | Pending |
| ARCH-03 | Phase 6 | Pending |
| ARCH-05 | Phase 6 | Pending |
| ARCH-06 | Phase 8 | Pending |
| SEC-01 | Phase 3 | Pending |
| SEC-02 | Phase 3 | Pending |
| SEC-03 | Phase 1 | Pending |

**Coverage:**
- v0.3 requirements: 31 total
- Mapped to phases: 31
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-16*
*Last updated: 2026-03-16 after initial definition*
