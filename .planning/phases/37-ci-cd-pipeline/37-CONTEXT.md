# Phase 37: CI/CD Pipeline - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

创建 GoReleaser 配置文件和 GitHub Actions workflow，实现推送 v* tag 后自动构建 Windows amd64 二进制并发布到 GitHub Releases（含 checksums 和 ldflags 版本注入）。

本阶段不涉及：Go 代码变更（自更新逻辑属于 Phase 38）、API 端点（Phase 39）、安全恢复机制（Phase 40）。

</domain>

<decisions>
## Implementation Decisions

### Release 产物格式
- **D-01:** ZIP 压缩包发布 — GoReleaser 默认为 Windows 生成 ZIP 格式。Phase 38 自更新代码需要下载 ZIP 并解压提取 exe 后调用 selfupdate.Apply()。ZIP 内可附带 README 等额外文件。

### 构建类型
- **D-02:** 仅 GUI 版本 — Release 只包含 `-H=windowsgui` 构建的 exe（无控制台窗口），与当前 Makefile `build-release` 目标一致。不发布 console 调试版本。

### CI/CD 流水线组织
- **D-03:** GoReleaser 管一切 — 单一 GoReleaser action 管理构建和发布流程。GoReleaser 自带 go test 能力，无需独立测试 job。保持工作流简洁。

### Claude's Discretion
- GoReleaser 配置细节（archive name template, checksum 算法等）
- GitHub Actions workflow 具体结构（runner 版本、Go 版本等）
- Release name 和 description 模板

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 项目构建配置
- `Makefile` — 现有构建逻辑：LDFLAGS_RELEASE 模式 (`-H=windowsgui -X main.Version=$(VERSION)`)
- `go.mod` — Go 1.24.11, module path: `github.com/HQGroup/nanobot-auto-updater`
- `cmd/nanobot-auto-updater/main.go:28` — `var Version = "dev"` ldflags 注入点

### 需求追踪
- `.planning/REQUIREMENTS.md` — CICD-01, CICD-02, CICD-03 需求定义
- `.planning/ROADMAP.md` — Phase 37 成功标准（3 条）

### 项目规范
- `CLAUDE.md` — 项目规则：使用中文回答

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile:10` — LDFLAGS_RELEASE 已定义 `-H=windowsgui -X main.Version=$(VERSION)`，GoReleaser ldflags 需复用此模式
- `cmd/nanobot-auto-updater/main.go:28` — `var Version = "dev"` 已就绪，GoReleaser 通过 ldflags 覆盖

### Established Patterns
- 版本注入: `main.Version` 通过 `-ldflags "-X main.Version=xxx"` 构建时注入
- GUI 构建: `-H=windowsgui` 隐藏控制台窗口
- 构建入口: `./cmd/nanobot-auto-updater`

### Integration Points
- `.github/workflows/` — 需新建目录和 workflow 文件
- `.goreleaser.yml` (或 `.goreleaser.yaml`) — 需新建 GoReleaser 配置文件
- Phase 38 (Self-Update Core) — 将消费本阶段产出的 GitHub Release，下载 ZIP 解压提取 exe

</code_context>

<specifics>
## Specific Ideas

- GoReleaser ldflags 配置应与 Makefile LDFLAGS_RELEASE 一致：`-H=windowsgui -X main.Version={{.Version}}`
- GoReleaser archive format: zip (Windows default)
- Binary name: `nanobot-auto-updater.exe`
- GitHub Actions trigger: `on: push: tags: - 'v*'`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 37-ci-cd-pipeline*
*Context gathered: 2026-03-29*
