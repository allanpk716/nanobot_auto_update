# Roadmap: Nanobot Auto Updater

## Milestones

- **v1.0 Single Instance Auto-Update** - Phases 01-04 (shipped 2026-02-18)
- **v0.2 Multi-Instance Support** - Phases 05-18 (shipped 2026-03-16)
- **v0.4 Real-time Log Viewing** - Phases 19-23 (shipped 2026-03-20)
- **v0.5 Core Monitoring and Automation** - Phases 24-29 (shipped 2026-03-24)
- **v0.6 Update Log Recording and Query System** - Phases 30-33 (shipped 2026-03-29)
- **v0.7 Update Lifecycle Notifications** - Phases 34-35 (shipped 2026-03-29)
- **v0.8 Self-Update** - Phases 36-40 (shipped 2026-03-30)
- **v0.9 Startup Notification & Telegram Monitor** - Phases 41-43 (shipped 2026-04-06)
- **v0.10 管理界面自更新功能** - Phases 44-45 (active)

## Phases

<details>
<summary>v1.0 Single Instance Auto-Update (Phases 01-04) - SHIPPED 2026-02-18</summary>

基础自动更新功能已交付。

</details>

<details>
<summary>v0.2 Multi-Instance Support (Phases 05-18) - SHIPPED 2026-03-16</summary>

多实例管理功能已交付。

</details>

<details>
<summary>v0.4 Real-time Log Viewing (Phases 19-23) - SHIPPED 2026-03-20</summary>

实时日志查看功能已交付。

</details>

<details>
<summary>v0.5 Core Monitoring and Automation (Phases 24-29) - SHIPPED 2026-03-24</summary>

核心监控和自动化功能已交付。

</details>

<details>
<summary>v0.6 Update Log Recording and Query System (Phases 30-33) - SHIPPED 2026-03-29</summary>

- [x] Phase 30: Log Structure and Recording (2/2 plans) - completed 2026-03-27
- [x] Phase 31: File Persistence (2/2 plans) - completed 2026-03-28
- [x] Phase 32: Query API (2/2 plans) - completed 2026-03-29
- [x] Phase 33: Integration and Testing (2/2 plans) - completed 2026-03-29

</details>

<details>
<summary>v0.7 Update Lifecycle Notifications (Phases 34-35) - SHIPPED 2026-03-29</summary>

- [x] Phase 34: Update Notification Integration (1/1 plan) - completed 2026-03-29
- [x] Phase 35: Notification Integration Testing (1/1 plan) - completed 2026-03-29

</details>

<details>
<summary>v0.8 Self-Update (Phases 36-40) - SHIPPED 2026-03-30</summary>

- [x] Phase 36: PoC Validation (1/1 plan) - completed 2026-03-29
- [x] Phase 37: CI/CD Pipeline (1/1 plan) - completed 2026-03-29
- [x] Phase 38: Self-Update Core (2/2 plans) - completed 2026-03-30
- [x] Phase 39: HTTP API Integration (2/2 plans) - completed 2026-03-30
- [x] Phase 40: Safety & Recovery (2/2 plans) - completed 2026-03-30

</details>

<details>
<summary>v0.9 Startup Notification & Telegram Monitor (Phases 41-43) - SHIPPED 2026-04-06</summary>

- [x] Phase 41: Startup Notification (2/2 plans) - completed 2026-04-06
- [x] Phase 42: Telegram Monitor Core (2/2 plans) - completed 2026-04-06
- [x] Phase 43: Telegram Monitor Integration (2/2 plans) - completed 2026-04-06

</details>

<details>
<summary>v0.10 管理界面自更新功能 (Phases 44-45) - ACTIVE</summary>

### Phase 44: 后端 — 自更新进度追踪 + Web Token API

**Goal:** 增强 selfupdate 包支持下载进度追踪，新增 Web UI 配置端点供前端获取认证 Token。

**Requirements:** API-01, API-02

**Tasks:**
1. selfupdate 包新增 `ProgressState` 结构（stage + download_percent + error）
2. 下载函数添加 `io.TeeReader` + `Content-Length` 进度追踪
3. `atomic.Value` 存储进度状态，`SetProgress`/`GetProgress` 方法
4. `GET /api/v1/self-update/check` 响应新增 `progress` 字段
5. `GET /api/v1/web-config` 端点（localhost-only，返回 auth_token）
6. 单元测试：进度状态并发安全、下载百分比计算、localhost 限制

**Plans:**
- [ ] 44-01-PLAN.md — selfupdate progress tracking (ProgressState + downloadWithProgress + atomic.Value)
- [ ] 44-02-PLAN.md — web-config API + check progress enhancement (localhost-only endpoint + tests)

---

### Phase 45: 前端 — 自更新管理 UI

**Goal:** home.html 顶部新增自更新管理区域，包含版本显示、检测更新、触发更新与进度展示。

**Requirements:** UI-01, UI-02, UI-03, UI-04, UI-05

**Tasks:**
1. home.html 顶部新增自更新管理区块（当前版本 + 操作按钮）
2. "检测更新"按钮调用 check API，展示版本详情（版本号、日期、说明）
3. "立即更新"按钮调用 update API，轮询显示进度（阶段 + 下载百分比）
4. 进度条组件（CSS 进度条 + 百分比文字）
5. 页面加载时自动获取 `/api/v1/web-config` token，存储后用于 API 调用
6. 更新状态 UI 交互（进行中禁用按钮、完成/失败提示）

**Plans:** 2 plans (45-01: HTML/CSS 自更新区块 + 版本显示, 45-02: 更新交互逻辑 + 进度轮询)

**Depends on:** Phase 44

</details>

---
*Last updated: 2026-04-07 (Phase 44 planned)*
