# Phase 38: Self-Update Core - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

创建 `internal/selfupdate/` 包，实现 GitHub Release 版本检查、semver 比较、ZIP 下载解压、SHA256 校验、exe 安全替换的完整自更新核心逻辑。本阶段产出独立可测试的 library，不涉及 HTTP API 端点（Phase 39）和安全恢复机制（Phase 40）。

本阶段不涉及：HTTP API handler（Phase 39）、重启/通知/清理（Phase 40）、CI/CD 配置（Phase 37 已完成）。

</domain>

<decisions>
## Implementation Decisions

### 下载与解压流程
- **D-01:** 内存解压 — 下载 ZIP 到内存（`bytes.Buffer`），用 `archive/zip` 解压提取 exe 到 `bytes.Buffer`，直接传 `io.Reader` 给 `selfupdate.Apply()`。不落盘，无临时文件清理问题。

### SHA256 校验方式
- **D-02:** checksums.txt 双重校验 — 先下载 GoReleaser 生成的 `checksums.txt`，解析出 ZIP 文件对应的 SHA256 hash，计算实际下载 ZIP 的 SHA256 并比对。校验通过后再解压提取 exe。这提供了 ZIP 传输完整性验证。

### 包公共 API 设计
- **D-03:** Updater struct + 方法模式。公共 API：
  - `NewUpdater(cfg SelfUpdateConfig) *Updater` — 构造函数，接收配置
  - `CheckLatest() (*ReleaseInfo, error)` — 检查 GitHub 最新 Release（带缓存）
  - `NeedUpdate(currentVersion string) (bool, *ReleaseInfo, error)` — semver 比较，dev 版本视为需要更新
  - `Update(currentVersion string) error` — 完整更新流程：下载 → 校验 → 解压 → Apply
  - 缓存和 `http.Client` 封装在 struct 内部

### 配置节设计
- **D-04:** 最小配置 — `config.yaml` 新增 `self_update` 配置节，仅包含：
  - `github_owner` (string) — GitHub 仓库 owner（如 "HQGroup"）
  - `github_repo` (string) — GitHub 仓库名（如 "nanobot-auto-updater"）
  - 其他参数硬编码为包常量：缓存 TTL=1h、HTTP timeout=30s、User-Agent 等

### Claude's Discretion
- semver 解析实现（标准库或简单字符串比较）
- 缓存具体实现方式（struct 字段 + time.Time）
- GitHub API 错误处理和重试策略
- 具体文件拆分（是否分 checker.go、downloader.go 等）
- 测试策略和 mock 方式

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 核心库和 PoC 参考
- `tmp/poc_selfupdate.go` — Phase 36 PoC 代码，minio/selfupdate.Apply() 使用模式
- `tmp/poc_selfupdate_test.go` — PoC 测试代码，自动化验证模式
- `.planning/research/STACK.md` — 库选择决策、Windows exe rename trick、依赖树分析
- `.planning/research/ARCHITECTURE.md` — 架构模式参考（注意：使用 creativeprojects/go-selfupdate，实际已决定使用 minio/selfupdate，架构思路有参考价值）

### Phase 37 产物（消费方）
- `.goreleaser.yaml` — Release 产物命名模板（`nanobot-auto-updater_{version}_windows_amd64.zip`）、checksums.txt 生成配置
- `.github/workflows/release.yml` — Release workflow 触发条件

### 项目集成点
- `cmd/nanobot-auto-updater/main.go:28` — `var Version = "dev"` ldflags 注入点，用于 semver 比较
- `internal/config/config.go` — Config struct，需新增 `SelfUpdate SelfUpdateConfig` 字段
- `go.mod` — 当前依赖基线，需新增 `github.com/minio/selfupdate v0.6.0`

### 需求追踪
- `.planning/REQUIREMENTS.md` — UPDATE-01 至 UPDATE-07 需求定义
- `.planning/ROADMAP.md` — Phase 38 成功标准（5 条）

### 项目规范
- `CLAUDE.md` — 项目规则：使用中文回答，临时文件放 tmp/

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `tmp/poc_selfupdate.go` — 已验证的 minio/selfupdate.Apply() 调用模式（OldSavePath、错误处理、rollback 检测）
- `cmd/nanobot-auto-updater/main.go:28` — `var Version = "dev"` 已就绪，Update() 需要此值做 semver 比较
- `internal/config/config.go` — Config struct + viper 模式，SelfUpdateConfig 可复用此模式

### Established Patterns
- 版本注入: `main.Version` 通过 ldflags `-X main.Version=xxx` 构建时注入
- 配置节扩展: Config struct 新增字段 + defaults() + viper 自动绑定
- 包组织: `internal/` 下各功能独立包（api、config、instance、lifecycle 等）
- 日志: `github.com/WQGroup/logger` 统一日志库
- 测试: `github.com/stretchr/testify` 断言库

### Integration Points
- `internal/config/config.go` — 新增 `SelfUpdate SelfUpdateConfig` 字段和 defaults
- `cmd/nanobot-auto-updater/main.go` — main.Version 传入 selfupdate.Updater
- Phase 39 (HTTP API Integration) — 将消费 Updater struct 的方法构建 handler
- Phase 40 (Safety & Recovery) — 将消费 .old 备份文件做清理和恢复

</code_context>

<specifics>
## Specific Ideas

- Release 产物命名约定: `nanobot-auto-updater_{version}_windows_amd64.zip`（来自 .goreleaser.yaml）
- checksums.txt 文件命名约定: `nanobot-auto-updater_{version}_checksums.txt`
- GitHub API endpoint: `GET /repos/{owner}/{repo}/releases/latest`
- 缓存实现: struct 内 `cachedRelease *ReleaseInfo` + `cacheTime time.Time`，每次 CheckLatest() 检查 time.Since(cacheTime) < 1h
- dev 版本检测: currentVersion == "dev" 时 NeedUpdate() 返回 true（REQUIREMENTS UPDATE-02）

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 38-self-update-core*
*Context gathered: 2026-03-29*
