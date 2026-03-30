# Phase 37: CI/CD Pipeline - Research

**Researched:** 2026-03-29
**Domain:** GoReleaser + GitHub Actions CI/CD for Go Windows binary releases
**Confidence:** HIGH

## Summary

本阶段需要创建两个配置文件：GoReleaser 配置文件 (`.goreleaser.yaml`) 和 GitHub Actions workflow 文件 (`.github/workflows/release.yml`)，实现推送 v* tag 后自动构建 Windows amd64 GUI 版本的二进制并发布到 GitHub Releases。项目已有成熟的 Makefile 构建逻辑和 ldflags 版本注入模式，GoReleaser 配置需要精确复现这些行为。

GoReleaser v2 是当前主流版本（v2.14 是 2026 年最新），配合官方 `goreleaser-action@v7`（浮动主版本标签）使用。关键注意事项：GoReleaser 默认 ldflags 注入的变量名是小写 `main.version`，但项目使用大写 `main.Version`，必须自定义 ldflags 覆盖默认值。同时需要保留 `-H=windowsgui` 链接器标志以匹配现有 Makefile 行为。

**Primary recommendation:** 使用 GoReleaser v2 + goreleaser-action@v7 的标准组合，严格按照官方文档模板创建 workflow，自定义 ldflags 匹配 Makefile 的 `LDFLAGS_RELEASE`。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** ZIP 压缩包发布 -- GoReleaser 默认为 Windows 生成 ZIP 格式。Phase 38 自更新代码需要下载 ZIP 并解压提取 exe 后调用 selfupdate.Apply()。ZIP 内可附带 README 等额外文件。
- **D-02:** 仅 GUI 版本 -- Release 只包含 `-H=windowsgui` 构建的 exe（无控制台窗口），与当前 Makefile `build-release` 目标一致。不发布 console 调试版本。
- **D-03:** GoReleaser 管一切 -- 单一 GoReleaser action 管理构建和发布流程。GoReleaser 自带 go test 能力，无需独立测试 job。保持工作流简洁。

### Claude's Discretion
- GoReleaser 配置细节（archive name template, checksum 算法等）
- GitHub Actions workflow 具体结构（runner 版本、Go 版本等）
- Release name 和 description 模板

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CICD-01 | GitHub Actions workflow 在 v* tag 推送时自动触发构建 | GitHub Actions workflow 配置: `on: push: tags: ['v*']`，官方模板已验证 |
| CICD-02 | GoReleaser 构建 Windows amd64 二进制并发布到 GitHub Releases | GoReleaser builds 配置: `goos: [windows]`, `goarch: [amd64]`，archives format: zip，release 自动发布 |
| CICD-03 | 通过 ldflags 注入版本号到编译产物（-X main.Version） | GoReleaser ldflags 自定义覆盖: `-H=windowsgui -X main.Version={{.Version}}`，匹配 Makefile LDFLAGS_RELEASE |
</phase_requirements>

## Standard Stack

### Core
| Library/Tool | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| GoReleaser CLI | v2.14 | 构建、打包、发布自动化 | Go 项目发布的事实标准，官方维护 |
| goreleaser-action | @v7 (浮动主版本标签) | 在 GitHub Actions 中运行 GoReleaser | 官方 GitHub Action，自动安装 GoReleaser CLI |
| actions/checkout | @v4 | 检出仓库代码 | GitHub 官方，最新稳定主版本 |
| actions/setup-go | @v5 | 安装 Go 工具链 | GitHub 官方，缓存默认开启，支持 `go-version: stable` |

### Supporting
| Library/Tool | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| GitHub GITHUB_TOKEN | 自动提供 | GoReleaser 发布到 Releases 的认证 | 必需，无需额外配置，`permissions: contents: write` 即可 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| goreleaser-action@v7 | goreleaser-action@v6.4.0 (精确版本) | v7 是浮动标签跟踪最新 v6.x；pin 到 v6.4.0 更可复现但需要手动更新 |

**Version verification:**

| Tool | Verified Version | Source | Date |
|------|-----------------|--------|------|
| GoReleaser CLI | v2.14 | [Official blog](https://goreleaser.com/blog/archive/2026/) | 2026 |
| goreleaser-action | @v7 (docs), v6.4.0 (latest release) | [Official docs](https://goreleaser.com/customization/ci/actions/) (updated 2026-03-22), [GitHub releases](https://github.com/goreleaser/goreleaser-action/releases) | 2026-03-22 |
| actions/checkout | v4 | [GitHub repo](https://github.com/actions/checkout) | Current |
| actions/setup-go | v5 | [GitHub repo](https://github.com/actions/setup-go) | Current |

## Architecture Patterns

### Recommended Project Structure
```
.github/
  workflows/
    release.yml          # GitHub Actions workflow -- tag push 触发构建发布
.goreleaser.yaml         # GoReleaser 配置 -- 构建、打包、checksums
cmd/
  nanobot-auto-updater/
    main.go              # 构建入口，var Version = "dev" (line 28)
Makefile                 # 现有构建逻辑（参考，不修改）
```

### Pattern 1: GitHub Actions Workflow with GoReleaser
**What:** 单一 job workflow，tag push 触发，GoReleaser 完成全部构建发布
**When to use:** 本阶段的唯一 workflow 模式
**Example:**
```yaml
# .github/workflows/release.yml
# Source: https://goreleaser.com/customization/ci/actions/ (updated 2026-03-22)
name: goreleaser

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Pattern 2: GoReleaser Configuration for Single-Platform Windows Build
**What:** GoReleaser yaml 配置，仅构建 Windows amd64，自定义 ldflags 匹配 Makefile
**When to use:** 本项目的 GoReleaser 配置
**Example:**
```yaml
# .goreleaser.yaml
# Source: https://goreleaser.com/customization/builds/ + https://goreleaser.com/customization/archive/

before:
  hooks:
    - go mod tidy

builds:
  - main: ./cmd/nanobot-auto-updater
    binary: nanobot-auto-updater
    goos:
      - windows
    goarch:
      - amd64
    ldflags:
      - -H=windowsgui -X main.Version={{.Version}}
    env:
      - CGO_ENABLED=0

archives:
  - format: zip
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}

checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
  algorithm: sha256
```

### Anti-Patterns to Avoid
- **使用 GoReleaser 默认 ldflags:** 默认注入 `-X main.version={{.Version}}`（小写 v），项目变量是大写 `Version`。必须完全自定义 `ldflags` 列表覆盖默认值，否则版本号不会注入。
- **省略 fetch-depth: 0:** GoReleaser 需要 git 历史生成 changelog 和确定版本号。默认 checkout 只获取单个 commit，会导致 GoReleaser 失败。
- **忘记 permissions: contents: write:** GITHUB_TOKEN 默认权限不足以创建 Release，必须显式声明 `contents: write`。
- **添加多平台构建:** 项目明确只需要 Windows amd64（D-02），添加 linux/darwin 会浪费时间且与决策冲突。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 二进制打包和发布 | 自定义脚本 zip + upload | GoReleaser archives + release | GoReleaser 处理 checksum、命名模板、多文件归档 |
| SHA256 checksums | 手动计算并上传 | GoReleaser checksum section | 默认 sha256，自动生成，Phase 38 的 UPDATE-03 需要此文件 |
| Changelog 生成 | 自定义 git log 脚本 | GoReleaser 默认 changelog | 自动从 git 历史生成，格式标准 |
| 版本号提取 | 自定义 git describe 逻辑 | GoReleaser `{{.Version}}` 模板 | 从 tag 自动提取，与 ldflags 注入一体化 |

**Key insight:** GoReleaser 是 Go 项目 CI/CD 的成熟标准工具。在这个领域手写自定义脚本只会引入 edge case 和维护负担。唯一需要自定义的是 ldflags 和构建矩阵（单平台）。

## Common Pitfalls

### Pitfall 1: ldflags 变量名大小写不匹配
**What goes wrong:** GoReleaser 默认 ldflags 注入 `main.version`（小写 v），项目变量是 `main.Version`（大写 V）。构建成功但版本号始终是 "dev"。
**Why it happens:** Go linker 的 `-X` flag 对变量名大小写敏感。GoReleaser 默认值假设常见的 `version` 小写命名。
**How to avoid:** 显式设置完整 `ldflags` 列表: `-H=windowsgui -X main.Version={{.Version}}`，完全覆盖 GoReleaser 默认值。
**Warning signs:** 构建成功但 `./nanobot-auto-updater --version` 显示 "dev"。

### Pitfall 2: 缺少 fetch-depth: 0
**What goes wrong:** GoReleaser 执行失败，报错无法获取 git 历史或无法确定版本号。
**Why it happens:** `actions/checkout` 默认 `fetch-depth: 1`（浅克隆），GoReleaser 需要完整历史生成 changelog 和计算版本。
**How to avoid:** 在 checkout step 添加 `with: fetch-depth: 0`。
**Warning signs:** CI 报错 `failed to describe tag` 或 changelog 为空。

### Pitfall 3: GITHUB_TOKEN 权限不足
**What goes wrong:** GoReleaser 构建成功但无法创建 GitHub Release，报 403 Forbidden。
**Why it happens:** GitHub Actions 新仓库默认 GITHUB_TOKEN 权限为 `contents: read`（或 `write` 但需要显式声明）。
**How to avoid:** 在 workflow 顶层声明 `permissions: contents: write`。
**Warning signs:** CI 日志显示 GoReleaser 构建和打包成功，但 Release 步骤返回 403。

### Pitfall 4: CGO_ENABLED 未设置
**What goes wrong:** 在 ubuntu runner 上交叉编译 Windows 二进制时可能因 CGO 依赖失败。
**Why it happens:** 默认 CGO_ENABLED 取决于环境，交叉编译 Windows 需要禁用 CGO（除非安装了 Windows 交叉编译工具链）。
**How to avoid:** 在 builds 配置中添加 `env: [CGO_ENABLED=0]`。
**Warning signs:** 构建报错 `cgo: C compiler "gcc" not found` 或类似错误。

### Pitfall 5: GoReleaser version 指定为 "latest"
**What goes wrong:** 使用 `version: "latest"` 可能在 GoReleaser 发布破坏性更新时意外升级。
**Why it happens:** goreleaser-action v6.0.0+ 默认使用 `"~> v2"`（兼容 v2.x 的最新补丁），但旧文档可能建议 `"latest"`。
**How to avoid:** 使用 `version: "~> v2"` 跟踪 v2.x 系列的最新补丁版本，避免意外大版本升级。
**Warning signs:** CI 突然开始失败，但本地 GoReleaser 正常工作。

## Code Examples

### 完整 GitHub Actions Workflow
```yaml
# Source: https://goreleaser.com/customization/ci/actions/ (updated 2026-03-22)
name: goreleaser

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 完整 GoReleaser 配置
```yaml
# .goreleaser.yaml
# Source: GoReleaser official docs - builds, archives, checksum pages

before:
  hooks:
    - go mod tidy

builds:
  - main: ./cmd/nanobot-auto-updater
    binary: nanobot-auto-updater
    goos:
      - windows
    goarch:
      - amd64
    # IMPORTANT: 完全覆盖 GoReleaser 默认 ldflags
    # 默认值使用小写 main.version，项目使用大写 main.Version
    # -H=windowsgui 匹配 Makefile LDFLAGS_RELEASE
    ldflags:
      - -H=windowsgui -X main.Version={{.Version}}
    env:
      - CGO_ENABLED=0

archives:
  - format: zip
    # 产出文件名: nanobot-auto-updater_1.0.0_windows_amd64.zip
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}

checksum:
  # 产出文件名: nanobot-auto-updater_1.0.0_checksums.txt
  name_template: "{{ .ProjectName }}_{{ . Version }}_checksums.txt"
  algorithm: sha256
```

### 版本号注入验证命令
```bash
# 构建后验证版本号是否正确注入
./nanobot-auto-updater --version
# 期望输出: nanobot-auto-updater v1.0.0 (而非 "dev")
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| goreleaser-action@v5 | goreleaser-action@v6+ (默认 ~> v2) | 2024-06 (v6.0.0) | v6.0.0 切换默认 GoReleaser 版本为 ~> v2 |
| goreleaser-action@v6 | goreleaser-action@v7 (浮动标签) | 2026-03 (docs updated) | 官方文档全面使用 @v7，是浮动主版本标签 |
| GoReleaser v1 | GoReleaser v2 | 2024-06 | 移除所有已废弃选项 |
| actions/checkout@v3 | actions/checkout@v4 | 2023-09 | Node 20 runtime |
| actions/setup-go@v4 | actions/setup-go@v5 | 2023-12 | 缓存默认开启 |

**Deprecated/outdated:**
- GoReleaser v1: 已废弃，v2 移除了所有 deprecated 选项
- goreleaser-action `version: "latest"`: v6.0.0 起默认改为 `"~> v2"`，使用 "latest" 不安全
- `brews`, `scoop` 等 announcer: 本项目不使用，无需配置

## Open Questions

1. **goreleaser-action@v7 vs @v6.4.0 精确版本**
   - What we know: 官方文档 (2026-03-22 更新) 全部使用 `@v7`，GitHub releases 页面最新 tagged release 是 `v6.4.0`
   - What's unclear: `@v7` 是浮动主版本标签（跟踪 v6.x 最新）还是真正的 v7.x 系列
   - Recommendation: 使用 `@v7`，与官方文档保持一致。浮动标签跟踪最新兼容版本是 GitHub Actions 的标准做法。

2. **是否需要 go test 步骤**
   - What we know: D-03 决定 "GoReleaser 管一切"，GoReleaser 自带 go test 能力
   - What's unclear: GoReleaser v2 的 `release` 命令是否默认运行测试
   - Recommendation: GoReleaser `release` 命令不默认运行 `go test`。如果需要 CI 测试，可以添加 `before: hooks: [go test ./...]` 或添加独立步骤。但根据 D-03 保持简洁的决策，可以不在 release workflow 中测试，或者仅在 PR workflow 中测试（本阶段不涉及）。

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | 无代码变更，无传统测试框架 |
| Config file | 不适用 |
| Quick run command | `goreleaser check .goreleaser.yaml` (本地验证配置) |
| Full suite command | `goreleaser release --snapshot --clean` (本地快照构建验证) |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CICD-01 | v* tag push 触发 workflow | manual-only | 推送 v* tag 后观察 GitHub Actions 触发 | N/A -- 创建新文件 |
| CICD-02 | Windows amd64 二进制发布到 GitHub Releases | manual-only | 推送 tag 后检查 Release 产物 | N/A -- 创建新文件 |
| CICD-03 | ldflags 注入版本号 | smoke | `goreleaser release --snapshot --clean && ./dist/...exe --version` | N/A -- 创建新文件 |

### Sampling Rate
- **Per task commit:** `goreleaser check .goreleaser.yaml` (YAML 语法和配置验证)
- **Per wave merge:** 不适用（单 wave 阶段）
- **Phase gate:** `goreleaser release --snapshot --clean` (本地完整构建验证)

### Wave 0 Gaps
- [ ] 本地安装 GoReleaser CLI (`go install github.com/goreleaser/goreleaser/v2@latest`) -- 用于本地验证
- [ ] 需要推送到 GitHub 并创建 tag 才能完整验证 workflow -- 本地只能验证 GoReleaser 配置

**注意:** 本阶段是纯配置文件创建（YAML），不修改 Go 代码。传统单元测试不适用。验证方式：
1. `goreleaser check` -- 验证 YAML 语法和配置有效性
2. `goreleaser release --snapshot --clean` -- 本地快照构建（不发布）验证构建流程
3. 推送 tag 后观察 GitHub Actions 运行 -- 完整端到端验证

## Sources

### Primary (HIGH confidence)
- [GoReleaser GitHub Actions docs](https://goreleaser.com/customization/ci/actions/) -- 官方 workflow 模板，updated 2026-03-22
- [GoReleaser builds docs](https://goreleaser.com/customization/builds/) -- Go builder 配置，ldflags 选项
- [GoReleaser archives docs](https://goreleaser.com/customization/archive/) -- archive 格式和命名模板
- [GoReleaser checksum docs](https://goreleaser.com/customization/checksum/) -- checksum 配置
- [goreleaser-action releases](https://github.com/goreleaser/goreleaser-action/releases) -- v6.4.0 latest tagged release
- [GoReleaser v2.14 announcement](https://goreleaser.com/blog/archive/2026/) -- 最新 CLI 版本

### Secondary (MEDIUM confidence)
- [actions/checkout repo](https://github.com/actions/checkout) -- v4 latest major
- [actions/setup-go repo](https://github.com/actions/setup-go) -- v5 latest major, caching by default

### Tertiary (LOW confidence)
- None -- 所有配置细节均从官方文档获取

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- 所有版本号从官方文档和 releases 页面验证
- Architecture: HIGH -- 直接使用 GoReleaser 官方推荐模式，无创新
- Pitfalls: HIGH -- ldflags 大小写问题从 GoReleaser 默认配置和项目代码确认
- Validation: MEDIUM -- 本地可验证配置，完整验证需要 GitHub 环境

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (GoReleaser 和 GitHub Actions 变化较慢，30 天有效)
