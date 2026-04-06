# Requirements: Nanobot Auto Updater — v0.9

**Defined:** 2026-04-06
**Core Value:** 自动保持 nanobot 处于最新版本,无需用户手动干预。

## v0.9 Requirements

### Startup Notification

- [ ] **STRT-01**: 多实例启动结果聚合成一条 Pushover 通知（包含每个实例的启动状态）
- [ ] **STRT-02**: 异步发送启动通知，不阻塞启动流程
- [ ] **STRT-03**: Pushover 未配置时优雅降级（跳过通知）

### Telegram Monitoring

- [ ] **TELE-01**: 检测日志中 "Starting Telegram bot" 自动触发连接监控
- [ ] **TELE-02**: 30 秒内检测 "Telegram bot commands registered" 判定连接成功
- [ ] **TELE-03**: 检测日志中 "httpx.ConnectError" 判定连接失败
- [ ] **TELE-04**: 30 秒超时未检测到成功标志判定连接失败
- [ ] **TELE-05**: Telegram 连接成功时发送 Pushover 通知
- [ ] **TELE-06**: Telegram 连接失败时发送 Pushover 通知
- [ ] **TELE-07**: 未检测到 "Starting Telegram bot" 则不启动监控
- [ ] **TELE-08**: 历史日志重放不触发误报（仅监控新写入的日志）
- [ ] **TELE-09**: 实例停止时取消正在进行的 Telegram 监控

## v2 Requirements

(None deferred)

## Out of Scope

| Feature | Reason |
|---------|--------|
| 自动重连/重启 Telegram | 监控器只观察不执行，避免与 nanobot 自身逻辑冲突 |
| Telegram API 健康检查 | 重复 nanobot 已有的逻辑，可能有代理问题 |
| 可配置日志模式 | v0.9 仅支持硬编码 Telegram 模式，未来可扩展 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| STRT-01 | — | Pending |
| STRT-02 | — | Pending |
| STRT-03 | — | Pending |
| TELE-01 | — | Pending |
| TELE-02 | — | Pending |
| TELE-03 | — | Pending |
| TELE-04 | — | Pending |
| TELE-05 | — | Pending |
| TELE-06 | — | Pending |
| TELE-07 | — | Pending |
| TELE-08 | — | Pending |
| TELE-09 | — | Pending |

**Coverage:**
- v0.9 requirements: 12 total
- Mapped to phases: 0
- Unmapped: 12 ⚠️

---
*Requirements defined: 2026-04-06*
*Last updated: 2026-04-06 after initial definition*
