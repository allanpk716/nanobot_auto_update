# Phase 36: PoC Validation - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

创建独立最小 PoC 程序，验证 `minio/selfupdate v0.6.0` 在 Windows 上完成运行中 exe 替换、备份和重启的可行性。消除技术不确定性，为后续阶段铺路。

PoC 不涉及：GitHub API 交互、CI/CD、HTTP API、配置集成。仅验证核心二进制替换能力。

</domain>

<decisions>
## Implementation Decisions

### PoC 程序范围
- **D-01:** 最小独立程序 — 一个 main.go（~50-80 行），仅包含版本打印 → selfupdate.Apply() → self-spawn 重启 → 新版本打印。不模拟项目结构，不引入 logger/config/HTTP server。
- **D-02:** 新版本二进制来源 — 本地构建 v1 和 v2 两个 exe。v1 启动后读取本地 v2 的二进制文件调用 selfupdate.Apply()，无需网络。

### 验证和测试
- **D-03:** 自动化测试脚本 — Go 测试程序：构建两个版本 → 运行 v1 → 等待重启 → 通过文件输出验证 v2 成功启动。
- **D-04:** 新版本检测方式 — 文件输出验证：PoC 程序将版本号写入文件，测试脚本读取文件确认新版本号。

### 代码保留
- **D-05:** PoC 代码保留在 `tmp/` 目录作为参考。正式实现（Phase 38）独立编写 `internal/selfupdate/` 包，但可回头参考 PoC 实现细节。

### Claude's Discretion
- PoC 程序的具体实现细节（如何构建两个版本、文件路径约定、等待超时等）
- 自动化测试脚本的结构和错误处理

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### 核心库文档
- `.planning/research/STACK.md` — 库选择决策（minio/selfupdate vs creativeprojects/go-selfupdate），Windows exe rename trick 详解，依赖树分析
- `.planning/research/ARCHITECTURE.md` — 架构模式参考（注意：该文档使用 creativeprojects/go-selfupdate，但实际已决定使用 minio/selfupdate，架构设计思路仍有参考价值）

### 项目规范
- `CLAUDE.md` — 项目规则：临时文件放 tmp/ 目录，使用中文回答

### 需求追踪
- `.planning/REQUIREMENTS.md` — VALID-01, VALID-02, VALID-03 需求定义
- `.planning/ROADMAP.md` — Phase 36 成功标准（3 条）

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/nanobot-auto-updater/main.go:28` — `var Version = "dev"` 通过 ldflags 注入版本号，PoC 可复用此模式区分 v1/v2
- `cmd/nanobot-auto-updater/main.go:27-40` — `--version` flag 处理模式

### Established Patterns
- 版本注入: `main.Version` 通过 `-ldflags "-X main.Version=xxx"` 构建时注入
- Windows 隐藏窗口: `windows.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}` 在 `internal/updater/updater.go` 中已有使用
- 项目使用 Go 1.24.11

### Integration Points
- `go.mod` — 当前无 selfupdate 依赖，PoC 需要引入 `github.com/minio/selfupdate v0.6.0`
- `tmp/` — 项目约定临时测试代码存放位置

</code_context>

<specifics>
## Specific Ideas

- PoC 程序验证流程：构建 v1 (Version="1.0.0") 和 v2 (Version="2.0.0") → 运行 v1 → v1 调用 selfupdate.Apply(v2 binary) → v1 self-spawn v2 → v2 启动写入版本号到文件 → 测试脚本验证文件内容
- 成功标准直接来自 ROADMAP.md：(1) exe 替换成功且新版本输出新版本号 (2) .old 备份文件可见 (3) self-spawn 重启正常

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 36-poc-validation*
*Context gathered: 2026-03-29*
