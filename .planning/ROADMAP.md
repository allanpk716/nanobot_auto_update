# Roadmap: Nanobot Auto Updater

## Milestones

- **v1.0 Single Instance Auto-Update** - Phases 01-04 (shipped 2026-02-18)
- **v0.2 Multi-Instance Support** - Phases 05-18 (shipped 2026-03-16)
- **v0.4 Real-time Log Viewing** - Phases 19-23 (shipped 2026-03-20)
- **v0.5 Core Monitoring and Automation** - Phases 24-29 (shipped 2026-03-24)
- **v0.6 Update Log Recording and Query System** - Phases 30-33 (shipped 2026-03-29)
- **v0.7 Update Lifecycle Notifications** - Phases 34-35 (shipped 2026-03-29)
- [IN PROGRESS] **v0.8 Self-Update** - Phases 36-40 (in progress)

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

- [x] Phase 30: Log Structure and Recording (2/2 plans) — completed 2026-03-27
- [x] Phase 31: File Persistence (2/2 plans) — completed 2026-03-28
- [x] Phase 32: Query API (2/2 plans) — completed 2026-03-29
- [x] Phase 33: Integration and Testing (2/2 plans) — completed 2026-03-29

</details>

<details>
<summary>v0.7 Update Lifecycle Notifications (Phases 34-35) - SHIPPED 2026-03-29</summary>

- [x] Phase 34: Update Notification Integration (1/1 plan) — completed 2026-03-29
- [x] Phase 35: Notification Integration Testing (1/1 plan) — completed 2026-03-29

</details>

### [IN PROGRESS] v0.8 Self-Update (In Progress)

**Milestone Goal:** 让 nanobot-auto-updater 程序自身支持通过 GitHub Releases 自动检测、下载并替换更新

- [x] **Phase 36: PoC Validation** - 独立测试程序验证 minio/selfupdate 在 Windows 上的可行性 (completed 2026-03-29)
- [x] **Phase 37: CI/CD Pipeline** - GoReleaser + GitHub Actions 自动构建发布 (completed 2026-03-29)
- [x] **Phase 38: Self-Update Core** - internal/selfupdate/ 包实现自更新核心逻辑 (completed 2026-03-30)
- [x] **Phase 39: HTTP API Integration** - 自更新 API 端点 + Help 接口更新 (completed 2026-03-30)
- [ ] **Phase 40: Safety & Recovery** - 重启机制、通知、备份清理与验证

## Phase Details

### Phase 36: PoC Validation
**Goal**: 用户确认 minio/selfupdate 在 Windows 上能完成 exe 替换、备份和重启，消除技术不确定性
**Depends on**: Nothing (first phase of milestone)
**Requirements**: VALID-01, VALID-02, VALID-03
**Success Criteria** (what must be TRUE):
  1. 独立 PoC 程序可以替换自身正在运行的 exe 文件为新版本（新版本启动后输出新版本号）
  2. 替换后旧的 exe 被保存为 .old 文件，用户可在文件系统中看到该备份
  3. PoC 程序替换完成后能 self-spawn 重启新版本进程并正常退出旧进程
**Plans**: 1 plan

Plans:
- [x] 36-01-PLAN.md — Create PoC program + automated test validating exe replacement, backup, and self-spawn restart

### Phase 37: CI/CD Pipeline
**Goal**: 用户推送 v* tag 后自动构建 Windows amd64 二进制并发布到 GitHub Releases，后续阶段有 Release 可下载
**Depends on**: Nothing (独立于 Go 代码变更)
**Requirements**: CICD-01, CICD-02, CICD-03
**Success Criteria** (what must be TRUE):
  1. 推送 v* tag 后 GitHub Actions 自动触发构建流程
  2. GoReleaser 编译出 Windows amd64 二进制并发布到 GitHub Releases 页面（含 checksums）
  3. 编译产物通过 ldflags 注入了版本号，运行 -version 可看到正确的版本信息
**Plans**: 1 plan

Plans:
- [x] 37-01-PLAN.md — Create .goreleaser.yaml and .github/workflows/release.yml for automated Windows binary releases

### Phase 38: Self-Update Core
**Goal**: internal/selfupdate/ 包能独立检查 GitHub 最新 Release、比较版本、下载并安全替换运行中的 exe
**Depends on**: Phase 36 (PoC 验证通过), Phase 37 (有 Release 可下载)
**Requirements**: UPDATE-01, UPDATE-02, UPDATE-03, UPDATE-04, UPDATE-05, UPDATE-06, UPDATE-07
**Success Criteria** (what must be TRUE):
  1. 调用 CheckLatest() 可获取 GitHub 最新 Release 的版本号和下载 URL
  2. 当前版本与最新版本进行 semver 比较，正确识别是否需要更新（dev 版本视为需要更新）
  3. 下载的二进制通过 SHA256 校验验证完整性
  4. 调用更新方法后运行中的 exe 被安全替换，旧 exe 被保存为 .old 备份
  5. Release 信息被缓存 1 小时，缓存有效期内不重复请求 GitHub API
**Plans**: 2 plans

Plans:
- [x] 38-01-PLAN.md — Create selfupdate package: types, Updater struct, CheckLatest, NeedUpdate, cache, unit tests
- [x] 38-02-PLAN.md — Add Update method (download, checksum, ZIP extract, Apply) + config self_update section

### Phase 39: HTTP API Integration
**Goal**: 用户可通过 HTTP API 检查自更新版本和触发自更新，Help 接口提供自更新端点说明
**Depends on**: Phase 38 (Self-Update Core 组件就绪)
**Requirements**: API-01, API-02, API-03, API-04
**Success Criteria** (what must be TRUE):
  1. POST /api/v1/self-update 端点需要 Bearer Token 认证，认证失败返回 401
  2. 自更新与 nanobot 更新互斥，并发请求返回 409 Conflict
  3. GET /api/v1/self-update/check 只读检查最新版本，返回当前版本和最新版本信息
  4. Help 接口包含自更新相关端点的使用说明
**Plans**: 2 plans

Plans:
- [x] 39-01-PLAN.md — SelfUpdateHandler (interfaces, HandleCheck, HandleUpdate) + InstanceManager mutex methods + Server routes + main.go integration
- [x] 39-02-PLAN.md — Help endpoint self_update_check and self_update entries + test

### Phase 40: Safety & Recovery
**Goal**: 更新后程序能自动重启，用户收到通知，异常情况下能自动恢复旧版本
**Depends on**: Phase 39 (HTTP API 已集成自更新)
**Requirements**: SAFE-01, SAFE-02, SAFE-03, SAFE-04
**Success Criteria** (what must be TRUE):
  1. 自更新完成后程序自动重启新版本，新版本成功绑定原有端口
  2. 自更新开始和完成（成功或失败）时发送 Pushover 通知
  3. 程序启动时清理上一次更新留下的 .old 备份文件
  4. 程序启动时检测异常的 .exe.old 文件存在，自动恢复旧版本确保系统可用
**Plans**: 2 plans

Plans:
- [x] 40-01-PLAN.md — Notifier injection + start/complete notifications + status file + self-spawn restart in SelfUpdateHandler
- [ ] 40-02-PLAN.md — Startup .old cleanup/recovery + port binding retry in Server.Start()

## Progress

**Execution Order:**
Phases execute in numeric order: 36 → 37 → 38 → 39 → 40

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 36. PoC Validation | 1/1 | Complete    | 2026-03-29 |
| 37. CI/CD Pipeline | 1/1 | Complete    | 2026-03-29 |
| 38. Self-Update Core | 2/2 | Complete    | 2026-03-30 |
| 39. HTTP API Integration | 2/2 | Complete    | 2026-03-30 |
| 40. Safety & Recovery | 1/2 | In Progress|  |

---

*Last updated: 2026-03-30 (Phase 40 Plan 01 complete)*
