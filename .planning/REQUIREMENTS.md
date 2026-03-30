# Requirements: Nanobot Auto Updater v0.8 Self-Update

**Defined:** 2026-03-29
**Core Value:** 自动保持 nanobot 处于最新版本,无需用户手动干预。

## v1 Requirements

Requirements for v0.8 Self-Update milestone. Each maps to roadmap phases.

### VALIDATION (可行性验证)

- [x] **VALID-01**: 创建独立 PoC 测试程序，验证 minio/selfupdate 在 Windows 上的 exe 替换可行性
- [x] **VALID-02**: 验证备份机制（.old 文件创建）和回滚功能正常工作
- [x] **VALID-03**: 验证 self-spawn 重启机制（更新后自动重启新版本进程）

### CI/CD (构建发布)

- [x] **CICD-01**: GitHub Actions workflow 在 v* tag 推送时自动触发构建
- [x] **CICD-02**: GoReleaser 构建 Windows amd64 二进制并发布到 GitHub Releases
- [x] **CICD-03**: 通过 ldflags 注入版本号到编译产物（-X main.Version）

### UPDATE (自更新核心)

- [x] **UPDATE-01**: GitHub Release API 检查最新版本（GET /repos/{owner}/{repo}/releases/latest）
- [x] **UPDATE-02**: Semver 版本比较（当前版本 vs 最新 Release tag）
- [x] **UPDATE-03**: SHA256 校验验证下载的二进制完整性
- [x] **UPDATE-04**: minio/selfupdate 安全替换运行中 exe（Windows rename trick）
- [x] **UPDATE-05**: 备份当前 exe（Options.OldSavePath 保存 .old 文件）
- [x] **UPDATE-06**: Release 信息缓存（TTL 1 小时，避免 GitHub API 限速 60次/时）
- [x] **UPDATE-07**: 配置文件新增 self_update 配置节（github_owner, github_repo）

### API (HTTP 接口)

- [x] **API-01**: POST /api/v1/self-update 端点（Bearer Token 认证，复用 Phase 28）
- [x] **API-02**: 并发更新保护（复用 atomic.Bool，与 nanobot 更新互斥，返回 409 Conflict）
- [x] **API-03**: GET /api/v1/self-update/check 端点（只读版本检查，不触发更新）
- [ ] **API-04**: Help 接口更新（新增自更新端点使用说明）

### SAFETY (安全与恢复)

- [ ] **SAFE-01**: 更新后自动重启（self-spawn + graceful shutdown + 端口重绑重试）
- [ ] **SAFE-02**: Pushover 通知（自更新开始/完成/失败通知）
- [ ] **SAFE-03**: .old 文件清理（启动时检查并清理旧备份文件）
- [ ] **SAFE-04**: 启动时备份验证（检测 .exe.old 异常文件存在，自动恢复旧版本）

## v2 Requirements

Deferred to future milestone. Not in current roadmap.

### Advanced Self-Update

- **ADV-01**: 定时版本检查（复用 cron 调度器，检测到新版本只通知不自动更新）
- **ADV-02**: Rollback API 端点（POST /api/v1/self-update/rollback）
- **ADV-03**: Pre-release channel 支持（下载 pre-release 版本）
- **ADV-04**: 下载进度 SSE 推送
- **ADV-05**: Linux 平台支持

## Out of Scope

| Feature | Reason |
|---------|--------|
| creativeprojects/go-selfupdate 库 | 依赖过重（go-github + gitea + gitlab），功能过剩 |
| google/go-github 库 | 单个 API 调用不需要完整 SDK |
| 代码签名 | SmartScreen 拦截是长期考虑，非 MVP |
| 多平台构建 | 仅 Windows amd64，按用户明确要求 |
| 自动定时自更新 | 仅 HTTP API 触发，按用户明确要求 |

## Traceability

Confirmed by roadmapper. All 21 v1 requirements mapped to 5 phases (36-40).

| Requirement | Phase | Status |
|-------------|-------|--------|
| VALID-01 | Phase 36 | Complete |
| VALID-02 | Phase 36 | Complete |
| VALID-03 | Phase 36 | Complete |
| CICD-01 | Phase 37 | Complete |
| CICD-02 | Phase 37 | Complete |
| CICD-03 | Phase 37 | Complete |
| UPDATE-01 | Phase 38 | Complete |
| UPDATE-02 | Phase 38 | Complete |
| UPDATE-03 | Phase 38 | Complete |
| UPDATE-04 | Phase 38 | Complete |
| UPDATE-05 | Phase 38 | Complete |
| UPDATE-06 | Phase 38 | Complete |
| UPDATE-07 | Phase 38 | Complete |
| API-01 | Phase 39 | Complete |
| API-02 | Phase 39 | Complete |
| API-03 | Phase 39 | Complete |
| API-04 | Phase 39 | Pending |
| SAFE-01 | Phase 40 | Pending |
| SAFE-02 | Phase 40 | Pending |
| SAFE-03 | Phase 40 | Pending |
| SAFE-04 | Phase 40 | Pending |

**Coverage:**
- v1 requirements: 21 total
- Mapped to phases: 21/21 (confirmed)
- Unmapped: 0

---
*Requirements defined: 2026-03-29*
*Last updated: 2026-03-29 (traceability confirmed by roadmapper)*
