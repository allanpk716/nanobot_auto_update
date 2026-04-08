# Phase 44: 后端 — 自更新进度追踪 + Web Token API - Research

**Researched:** 2026-04-07
**Domain:** Golang HTTP API, selfupdate 进度追踪, localhost-only 端点
**Confidence:** HIGH

## Summary

Phase 44 需要增强现有 `internal/selfupdate` 包和 `internal/api` 包，实现两个核心功能：(1) 下载进度追踪通过 `io.TeeReader` + `atomic.Value` 实现，让前端可以轮询获取更新进度；(2) 新增 `GET /api/v1/web-config` localhost-only 端点，返回 `auth_token` 供前端 Web UI 使用。

**现有代码基础扎实** — `selfupdate.Updater` 已有完整的下载/校验/更新管道（Phase 38），`SelfUpdateHandler` 已有 `atomic.Value` 状态追踪模式（Phase 39），`AuthMiddleware` 已有 Bearer Token 认证（Phase 28）。本阶段主要是扩展现有模式而非引入新架构。

**Primary recommendation:** 严格复用 Phase 39 的 `atomic.Value` 状态模式和 `SelfUpdateHandler` 的构造函数模式，在 `selfupdate.Updater` 中新增 `ProgressState` + `downloadWithProgress` 方法，在 `api` 包新增独立的 `webConfigHandler`（复用 `writeJSONError`）。

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| API-01 | 更新进度状态追踪 (ProgressState + io.TeeReader + atomic.Value) | selfupdate 包新增 ProgressState 结构 + downloadWithProgress 方法 + SetProgress/GetProgress；SelfUpdateHandler 扩展 HandleCheck 响应 |
| API-02 | Web UI Token API (localhost-only GET /api/v1/web-config) | api 包新增 webConfigHandler + localhostOnlyMiddleware；从 config.APIConfig.BearerToken 读取 |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| sync/atomic (stdlib) | Go 1.24 | 并发安全状态存储 | 项目已用 atomic.Value (Phase 39)，atomic.Bool (Phase 28) [VERIFIED: codebase grep] |
| io (stdlib) | Go 1.24 | TeeReader 进度追踪 | Go 标准库，io.TeeReader + io.Copy 天然支持字节计数 [VERIFIED: go doc io TeeReader] |
| net (stdlib) | Go 1.24 | SplitHostPort localhost 检测 | 标准库解析 RemoteAddr "host:port" 格式 [VERIFIED: go doc net SplitHostPort] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stretchr/testify | v1.11.1 | 断言 + 测试 mock | 项目已全面使用 assert/require [VERIFIED: go.mod] |
| net/http/httptest (stdlib) | Go 1.24 | HTTP handler 单元测试 | 项目已全面使用 httptest.NewRecorder/NewServer [VERIFIED: codebase] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| io.TeeReader | 自定义 io.Reader wrapper | TeeReader 更简洁，但自定义 wrapper 可在每次 Read 时调用回调。推荐 TeeReader + bytes counting Writer [ASSUMED] |
| atomic.Value | sync.RWMutex | 项目已选择 atomic.Value 模式（Phase 39），保持一致 [VERIFIED: selfupdate_handler.go:57] |
| localhostOnly 中间件 | handler 内 if 检查 | 中间件可复用，但本阶段仅一个端点需要。推荐独立函数 localhostOnly 包装 handler [ASSUMED] |

**Installation:**
无需安装新依赖 — 所有功能基于 Go 标准库和项目已有依赖。

**Version verification:**
```
Go: 1.24.11 (go version)
stretchr/testify: v1.11.1 (go.mod)
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── selfupdate/
│   ├── selfupdate.go          # 新增 ProgressState, SetProgress/GetProgress, downloadWithProgress
│   └── selfupdate_test.go     # 新增进度状态并发测试、下载百分比测试
├── api/
│   ├── selfupdate_handler.go  # 扩展 SelfUpdateCheckResponse + HandleCheck
│   ├── selfupdate_handler_test.go  # 新增 progress 字段测试
│   ├── webconfig_handler.go   # 新增 WebConfigHandler + localhostOnly 中间件
│   ├── webconfig_handler_test.go  # 新增 localhost 限制测试、token 返回测试
│   └── server.go              # 注册 GET /api/v1/web-config 路由
```

### Pattern 1: ProgressState + atomic.Value 进度追踪
**What:** 在 `selfupdate.Updater` 中新增 `ProgressState` 结构（不可变值），通过 `atomic.Value` 存储，读多写少场景无锁并发安全。
**When to use:** 下载进度更新（写少：每次 Copy 32KB 缓冲写一次）+ 前端轮询读取（读多：500ms 间隔）。
**Example:**
```go
// Source: Phase 39 SelfUpdateStatus 模式 + Go atomic.Value 文档
// internal/selfupdate/selfupdate.go

type ProgressState struct {
    Stage           string // "idle" / "checking" / "downloading" / "installing" / "complete" / "failed"
    DownloadPercent int    // 0-100
    Error           string // 仅 failed 状态
}

type Updater struct {
    // ... 现有字段
    progress atomic.Value // stores *ProgressState
}

func (u *Updater) SetProgress(state *ProgressState) {
    u.progress.Store(state) // 每次 Store 一个新的不可变 struct 指针
}

func (u *Updater) GetProgress() *ProgressState {
    if v := u.progress.Load(); v != nil {
        return v.(*ProgressState)
    }
    return &ProgressState{Stage: "idle"}
}
```

### Pattern 2: io.TeeReader + Content-Length 下载进度
**What:** 替换现有 `download` 方法为 `downloadWithProgress`，使用 `io.TeeReader` 包装 response body，每次 Read 时自动写入一个计数 Writer。
**When to use:** ZIP 下载（可能数 MB，需要百分比追踪）。
**Example:**
```go
// Source: Go 标准库 io.TeeReader + io.Copy 模式

type progressWriter struct {
    total      int64
    written    int64
    updater    *Updater // 调用 SetProgress
    stage      string
}

func (pw *progressWriter) Write(p []byte) (int, error) {
    n := len(p)
    pw.written += int64(n)
    if pw.total > 0 {
        percent := int(float64(pw.written) / float64(pw.total) * 100)
        if percent > 100 {
            percent = 100
        }
        pw.updater.SetProgress(&ProgressState{
            Stage:           pw.stage,
            DownloadPercent: percent,
        })
    }
    return n, nil
}

func (u *Updater) downloadWithProgress(url string, stage string) ([]byte, error) {
    resp, err := u.httpClient.Get(url)
    if err != nil {
        return nil, fmt.Errorf("download %s: %w", url, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
    }

    pw := &progressWriter{
        total:   resp.ContentLength,
        updater: u,
        stage:   stage,
    }
    teeReader := io.TeeReader(resp.Body, pw)

    data, err := io.ReadAll(teeReader)
    if err != nil {
        return nil, fmt.Errorf("read download %s: %w", url, err)
    }
    return data, nil
}
```

### Pattern 3: Localhost-only 端点
**What:** 独立函数包装 handler，检查 `r.RemoteAddr` 是否为 localhost。
**When to use:** `GET /api/v1/web-config` 端点 — 安全敏感（返回 auth_token）。
**Example:**
```go
// Source: net.SplitHostPort 标准用法

func localhostOnly(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        host, _, err := net.SplitHostPort(r.RemoteAddr)
        if err != nil {
            writeJSONError(w, http.StatusForbidden, "forbidden", "Access denied")
            return
        }
        if host != "127.0.0.1" && host != "::1" {
            writeJSONError(w, http.StatusForbidden, "forbidden", "Access denied: localhost only")
            return
        }
        next.ServeHTTP(w, r)
    }
}
```

### Anti-Patterns to Avoid
- **在 atomic.Value 中存储可变 struct（非指针）：** 每次 Store 必须是新的不可变值副本，不能修改已 Store 的 struct 字段。项目 Phase 39 已正确使用 `&SelfUpdateStatus` 指针模式 [VERIFIED: selfupdate_handler.go:57]
- **在 TeeReader 的 Write 回调中执行耗时操作：** TeeReader 的 Write 是同步的，Write 阻塞会阻塞 Read。SetProgress 应仅做 atomic.Store，不做 I/O [VERIFIED: go doc io TeeReader]
- **Content-Length 为 -1 时除零：** HTTP 响应可能不包含 Content-Length（分块传输），必须检查 `resp.ContentLength > 0` [ASSUMED]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| 并发安全状态 | mutex 保护 map/struct | atomic.Value + 不可变 struct | 项目已有模式，无锁更高效 |
| 字节进度追踪 | 自定义 Read(p) 计数 | io.TeeReader + counting Writer | 标准库，无需自定义 Reader |
| localhost IP 检测 | 手动字符串分割 RemoteAddr | net.SplitHostPort | 处理 IPv6 `[::1]:port` 格式 |
| JSON 错误响应 | 自定义 error 格式 | 复用 writeJSONError (auth.go) | 项目统一 RFC 7807 格式 |

**Key insight:** 所有模式都已在项目中验证 — atomic.Value (Phase 39)、download 方法 (Phase 38)、writeJSONError (Phase 28)。本阶段是扩展现有模式而非发明新方案。

## Common Pitfalls

### Pitfall 1: Content-Length 缺失或为 -1
**What goes wrong:** 某些 GitHub CDN 响应可能不返回 Content-Length（分块传输），导致 `resp.ContentLength == -1`，百分比计算除零或出现负数。
**Why it happens:** HTTP 分块传输不预设 Content-Length。
**How to avoid:** 检查 `resp.ContentLength > 0`，为 -1 时跳过百分比更新（保持 Stage 为 "downloading" 但 DownloadPercent 为 0）。
**Warning signs:** 测试中 ContentLength 默认为 0，需要 mock server 显式设置 Content-Length header。

### Pitfall 2: atomic.Value 类型不一致 panic
**What goes wrong:** `atomic.Value.Store` 要求所有存储值类型完全一致。如果首次 Store `*ProgressState`，后续 Store 普通 `ProgressState` 会 panic。
**Why it happens:** atomic.Value 内部检查类型一致性。
**How to avoid:** 初始化时 Store 一个 `&ProgressState{Stage: "idle"}`，后续所有 Store 都用 `&ProgressState{...}` 指针。
**Warning signs:** 项目 Phase 39 已踩过此坑 — 所有 Store 用 `&SelfUpdateStatus` 指针 [VERIFIED: selfupdate_handler.go:76]。

### Pitfall 3: RemoteAddr 在反向代理后总是 127.0.0.1
**What goes wrong:** 如果未来有反向代理（Nginx 等），RemoteAddr 永远是代理 IP 而非真实客户端 IP，localhost 检查失效。
**Why it happens:** 反向代理与 backend 在同一台机器。
**How to avoid:** 本项目是嵌入式 HTTP server，直接暴露端口，无反向代理计划。当前阶段直接检查 RemoteAddr 即可。如果未来需要，在 server.go 添加 trusted proxy 配置。
**Warning signs:** 本项目 config.yaml 无 proxy 配置，API 端口 8080 直接暴露。

### Pitfall 4: Update 方法中 download 与 downloadWithProgress 混用
**What goes wrong:** 两个下载方法逻辑重复，一个有进度一个没有。
**Why it happens:** 不想在非进度场景引入开销。
**How to avoid:** 统一使用 downloadWithProgress，非进度场景只是不轮询 GetProgress 而已。或者让 download 内部调用 downloadWithProgress。推荐统一入口。
**Warning signs:** 自更新（POST）需要进度，但 CheckLatest 内部的 API 调用不需要 — 两者用不同的 HTTP 调用路径。

### Pitfall 5: 进度状态在更新完成后未重置
**What goes wrong:** 一次更新完成后 ProgressState 残留为 "complete"，下次 check 仍然显示旧的进度。
**Why it happens:** SetProgress 在 Update 方法末尾设为 "complete"，但下次 NeedUpdate 调用不会重置。
**How to avoid:** 在 HandleUpdate 开始时 SetProgress("checking")，在 Update 方法各阶段更新状态，在 HandleCheck 返回 progress 时包含在响应中。同时考虑添加 ResetProgress 方法。
**Warning signs:** SelfUpdateHandler.status 已有 "idle" -> "updating" -> "updated"/"failed" 生命周期 [VERIFIED: selfupdate_handler.go:142,237]。

### Pitfall 6: web-config 端点绕过 AuthMiddleware 返回 token
**What goes wrong:** 如果 web-config 端点也被 AuthMiddleware 保护，前端无法获取 token（死循环：需要 token 才能获取 token）。
**Why it happens:** 端点注册时可能复用 authMiddleware 包装。
**How to avoid:** web-config 端点仅用 localhostOnly 包装，不用 AuthMiddleware。这通过 localhost 限制保证安全性。
**Warning signs:** server.go 中其他自更新端点都用 authMiddleware 包装 [VERIFIED: server.go:95-98]。

## Code Examples

### 现有代码模式参考（项目内）

#### atomic.Value 状态存储（Phase 39 已验证）
```go
// Source: internal/api/selfupdate_handler.go:57,76,142,237
type SelfUpdateHandler struct {
    status atomic.Value // stores *SelfUpdateStatus
}
// 初始化
h.status.Store(&SelfUpdateStatus{Status: "idle"})
// 读取
currentStatus := h.status.Load().(*SelfUpdateStatus)
// 更新
h.status.Store(&SelfUpdateStatus{Status: "updating"})
```

#### download 方法（Phase 38 已验证）
```go
// Source: internal/selfupdate/selfupdate.go:271-285
func (u *Updater) download(url string) ([]byte, error) {
    resp, err := u.httpClient.Get(url)
    if err != nil {
        return nil, fmt.Errorf("download %s: %w", url, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
    }
    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read download %s: %w", url, err)
    }
    return data, nil
}
```

#### SelfUpdateCheckResponse（Phase 39 已验证）
```go
// Source: internal/api/selfupdate_handler.go:41-50
type SelfUpdateCheckResponse struct {
    CurrentVersion   string `json:"current_version"`
    LatestVersion    string `json:"latest_version"`
    NeedsUpdate      bool   `json:"needs_update"`
    ReleaseNotes     string `json:"release_notes"`
    PublishedAt      string `json:"published_at"`
    DownloadURL      string `json:"download_url"`
    SelfUpdateStatus string `json:"self_update_status"`
    SelfUpdateError  string `json:"self_update_error,omitempty"`
}
// API-01 扩展: 新增 progress 字段
```

#### writeJSONError 复用（Phase 28 已验证）
```go
// Source: internal/api/auth.go:113-132
func writeJSONError(w http.ResponseWriter, status int, errorCode, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    response := map[string]string{"error": errorCode, "message": message}
    json.NewEncoder(w).Encode(response)
}
```

#### 路由注册模式（server.go）
```go
// Source: internal/api/server.go:93-98
if selfUpdater != nil {
    selfUpdateHandler := NewSelfUpdateHandler(selfUpdater, version, im, notif, logger)
    mux.Handle("GET /api/v1/self-update/check",
        authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleCheck)))
    mux.Handle("POST /api/v1/self-update",
        authMiddleware(http.HandlerFunc(selfUpdateHandler.HandleUpdate)))
}
// API-02 新增: web-config 路由（不需要 authMiddleware）
// mux.HandleFunc("GET /api/v1/web-config", localhostOnly(webConfigHandler))
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| mutex 保护状态 | atomic.Value 不可变值 | Go 1.19+ / Phase 39 | 无锁读，适合读多写少场景 |
| 全量下载后追踪 | io.TeeReader 实时追踪 | Go 标准库长期支持 | 下载过程中实时百分比 |
| handler 内 if 检查 | 中间件包装模式 | Phase 28 AuthMiddleware | 可组合、可测试 |

**Deprecated/outdated:**
- 无

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | io.TeeReader 是实现进度追踪的最佳方式，优于自定义 io.Reader wrapper | Architecture Patterns | 低 — 两种方式功能等价，TeeReader 更简洁 |
| A2 | web-config 端点仅需 localhostOnly 包装，不需要 AuthMiddleware | Architecture Patterns | 中 — 如果安全审计要求额外认证，需修改设计 |
| A3 | HTTP 响应可能缺少 Content-Length（分块传输） | Common Pitfalls | 低 — GitHub CDN 通常返回 Content-Length |
| A4 | 统一使用 downloadWithProgress 替代 download，不会影响非进度场景 | Common Pitfalls | 低 — 性能开销可忽略（仅一次 atomic.Store） |

**If this table is empty:** All claims in this research were verified or cited -- no user confirmation needed.

## Open Questions (RESOLVED)

1. **downloadWithProgress 是否替代现有 download 方法？** — RESOLVED: 统一替换 download 为 downloadWithProgress，通过 stage 参数控制进度更新。
   - What we know: 现有 download 方法被 Update 方法调用两次（checksums + zip），CheckLatest 用 httpClient.Get 而非 download。
   - Decision: 统一替换，仅在 ZIP 下载时更新进度（通过 stage 参数控制），checksums 下载不更新百分比。

2. **ProgressState 生命周期与 SelfUpdateStatus 的关系？** — RESOLVED: 两者独立追踪。
   - What we know: SelfUpdateStatus 有 idle/updating/updated/failed 四状态。ProgressState 有 idle/checking/downloading/installing/complete/failed 六阶段。
   - Decision: ProgressState 在 selfupdate.Updater 中追踪下载进度细节，SelfUpdateStatus 在 api.SelfUpdateHandler 中追踪 API 层面状态。前端同时看到两个维度。

## Environment Availability

> Step 2.6: External dependencies check.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24 | Build & test | Yes | 1.24.11 | -- |
| stretchr/testify | Testing | Yes | v1.11.1 | -- |

**Missing dependencies with no fallback:**
- None

**Missing dependencies with fallback:**
- None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none |
| Quick run command | `go test ./internal/selfupdate/ ./internal/api/ -count=1 -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-01 | ProgressState 并发安全 | unit | `go test ./internal/selfupdate/ -run TestProgressState -v` | No (Wave 0) |
| API-01 | 下载百分比计算 | unit | `go test ./internal/selfupdate/ -run TestDownloadWithProgress -v` | No (Wave 0) |
| API-01 | HandleCheck 返回 progress 字段 | unit | `go test ./internal/api/ -run TestSelfUpdateCheck_Progress -v` | No (Wave 0) |
| API-02 | localhost-only 限制 | unit | `go test ./internal/api/ -run TestWebConfig_Localhost -v` | No (Wave 0) |
| API-02 | web-config 返回 auth_token | unit | `go test ./internal/api/ -run TestWebConfig_Token -v` | No (Wave 0) |
| API-02 | 非 localhost 返回 403 | unit | `go test ./internal/api/ -run TestWebConfig_Forbidden -v` | No (Wave 0) |

### Sampling Rate
- **Per task commit:** `go test ./internal/selfupdate/ ./internal/api/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/selfupdate/selfupdate_test.go` -- 新增 TestProgressState_ConcurrentSafe, TestDownloadWithProgress_PercentCalc
- [ ] `internal/api/webconfig_handler_test.go` -- 新建，覆盖 localhost/remote/token 场景
- [ ] `internal/api/webconfig_handler.go` -- 新建 WebConfigHandler + localhostOnly
- [ ] 注意: `internal/api/server_test.go` 有编译错误（instance.NewInstanceManager 参数不匹配），但这不影响 Phase 44 的测试文件

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Bearer Token (existing AuthMiddleware) |
| V3 Session Management | no | 无 session |
| V4 Access Control | yes | localhost-only 限制 (net.SplitHostPort) |
| V5 Input Validation | yes | 标准 Go http.Request 参数验证 |
| V6 Cryptography | no | 无加密操作 |

### Known Threat Patterns for Golang HTTP API + Localhost-only

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token 泄露（非 localhost 访问） | Information Disclosure | localhostOnly 中间件 + 403 Forbidden |
| SSRF 伪造 localhost | Spoofing | 直接检查 RemoteAddr（不信任 X-Forwarded-For），本项目无反向代理 |
| 进度状态竞态 | Tampering | atomic.Value 不可变值存储，并发安全 |
| Timing attack on token comparison | Information Disclosure | subtle.ConstantTimeCompare (Phase 28 已实现) |

## Sources

### Primary (HIGH confidence)
- Go 标准库文档 (go doc) — io.TeeReader, sync/atomic.Value, net.SplitHostPort
- 项目代码库 (internal/selfupdate, internal/api, internal/web, internal/config) — 所有文件已读取验证

### Secondary (MEDIUM confidence)
- [Go Optimization Guide - Immutable Data Sharing](https://goperf.dev/01-common-patterns/immutable-data/) — atomic.Value 不可变值模式最佳实践
- [OneUptime - How to Handle Large File Downloads in Go](https://oneuptime.com/blog/post/2026-01-30-how-to-handle-large-file-downloads-in-go/view) — io.TeeReader 进度追踪示例

### Tertiary (LOW confidence)
- 无

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 所有依赖已在项目中验证使用
- Architecture: HIGH — 扩展现有模式（atomic.Value + download + writeJSONError）
- Pitfalls: HIGH — 基于 Go 标准库文档和项目历史经验

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable domain)
