# Stack Research

**Domain:** Self-Update via GitHub Releases (v0.8 milestone)
**Researched:** 2026-03-29
**Confidence:** HIGH (verified with source code analysis, official docs, and go.mod inspection)

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **github.com/minio/selfupdate** | v0.6.0 | Running exe binary replacement | Fork of inconshreveable/go-update maintained by MinIO. Handles Windows running-exe replacement via rename trick (`app.exe` -> `.app.exe.old`, `.app.exe.new` -> `app.exe`). Built-in rollback. Windows-specific `hideFile()` via kernel32.dll for `.old` cleanup. v0.6.0 released Jan 2023, 812 stars, 945+ downstream users. |
| **github.com/google/go-github/v74** | v74.0.0 | GitHub Releases API client | Official Google-maintained Go client for GitHub API v3. `Repositories.GetLatestRelease()` fetches latest release info including assets, tag names, and download URLs. Minimal dependency: only `go-querystring` + `oauth2`. v74 is current as of 2025. |
| **GoReleaser** | v2.x | CI/CD release automation | Industry standard for Go binary releases. Produces GitHub Releases with tagged binaries, checksums, and changelogs. Single `.goreleaser.yaml` config. Trigger on tag push (`v*` pattern). Produces `nanobot-auto-updater_windows_amd64.exe` asset matching go-selfupdate naming conventions. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **crypto/sha256** (stdlib) | Go 1.24+ | Binary checksum verification | Verify downloaded binary integrity against checksums from GitHub Release. `minio/selfupdate` supports `Options.Checksum` for this. |
| **encoding/hex** (stdlib) | Go 1.24+ | Checksum string decoding | Decode hex-encoded SHA256 checksums from GitHub Release assets for verification. |
| **archive/zip** (stdlib) | Go 1.24+ | Unzip release assets | If GoReleaser produces `.zip` archives instead of raw binaries, extract the exe from the zip. |
| **net/http** (stdlib) | Go 1.24+ | Download release binary | Download the new exe from GitHub Release asset URL. Pass `resp.Body` directly to `selfupdate.Apply()`. |
| **golang.org/x/sys** | v0.41.0 | Windows service management | Already in go.mod. Needed for potential service stop/restart during update if running as Windows service. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **GoReleaser CLI + GitHub Action** | Build + publish releases on tag push | Use `goreleaser/goreleaser-action@v6` in workflow. Config in `.goreleaser.yaml`. |
| **GitHub Actions** | CI/CD pipeline | Workflow `.github/workflows/release.yml` triggers on tag push `v*`. Runs `goreleaser` to build Windows amd64 binary and publish GitHub Release. |
| **git tag** | Version trigger | Push `v0.8.0` tag to trigger release pipeline. Matches semver pattern expected by self-update logic. |

## Installation

```bash
# Core self-update dependencies
go get github.com/minio/selfupdate@v0.6.0
go get github.com/google/go-github/v74@v74.0.0

# No other new dependencies needed - crypto/sha256, encoding/hex,
# archive/zip, net/http are all Go standard library
```

## Integration with Existing Stack

### Existing Stack Reuse

| Existing Component | Reuse Pattern | Rationale |
|-------------------|---------------|-----------|
| **internal/api/server.go** | Add new self-update handler | Follow existing handler registration pattern. New `SelfUpdateHandler` alongside `TriggerHandler`. |
| **internal/api/auth** | Bearer Token on self-update endpoint | Self-update is destructive, must be authenticated. Reuse existing middleware. |
| **internal/api/trigger.go patterns** | Atomic update control (`sync.AtomicBool`) | Self-update needs same concurrency guard to prevent overlapping updates. |
| **internal/config** | Add `SelfUpdateConfig` section | GitHub owner/repo, update channel, backup path. Extends existing YAML config. |
| **internal/notifier** | Pushover notification on self-update | Notify user when self-update starts/completes. Reuse existing Notifier interface. |
| **internal/updatelog** | Log self-update operation | Record self-update in UpdateLog alongside nanobot updates. Same JSONL persistence. |
| **Version injection** (main -> server -> handler) | Inject build version for comparison | Already exists from Phase 29. Use `ldflags` in GoReleaser to set version at build time. |

### Integration Architecture

```
HTTP Request -> SelfUpdateHandler
                    |
                    v
            1. GetLatestRelease() via go-github
                    |
                    v
            2. Compare versions (tag vs current)
                    |
                    v
            3. Download asset binary
                    |
                    v
            4. selfupdate.Apply(binary, opts)
                    |
                    v
            5. Process exits (or service restarts)
```

### Critical Integration Detail: Windows Running Exe

The `minio/selfupdate` library handles the core Windows challenge:

1. Writes new binary to `.nanobot-auto-updater.exe.new`
2. Renames running exe to `.nanobot-auto-updater.exe.old` (Windows allows rename of running exe)
3. Renames new binary to `nanobot-auto-updater.exe`
4. On Windows, cannot delete `.old` file (still locked), so hides it via `kernel32.dll SetFileAttributesW`
5. On next restart, `.old` file can be cleaned up

This means the update takes effect on **next process restart**. The handler should:
- Apply the update (binary replacement)
- Return success response to client
- The running process continues serving with old binary
- Next restart (manual or service restart) loads new version

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| **minio/selfupdate** | inconshreveable/go-update | Original is unmaintained (last release 2015, no v0.6.0 fixes). MinIO fork is actively maintained, adds Windows hide file support, minisign verification. Same API surface. |
| **minio/selfupdate** | creativeprojects/go-selfupdate | Too opinionated: enforces strict asset naming (`{cmd}_{goos}_{goarch}.zip`), pulls in heavy dependency tree (go-github/v74 + gitea + gitlab + semver + xz). We need fine-grained control over download/verification. Use minio/selfupdate for the hard part (exe replacement) + go-github directly for release discovery. |
| **minio/selfupdate** | Manual implementation (rename + download) | Would need to reimplement: Windows rename trick, rollback on failure, hide `.old` file via kernel32.dll, checksum verification, signature verification. All solved by minio/selfupdate in ~400 lines of battle-tested code. |
| **GoReleaser** | Manual GitHub Actions (go build + gh release) | GoReleaser is simpler to configure, generates checksums automatically, handles changelogs, and produces asset naming compatible with self-update discovery. Manual approach requires more YAML and error-prone scripting. |
| **go-github/v74** | Direct HTTP to api.github.com | go-github handles pagination, rate limiting, authentication, error parsing, and type safety. Direct HTTP requires manual JSON parsing and header management. |
| **go-github/v74** | creativeprojects/go-selfupdate (includes GitHub client) | creativeprojects pulls in go-github/v74 anyway, plus gitea, gitlab, semver, xz libraries we do not need. Using go-github directly gives us only what we need. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **creativeprojects/go-selfupdate** | Enforces naming conventions (`{cmd}_{goos}_{goarch}.ext`), heavy dependency tree (gitea + gitlab + xz + semver), no GitHub Release releases (uses tags but no formal releases published). Opinionated for generic self-update, not suited for service-level control. | minio/selfupdate + go-github directly |
| **rhysd/go-github-selfupdate** | Unmaintained since ~2021. Fork of original creativeprojects but stale. | minio/selfupdate |
| **sanbornm/go-selfupdate** | Abandonware. Original 2014-era library, no Windows support. | minio/selfupdate |
| **inconshreveable/go-update** | Original library, unmaintained since 2015. No Windows `.old` file handling. No `PrepareAndCheckBinary`/`CommitBinary` split. | minio/selfupdate (maintained fork) |
| **goservice/svc** or similar service wrappers | Unnecessary abstraction. Project already runs as a Windows background process with `ShowWindow(SW_HIDE)`. | Keep existing startup pattern, add service restart if needed |
| **go-bindata or go:embed for config** | Config file should be external (YAML). Embedding would prevent runtime config changes. | Keep external config.yaml, embed only static web UI (existing pattern) |
| **Checksum file from GitHub Release** (optional) | GoReleaser can generate checksums.txt, but adds download complexity. Can verify binary hash inline. | SHA256 hash comparison against expected value from Release metadata |

## Stack Patterns by Variant

**If running as background process (current):**
- Use `minio/selfupdate.Apply()` directly
- After apply, notify user that restart is needed
- Process continues running old version until manual restart
- Because: No service manager integration needed, simplest approach

**If running as Windows Service (future consideration):**
- Stop service -> apply update -> start service
- Use `os/exec` to run `net stop <service>` / `net start <service>`
- Because: Windows Service Control Manager must coordinate restart

**If checksum verification needed (recommended):**
- GoReleaser generates `checksums.txt` asset
- Download checksums.txt, extract expected hash for `nanobot-auto-updater_windows_amd64.exe`
- Pass hash to `selfupdate.Options{Checksum: hashBytes}`
- Because: Defense-in-depth against corrupted or tampered downloads

**If private GitHub repository:**
- Set `GITHUB_TOKEN` env var (go-github reads it automatically)
- Or pass `http.Client` with OAuth2 token transport to `github.NewClient()`
- Because: GitHub API rate limits unauthenticated requests (60/hr vs 5000/hr)

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| minio/selfupdate@v0.6.0 | Go 1.24+ | go.mod specifies `go 1.24.0`. Minimal deps: only `aead.dev/minisign` + `golang.org/x/crypto` + `golang.org/x/sys`. |
| google/go-github/v74@v74.0.0 | Go 1.24+ | go.mod in creativeprojects/go-selfupdate confirms v74 works with Go 1.24.11. |
| golang.org/x/sys@v0.41.0 (existing) | Both new libraries | minio/selfupdate needs `x/sys` (already in go.mod at v0.41.0). No conflict. |
| github.com/stretchr/testify@v1.11.1 (existing) | go-github/v74 | No overlap. go-github only depends on `go-querystring` + `oauth2`. |

### Dependency Tree Impact

```
NEW dependencies added to go.mod:
  github.com/minio/selfupdate v0.6.0
    -> aead.dev/minisign v0.2.0        (signature verification, optional)
    -> golang.org/x/crypto v0.43.0     (may upgrade existing v0.x)
    -> golang.org/x/sys v0.37.0        (already have v0.41.0, no downgrade)

  github.com/google/go-github/v74 v74.0.0
    -> github.com/google/go-querystring v1.1.0  (small HTTP helper)
    -> golang.org/x/oauth2 v0.34.0              (for private repo auth)
```

Total new transitive dependencies: ~5 small packages. Acceptable.

## Sources

- **[minio/selfupdate GitHub](https://github.com/minio/selfupdate)** -- Source code analysis of apply.go, hide_windows.go, go.mod. Verified v0.6.0 Windows exe rename pattern. (HIGH confidence)
- **[minio/selfupdate apply.go](https://github.com/minio/selfupdate/blob/master/apply.go)** -- CommitBinary function: rename trick, rollback, Windows hide file fallback. (HIGH confidence)
- **[minio/selfupdate hide_windows.go](https://github.com/minio/selfupdate/blob/master/hide_windows.go)** -- kernel32.dll SetFileAttributesW call for hiding `.old` files on Windows. (HIGH confidence)
- **[minio/selfupdate go.mod](https://github.com/minio/selfupdate/blob/master/go.mod)** -- Go 1.24.0, minimal dependency tree. (HIGH confidence)
- **[creativeprojects/go-selfupdate GitHub](https://github.com/creativeprojects/go-selfupdate)** -- Evaluated and rejected due to naming enforcement and heavy deps. No formal releases published. go.mod inspected. (HIGH confidence)
- **[creativeprojects/go-selfupdate go.mod](https://github.com/creativeprojects/go-selfupdate/blob/main/go.mod)** -- go-github/v74 + gitea + gitlab + semver + xz dependencies confirmed. (HIGH confidence)
- **[google/go-github v68 pkg.go.dev](https://pkg.go.dev/github.com/google/go-github/v68/github)** -- Releases API: GetLatestRelease, ListReleases. API pattern verified. (HIGH confidence)
- **[GoReleaser Official Docs](https://goreleaser.com/customization/builds/)** -- Build configuration for Windows amd64. (HIGH confidence)
- **[GoReleaser GitHub Action](https://github.com/goreleaser/goreleaser-action)** -- CI/CD integration. (HIGH confidence)
- **[inconshreveable/go-update GitHub](https://github.com/inconshreveable/go-update)** -- Original library, evaluated and rejected (unmaintained since 2015). (HIGH confidence)
- **[Stack Overflow: How to self-update application while running](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running)** -- Windows rename pattern confirmation. (MEDIUM confidence)
- **[Reddit: Self-updating binaries current stage](https://www.reddit.com/r/golang/comments/1poccfb/selfupdating_binaries_what_is_current_stage_and/)** -- Community consensus on minio/selfupdate for production use. (LOW confidence)
- **Existing project go.mod** -- Current dependency baseline: golang.org/x/sys v0.41.0, google/uuid v1.6.0, testify v1.11.1. (HIGH confidence)

---
*Stack research for: Self-Update via GitHub Releases*
*Researched: 2026-03-29*
