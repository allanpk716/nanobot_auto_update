# Requirements: Nanobot Auto Updater

**Defined:** 2025-02-18
**Core Value:** 自动保持 nanobot 处于最新版本，无需用户手动干预

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Infrastructure

- [ ] **INFR-01**: 程序支持自定义日志格式输出 (2024-01-01 12:00:00.123 - [INFO]: message)
- [ ] **INFR-02**: 日志存储在 ./logs/ 目录，支持 24 小时轮转，保留 7 天
- [ ] **INFR-03**: 从 ./config.yaml 加载配置
- [ ] **INFR-04**: 配置文件支持 cron 字段（默认 "0 3 * * *"）
- [ ] **INFR-05**: 支持 -config 指定配置文件路径
- [ ] **INFR-06**: 支持 -cron 覆盖配置文件中的 cron 表达式
- [ ] **INFR-07**: 支持 -run-once 立即执行一次更新后退出
- [ ] **INFR-08**: 支持 -version 显示版本信息
- [ ] **INFR-09**: 支持 help 显示帮助信息
- [ ] **INFR-10**: 执行 uv 命令时隐藏命令窗口（使用 SysProcAttr.HideWindow）

### Core Update

- [ ] **UPDT-01**: 启动时检查 uv 是否安装
- [ ] **UPDT-02**: uv 未安装时记录错误日志并退出
- [ ] **UPDT-03**: 使用 uv 安装 nanobot GitHub main 分支最新代码
- [ ] **UPDT-04**: 更新失败时回退使用 uv tool install nanobot-ai 安装稳定版
- [ ] **UPDT-05**: 记录更新过程的详细日志

### Scheduling

- [ ] **SCHD-01**: 支持 cron 表达式定时触发更新
- [ ] **SCHD-02**: 默认 cron 为 "0 3 * * *"（每天凌晨 3 点）
- [ ] **SCHD-03**: 防止任务重叠执行（SkipIfStillRunning 模式）

### Notifications

- [ ] **NOTF-01**: 从环境变量读取 Pushover 配置 (PUSHOVER_TOKEN, PUSHOVER_USER)
- [ ] **NOTF-02**: 更新失败时通过 Pushover 发送通知
- [ ] **NOTF-03**: 通知包含失败原因
- [ ] **NOTF-04**: Pushover 配置缺失时仅记录警告日志，不阻止程序运行

### Runtime

- [ ] **RUN-01**: 支持 Windows 后台运行，隐藏控制台窗口
- [ ] **RUN-02**: 程序手动启动，非开机自启动

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Features

- **ADVT-01**: 维护窗口 - 仅在指定时间段内更新
- **ADVT-02**: 版本锁定 - 更新到特定版本
- **ADVT-03**: 健康检查端点 - HTTP 健康检查接口
- **ADVT-04**: 更新验证 - 校验和/签名验证

### Retry Mechanism

- **RETRY-01**: 失败重试机制 - 指数退避重试
- **RETRY-02**: 最大重试次数配置

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| GUI 界面 | 命令行工具，无需图形界面 |
| 更新历史记录 | 保持简单，不存储历史 |
| 开机自启动 | 用户手动启动 |
| 跨平台支持 | 仅支持 Windows |
| Linux/macOS 支持 | 用户仅需 Windows |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFR-01 | Phase 1 | Pending |
| INFR-02 | Phase 1 | Pending |
| INFR-03 | Phase 1 | Pending |
| INFR-04 | Phase 1 | Pending |
| INFR-05 | Phase 1 | Pending |
| INFR-06 | Phase 1 | Pending |
| INFR-07 | Phase 1 | Pending |
| INFR-08 | Phase 1 | Pending |
| INFR-09 | Phase 1 | Pending |
| INFR-10 | Phase 1 | Pending |
| UPDT-01 | Phase 2 | Pending |
| UPDT-02 | Phase 2 | Pending |
| UPDT-03 | Phase 2 | Pending |
| UPDT-04 | Phase 2 | Pending |
| UPDT-05 | Phase 2 | Pending |
| SCHD-01 | Phase 3 | Pending |
| SCHD-02 | Phase 3 | Pending |
| SCHD-03 | Phase 3 | Pending |
| NOTF-01 | Phase 3 | Pending |
| NOTF-02 | Phase 3 | Pending |
| NOTF-03 | Phase 3 | Pending |
| NOTF-04 | Phase 3 | Pending |
| RUN-01 | Phase 4 | Pending |
| RUN-02 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0 ✓

---
*Requirements defined: 2025-02-18*
*Last updated: 2025-02-18 after initial definition*
