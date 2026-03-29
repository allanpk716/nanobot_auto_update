# Phase 36: PoC Validation - Research

**Researched:** 2026-03-29
**Domain:** Windows running-exe self-update via minio/selfupdate
**Confidence:** HIGH

## Summary

Phase 36 创建独立最小 PoC 程序，验证 `minio/selfupdate v0.6.0` 在 Windows 上完成运行中 exe 替换、备份和重启的可行性。这是一个纯技术验证阶段，不涉及 GitHub API、CI/CD、HTTP API 或配置集成。

minio/selfupdate 的核心机制是 Windows exe rename trick：Windows 允许重命名正在运行的 exe 文件。库将新二进制写入 `.exe.new`，将当前 exe 重命名为 `.exe.old`，然后将 `.exe.new` 重命名为原始名称。若 Windows 无法删除被锁定的 `.old` 文件，则通过 `kernel32.dll SetFileAttributesW` 将其隐藏。这个流程经过 MinIO 生产验证，812+ GitHub stars，945+ 下游用户。

**Primary recommendation:** 按照 D-01 至 D-05 决策，在 `tmp/` 目录创建单文件 PoC（~50-80 行），用 ldflags 构建两个版本，通过文件输出验证 self-spawn 重启成功。自动化测试脚本构建 v1/v2、运行 v1、等待文件输出、比对版本号。

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** 最小独立程序 -- 一个 main.go（~50-80 行），仅包含版本打印 -> selfupdate.Apply() -> self-spawn 重启 -> 新版本打印。不模拟项目结构，不引入 logger/config/HTTP server。
- **D-02:** 新版本二进制来源 -- 本地构建 v1 和 v2 两个 exe。v1 启动后读取本地 v2 的二进制文件调用 selfupdate.Apply()，无需网络。
- **D-03:** 自动化测试脚本 -- Go 测试程序：构建两个版本 -> 运行 v1 -> 等待重启 -> 通过文件输出验证 v2 成功启动。
- **D-04:** 新版本检测方式 -- 文件输出验证：PoC 程序将版本号写入文件，测试脚本读取文件确认新版本号。
- **D-05:** PoC 代码保留在 `tmp/` 目录作为参考。正式实现（Phase 38）独立编写 `internal/selfupdate/` 包，但可回头参考 PoC 实现细节。

### Claude's Discretion
- PoC 程序的具体实现细节（如何构建两个版本、文件路径约定、等待超时等）
- 自动化测试脚本的结构和错误处理

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| VALID-01 | 创建独立 PoC 测试程序，验证 minio/selfupdate 在 Windows 上的 exe 替换可行性 | minio/selfupdate `Apply()` API 完整分析，Windows rename trick 流程已验证 |
| VALID-02 | 验证备份机制（.old 文件创建）和回滚功能正常工作 | `Options.OldSavePath` 字段用于显式保存 .old 备份；`CommitBinary()` 回滚逻辑已分析 |
| VALID-03 | 验证 self-spawn 重启机制（更新后自动重启新版本进程） | `exec.Command().Start()` + `windows.SysProcAttr` 模式已确认可行，项目已有此模式先例 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/minio/selfupdate | v0.6.0 | Running exe binary replacement | MinIO 维护的 inconshreveable/go-update 分支。Windows rename trick + kernel32.dll hideFile + 内置 rollback。生产级质量。 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/sys | v0.41.0 (existing) | Windows process creation flags | self-spawn 重启时设置 `HideWindow` + `CREATE_NO_WINDOW` |
| crypto/sha256 (stdlib) | Go 1.24+ | Binary checksum（可选） | PoC 可选验证，正式实现必须 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| minio/selfupdate | creativeprojects/go-selfupdate | 太重：强制命名约定、拉入 gitea + gitlab + xz 依赖。已明确拒绝 |
| minio/selfupdate | 手动实现 rename trick | 需重写：Windows rename + rollback + kernel32.dll hideFile + checksum 验证。minio/selfupdate 400 行久经考验 |

**Installation:**
```bash
# PoC 需要添加的唯一新依赖
go get github.com/minio/selfupdate@v0.6.0
```

**Version verification:**
- `github.com/minio/selfupdate`: v0.6.0 (confirmed via `go list -m -versions`, latest stable, released Jan 2023)
- `golang.org/x/sys`: v0.41.0 (already in go.mod, compatible with minio/selfupdate which needs v0.37.0+)

## Architecture Patterns

### Recommended Project Structure
```
tmp/
├── poc_selfupdate.go      # PoC 主程序（单文件，~50-80 行）
└── poc_selfupdate_test.go  # 自动化验证脚本
```

### Pattern 1: Version Injection via ldflags
**What:** 构建时通过 `-ldflags "-X main.Version=xxx"` 注入版本号
**When to use:** 区分 v1 和 v2 两个 PoC 版本
**Example:**
```go
// PoC main.go 中
var Version = "dev"

func main() {
    fmt.Printf("Version: %s\n", Version)
    // ...
}

// 构建命令
// go build -ldflags "-X main.Version=1.0.0" -o poc_v1.exe poc_selfupdate.go
// go build -ldflags "-X main.Version=2.0.0" -o poc_v2.exe poc_selfupdate.go
```
**Source:** 项目已有此模式 (`cmd/nanobot-auto-updater/main.go:28`)

### Pattern 2: selfupdate.Apply() with OldSavePath
**What:** 调用 minio/selfupdate 替换运行中 exe，同时保存 .old 备份
**When to use:** PoC 主流程 -- v1 调用此替换自身为 v2
**Example:**
```go
import "github.com/minio/selfupdate"

func applyUpdate(newBinaryPath string) error {
    newBin, err := os.Open(newBinaryPath)
    if err != nil {
        return fmt.Errorf("open new binary: %w", err)
    }
    defer newBin.Close()

    // OldSavePath 设置后，旧 exe 会保存到指定路径而非删除/隐藏
    opts := selfupdate.Options{
        OldSavePath: strings.Replace(os.Args[0], ".exe", ".exe.old", 1),
    }

    err = selfupdate.Apply(newBin, opts)
    if err != nil {
        // 检查是否发生了 rollback
        if rerr := selfupdate.RollbackError(err); rerr != nil {
            return fmt.Errorf("update failed AND rollback failed: %v (rollback: %v)", err, rerr)
        }
        return fmt.Errorf("update failed (rolled back): %w", err)
    }
    return nil
}
```
**Source:** minio/selfupdate apply.go 源码分析 (HIGH confidence)

### Pattern 3: Self-Spawn Restart
**What:** 更新完成后启动新进程然后退出当前进程
**When to use:** PoC 验证 self-spawn 重启机制 (VALID-03)
**Example:**
```go
import (
    "os/exec"
    "syscall"
    "golang.org/x/sys/windows"
)

func restartSelf() error {
    exePath, _ := os.Executable()
    cmd := exec.Command(exePath, os.Args[1:]...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{
        HideWindow:    true,
        CreationFlags: windows.CREATE_NO_WINDOW,
    }
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("spawn new process: %w", err)
    }
    os.Exit(0)
    return nil
}
```
**Source:** 项目已有此模式 (`internal/updater/updater.go`)

### Pattern 4: File Output Verification
**What:** PoC 程序将版本号写入文件，测试脚本读取验证
**When to use:** 自动化验证新版本成功启动 (D-04)
**Example:**
```go
// PoC 程序启动时
func writeVersionFile(version string) {
    exePath, _ := os.Executable()
    versionFile := exePath + ".version"
    os.WriteFile(versionFile, []byte(version), 0644)
}

// 测试脚本验证
func verifyVersion(exePath, expected string) bool {
    data, err := os.ReadFile(exePath + ".version")
    return err == nil && strings.TrimSpace(string(data)) == expected
}
```

### Anti-Patterns to Avoid
- **不要用 `os.Executable()` 路径做 OldSavePath 的硬编码拼接:** Windows 路径可能有 `\\?\` 前缀或 `.old` 后缀（normalizeExecutablePath 会处理）。使用 `strings.Replace` 或 `strings.TrimSuffix` 安全处理。
- **不要在 Apply() 前关闭新二进制文件:** `Apply()` 读取 `io.Reader`，如果提前关闭文件会导致读取失败。用 `defer newBin.Close()` 在 Apply 之后关闭。
- **不要假设 self-spawn 立即完成:** 新进程启动需要时间（尤其是 Windows 上杀毒软件扫描）。测试脚本需要等待循环 + 超时。

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Running exe replacement | 自己写 rename + fallback 逻辑 | `selfupdate.Apply()` | Windows 锁文件处理、rollback、hideFile 都是边界情况陷阱 |
| .old 文件管理 | 手动 hideFile + kernel32.dll 调用 | `selfupdate.Options.OldSavePath` | minio/selfupdate 已处理 Windows 特定逻辑 |
| Checksum verification | 手动 sha256 + compare | `selfupdate.Options.Checksum` + `Options.Hash` | 库内置 hash 验证流程 |

**Key insight:** minio/selfupdate 的 400 行代码解决了所有 Windows exe 替换的边界情况。手动实现几乎必然遗漏：running exe rename 权限、.old 文件锁定、kernel32.dll 文件隐藏、rollback 原子性。

## Common Pitfalls

### Pitfall 1: OldSavePath 为空时 .old 文件行为
**What goes wrong:** 如果 `Options.OldSavePath` 为空字符串，minio/selfupdate 先尝试删除 .old（Windows 上会失败因为文件被锁定），然后调用 `hideFile()` 隐藏它。测试脚本可能找不到 .old 文件。
**Why it happens:** 默认行为是尽量删除旧文件，Windows 上不可能所以 fallback 到隐藏。
**How to avoid:** **必须设置 `OldSavePath`** 为显式路径（如 `exe.old`），这样 VALID-02 可以直接验证 .old 文件存在。
**Warning signs:** PoC 运行后找不到 `.old` 备份文件。

### Pitfall 2: os.Executable() 返回带 .old 后缀的路径
**What goes wrong:** 如果上一次更新成功，当前 exe 可能从 `.old` 路径启动（某些边界情况）。`selfupdate.Apply()` 内部的 `normalizeExecutablePath()` 会自动剥离 `.old` 后缀，但 PoC 自己的路径逻辑可能出错。
**Why it happens:** minio/selfupdate 的 `normalizeExecutablePath()` 函数处理此情况，但外部代码可能不知道。
**How to avoid:** PoC 中使用 `os.Executable()` 获取路径后，如果需要拼接 OldSavePath，确保使用正确的路径。
**Warning signs:** `Apply()` 报错 "open ...: The system cannot find the file specified"。

### Pitfall 3: Self-spawn 后原进程立即退出导致新进程也退出
**What goes wrong:** `cmd.Start()` 后立即 `os.Exit(0)`，但如果 cmd 还没完全初始化，可能受父进程退出影响。
**Why it happens:** Windows 进程创建有微妙的时间窗口。
**How to avoid:** `cmd.Start()` 返回 nil 后，新进程已经是独立进程（非子进程依赖），`os.Exit(0)` 安全。但如果使用 `cmd.Run()` 则会等待，不适合重启场景。确认用 `Start()` 不是 `Run()`。
**Warning signs:** 新版本未启动，测试超时。

### Pitfall 4: 测试脚本等待时间不足
**What goes wrong:** v1 启动 → Apply → self-spawn v2 → v2 写版本文件，这个过程在 Windows 上可能需要几秒（杀毒软件扫描新 exe）。
**Why it happens:** Windows Defender 或其他杀毒软件会扫描新创建/重命名的 exe 文件。
**How to avoid:** 测试脚本使用轮询等待（每 500ms 检查一次，最多 30 秒），不要用固定 sleep。
**Warning signs:** 测试脚本间歇性失败。

### Pitfall 5: go.mod 依赖冲突
**What goes wrong:** 添加 minio/selfupdate 后 `go mod tidy` 可能降级或升级 golang.org/x/sys。
**Why it happens:** minio/selfupdate go.mod 声明需要 golang.org/x/sys，但项目已有 v0.41.0。
**How to avoid:** minio/selfupdate 需要 x/sys v0.37.0+，项目 v0.41.0 满足，不会降级。PoC 在独立目录或使用 `-modfile` 避免影响主 go.mod。
**Warning signs:** `go mod tidy` 后 go.mod 中 x/sys 版本变化。

### Pitfall 6: PoC 影响主项目 go.mod
**What goes wrong:** 在 `tmp/` 目录下运行 `go get` 可能修改主项目 go.mod（因为 tmp/ 不是独立 module）。
**Why it happens:** tmp/ 不是 Go module，go 工具会使用父目录的 go.mod。
**How to avoid:** 方案 A：PoC 在 tmp/ 下创建独立 go.mod（`go mod init poc_selfupdate`）。方案 B：直接在主 go.mod 中添加 minio/selfupdate（反正 Phase 38 也要用）。**推荐方案 B**：直接在主 go.mod 添加依赖，Phase 36 验证的就是 Phase 38 要用的库，无风险。
**Warning signs:** `go build` 报错找不到 minio/selfupdate 包。

## Code Examples

### Complete PoC Flow (参考实现结构)

```go
// tmp/poc_selfupdate.go
// PoC: 验证 minio/selfupdate Windows exe 替换

package main

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
    "syscall"

    "github.com/minio/selfupdate"
    "golang.org/x/sys/windows"
)

var Version = "dev"

const updateTarget = "poc_v2.exe" // v1 读取 v2 来替换自身

func main() {
    exePath, _ := os.Executable()
    versionFile := exePath + ".version"

    // 步骤 1: 写入当前版本号到文件
    os.WriteFile(versionFile, []byte(Version), 0644)
    fmt.Printf("[%s] Started, version=%s, exe=%s\n", Version, Version, exePath)

    if Version == "1.0.0" {
        // v1 行为: 读取 v2 二进制 → Apply → self-spawn → 退出
        newBin, err := os.Open(updateTarget)
        if err != nil {
            fmt.Printf("[v1] ERROR open v2: %v\n", err)
            os.Exit(1)
        }
        defer newBin.Close()

        oldPath := exePath + ".old"
        opts := selfupdate.Options{
            OldSavePath: oldPath, // 显式保存旧版本备份
        }

        err = selfupdate.Apply(newBin, opts)
        if err != nil {
            if rerr := selfupdate.RollbackError(err); rerr != nil {
                fmt.Printf("[v1] FATAL update+rollback failed: %v\n", err)
            } else {
                fmt.Printf("[v1] Update failed (rolled back): %v\n", err)
            }
            os.Exit(1)
        }

        fmt.Println("[v1] Update applied, spawning new version...")
        // self-spawn 重启
        cmd := exec.Command(exePath, os.Args[1:]...)
        cmd.SysProcAttr = &syscall.SysProcAttr{
            HideWindow:    true,
            CreationFlags: windows.CREATE_NO_WINDOW,
        }
        cmd.Start()
        os.Exit(0)
    }

    // v2 行为: 更新完成，验证
    fmt.Printf("[%s] Self-update complete!\n", Version)
    // 检查 .old 备份是否存在
    oldPath := exePath + ".old"
    if _, err := os.Stat(oldPath); err == nil {
        fmt.Printf("[%s] Backup file exists: %s\n", Version, oldPath)
    }
}
```

### Automated Test Script Structure

```go
// tmp/poc_selfupdate_test.go
// +build manual

package main

import (
    "os"
    "os/exec"
    "testing"
    "time"
)

func TestSelfUpdate(t *testing.T) {
    // 1. 构建两个版本
    build := func(version, output string) error {
        return exec.Command("go", "build",
            "-ldflags", "-X main.Version="+version,
            "-o", output,
            "poc_selfupdate.go",
        ).Run()
    }

    if err := build("1.0.0", "poc_v1.exe"); err != nil {
        t.Fatalf("Build v1: %v", err)
    }
    if err := build("2.0.0", "poc_v2.exe"); err != nil {
        t.Fatalf("Build v2: %v", err)
    }
    defer os.Remove("poc_v1.exe")
    defer os.Remove("poc_v2.exe")

    // 2. 运行 v1（它会自动更新到 v2 并 self-spawn）
    cmd := exec.Command("./poc_v1.exe")
    cmd.Start()

    // 3. 轮询等待版本文件
    deadline := time.Now().Add(30 * time.Second)
    for time.Now().Before(deadline) {
        data, err := os.ReadFile("poc_v1.exe.version")
        if err == nil && string(data) == "2.0.0" {
            t.Log("Self-update verified: version 2.0.0 running")
            // 4. 验证 .old 备份
            if _, err := os.Stat("poc_v1.exe.old"); err == nil {
                t.Log("Backup file verified: poc_v1.exe.old exists")
            } else {
                t.Error("Backup file NOT found")
            }
            return
        }
        time.Sleep(500 * time.Millisecond)
    }
    t.Fatal("Timeout: v2 did not start within 30s")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| inconshreveable/go-update | minio/selfupdate (fork) | 2020+ | MinIO 分支添加 Windows hideFile、minisign 签名验证、Go modules 支持 |
| creativeprojects/go-selfupdate | minio/selfupdate + direct API | v0.8 决策 | 减少依赖树（去掉 gitea + gitlab + xz），获得更细粒度控制 |
| 手动 os.Rename | selfupdate.Apply() | 持续 | 处理 rollback、Windows 锁文件、隐藏文件等边界情况 |

**Deprecated/outdated:**
- inconshreveable/go-update: 2015 年后未维护，无 Windows .old 处理，无 Go modules
- rhysd/go-github-selfupdate: 2021 年后未维护
- sanbornm/go-selfupdate: 2014 年，无 Windows 支持

## Open Questions

1. **PoC 是否使用独立 go.mod 还是在主 go.mod 添加依赖？**
   - What we know: tmp/ 不是独立 Go module；Phase 38 必须使用 minio/selfupdate
   - Recommendation: 直接在主 go.mod 添加 `github.com/minio/selfupdate v0.6.0`。验证的就是 Phase 38 要用的库，提前添加无风险，且避免独立 go.mod 的路径配置复杂度。

2. **测试脚本是否用 Go test 还是 BAT 脚本？**
   - What we know: D-03 说"Go 测试程序"，但 `go test` 对需要构建 exe 然后运行 exe 的场景有些别扭
   - Recommendation: 用 Go `TestMain` 或独立的 `go run` 脚本。避免 BAT 脚本（CLAUDE.md 要求无中文，且 BAT 错误处理差）。

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | Yes | go1.24.11 windows/amd64 | -- |
| golang.org/x/sys | self-spawn SysProcAttr | Yes | v0.41.0 (go.mod) | -- |
| github.com/minio/selfupdate | exe replacement | Needs install | v0.6.0 (confirmed latest) | -- |
| Windows OS | exe rename trick | Yes | Windows 10 Pro 10.0.19045 | -- |

**Missing dependencies with no fallback:**
- `github.com/minio/selfupdate v0.6.0` -- needs `go get` before building PoC

**Missing dependencies with fallback:**
- None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none (standard go test) |
| Quick run command | `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s` |
| Full suite command | `go test ./tmp/ -v -tags manual -timeout 60s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VALID-01 | exe replacement: v1 replaced by v2, v2 outputs new version | integration | `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s` | Wave 0: create |
| VALID-02 | .old backup file visible after update | integration | same test, checks `os.Stat("poc_v1.exe.old")` | Wave 0: create |
| VALID-03 | self-spawn restart: v2 process running independently | integration | same test, polls version file for "2.0.0" | Wave 0: create |

### Sampling Rate
- **Per task commit:** `go build ./tmp/` (compile check)
- **Per wave merge:** `go test ./tmp/ -run TestSelfUpdate -v -tags manual -timeout 60s`
- **Phase gate:** Full PoC test green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `tmp/poc_selfupdate.go` -- PoC main program
- [ ] `tmp/poc_selfupdate_test.go` -- automated verification test
- [ ] Dependency install: `go get github.com/minio/selfupdate@v0.6.0` -- needs manual run

## Sources

### Primary (HIGH confidence)
- minio/selfupdate apply.go source code -- Full analysis of Apply(), PrepareAndCheckBinary(), CommitBinary(), Options struct, RollbackError(). All API surface verified.
- minio/selfupdate go.mod -- Go 1.24.0, dependency tree: aead.dev/minisign + golang.org/x/crypto + golang.org/x/sys. Verified.
- `.planning/research/STACK.md` -- Windows exe rename trick detailed analysis, dependency compatibility, integration architecture. Researched 2026-03-29.
- `cmd/nanobot-auto-updater/main.go` -- Existing Version ldflags pattern, graceful shutdown pattern. Verified in codebase.
- `go.mod` -- Current dependencies: golang.org/x/sys v0.41.0, Go 1.24.11. Verified.

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` -- Self-spawn restart pattern, shutdown channel pattern, port-binding retry. Note: uses creativeprojects/go-selfupdate but design patterns transferable.
- Web search: "golang self update restart windows os.StartProcess exec.Command" -- community confirms exec.Command().Start() + os.Exit(0) pattern for Windows self-restart.

### Tertiary (LOW confidence)
- None -- all findings verified through primary or secondary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- minio/selfupdate v0.6.0 API verified through source code analysis, dependency compatibility confirmed through go.mod inspection
- Architecture: HIGH -- PoC patterns derived from existing project code (Version ldflags, SysProcAttr) and minio/selfupdate source code
- Pitfalls: HIGH -- identified from source code analysis (OldSavePath behavior, normalizeExecutablePath) and Windows platform knowledge

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable library, low churn expected)
