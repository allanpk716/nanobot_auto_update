# Project Research Summary

**Project:** nanobot-auto-updater v0.8 -- Self-Update via GitHub Releases
**Domain:** Windows Go application self-update via GitHub Releases API
**Researched:** 2026-03-29
**Confidence:** HIGH

## Executive Summary

This project adds self-update capability to a long-running Windows Go background process (nanobot-auto-updater) using GitHub Releases as the update source. The domain is well-traveled: the core challenge is replacing a running `.exe` on Windows, which requires a rename-trick pattern (rename running exe to `.old`, write new exe to original path). Libraries exist that handle this correctly, and the project already has established patterns (atomic concurrency guards, auth middleware, notification integration) that the self-update feature will reuse directly.

The recommended approach uses `minio/selfupdate` for the binary replacement layer and raw `net/http` with manual JSON unmarshaling for GitHub Release discovery. This combination is the lightest-weight option: `minio/selfupdate` provides battle-tested Windows exe replacement with rollback (~400 lines, minimal dependency tree), while a simple HTTP GET to `api.github.com/repos/{owner}/{repo}/releases/latest` avoids pulling in the heavy `google/go-github` library for what is essentially one API call. The feature integrates as a new `internal/selfupdate/` package following the same dependency-injection pattern already used by TriggerHandler.

The primary risk is the restart-after-update sequence: after replacing the binary, the process must gracefully shut down all components (HTTP server, cron, monitors) and self-spawn the new executable. This touches the main shutdown path and must be tested carefully. Secondary risks include GitHub API rate limiting (60 req/hr unauthenticated), checksum verification (must not skip), and concurrent update race conditions (reuse existing atomic.Bool pattern).

## Key Findings

### Recommended Stack

After reconciling disagreement across research files (STACK.md and FEATURES.md recommend `minio/selfupdate`, ARCHITECTURE.md recommends `creativeprojects/go-selfupdate`, PITFALLS.md references the original `inconshreveable/go-update`), the clear winner is `minio/selfupdate` for binary replacement combined with raw `net/http` for GitHub API access.

**Why not `creativeprojects/go-selfupdate`:** It bundles GitHub/GitLab/Gitea source detection, enforces strict asset naming conventions (`{cmd}_{goos}_{goarch}.zip`), and pulls in a heavy dependency tree (go-github + gitea + gitlab + semver + xz). For this single-platform, single-source project, most of its capabilities are unused overhead. Its "automatic rollback on Apply failure" also does not cover the real failure mode: a new binary that starts but behaves incorrectly.

**Why not `google/go-github/v74`:** FEATURES.md correctly identifies that for a single API endpoint (get latest release), the full go-github library is overkill. One struct, one HTTP GET, manual JSON decode. Much lighter than pulling in go-querystring + oauth2 transitive dependencies.

**Why not `inconshreveable/go-update`:** Unmaintained since 2015. No Windows `.old` file handling. `minio/selfupdate` is the maintained fork with Windows-specific improvements.

**Core technologies:**

- **`minio/selfupdate` v0.6.0:** Binary replacement layer -- handles Windows running-exe rename trick, `.old` file hiding via kernel32.dll, rollback on failure. Maintained by MinIO team, 812 stars, battle-tested in production.
- **`net/http` (stdlib):** GitHub API access -- single GET to `/repos/{owner}/{repo}/releases/latest`, manual JSON decode. No external library needed.
- **`crypto/sha256` (stdlib):** Checksum verification -- verify downloaded binary integrity against GoReleaser checksums.txt or GitHub native `asset.digest` (available since June 2025).
- **GoReleaser v2.x + GitHub Actions:** CI/CD pipeline -- build Windows amd64 binary on tag push, produce GitHub Release with checksums. Industry standard for Go binary releases.

### Expected Features

**Must have (table stakes):**
- GitHub Actions CI/CD pipeline triggered on `v*` tag push -- without releases there is nothing to update to
- Version embedding via ldflags (`-X main.Version`) -- already partially exists as `var Version = "dev"` in main.go
- Check for latest GitHub Release -- query API, compare semver against current version
- `POST /api/v1/self-update` HTTP endpoint with Bearer Token auth -- explicit user trigger for update
- Safe binary replacement on Windows -- `minio/selfupdate.Apply()` handles the rename trick
- Backup current exe before replacement -- `Options.OldSavePath` saves old binary alongside new
- Process restart after update -- graceful shutdown + self-spawn new process
- Pushover notification on self-update -- reuse existing `Notifier` interface
- Self-update config section in config.yaml -- `github_owner`, `github_repo` fields

**Should have (differentiators):**
- SHA256 checksum verification -- verify downloaded binary integrity (defense-in-depth)
- `GET /api/v1/self-update/check` -- read-only version check endpoint (safe to call anytime)
- Scheduled self-check via existing cron -- notify user of available updates without auto-applying

**Defer (v2+):**
- Rollback API endpoint (`POST /api/v1/self-update/rollback`) -- requires proven backup management first
- Pre-release channel support -- trivial to add but not needed for MVP
- Download progress reporting via SSE -- adds complexity for small binary downloads

### Architecture Approach

The self-update feature adds one new internal package (`internal/selfupdate/`) and modifies four existing files (main.go, server.go, help.go, config.go). It follows the same dependency-injection pattern already established in the project: components are created in main.go, injected into API handlers via constructors, and tested via locally-defined interfaces. The most significant architectural addition is a shutdown channel (`restartCh chan struct{}`) in main.go that lets the self-update handler signal the main goroutine to begin graceful shutdown and self-spawn.

**Major components:**
1. **`internal/selfupdate/` package** -- SelfUpdater struct with CheckLatest() and CheckAndUpdate() methods, wraps minio/selfupdate library
2. **`internal/api/selfupdate.go`** -- HTTP handlers for check and update endpoints, defines SelfUpdateChecker/SelfUpdateExecutor interfaces
3. **`.github/workflows/release.yml` + `.goreleaser.yaml`** -- CI/CD pipeline producing GitHub Releases with Windows amd64 binary + checksums
4. **main.go modifications** -- Wire SelfUpdater, add restartCh to shutdown select block, self-spawn logic

### Critical Pitfalls

1. **Windows running exe file lock (CRITICAL)** -- Windows locks the running executable for the process lifetime. Cannot overwrite it. Must use rename-trick pattern. Prevention: `minio/selfupdate` handles this internally (rename to `.old`, write new, hide `.old` via kernel32.dll).
2. **Rollback failure leaves no executable (CRITICAL)** -- If rename of new binary fails after old was renamed, no working exe remains. Prevention: always check `RollbackError(err)`, implement startup validation for backup `.exe.old` files, add cleanup on startup.
3. **Concurrent self-update race condition (HIGH)** -- Two simultaneous update requests can corrupt the binary. Prevention: reuse existing `atomic.Bool` pattern, single mutex for ALL update operations (nanobot update + self-update), return HTTP 409 Conflict.
4. **GitHub API rate limiting at 60 req/hr (HIGH)** -- Unauthenticated requests are heavily limited. Prevention: cache release info with TTL (1 hour), log `X-RateLimit-Remaining` headers, default to checking no more than once per hour.
5. **Checksum verification skipped (HIGH)** -- `minio/selfupdate` checksum is optional; skipping it means corrupted or tampered binaries get installed. Prevention: always pass `Options.Checksum`, use GitHub native `asset.digest` (since June 2025) or GoReleaser checksums.txt.

## Library Choice Resolution

The four research files disagreed on library choice. Here is the resolved recommendation with rationale:

| Decision | Resolution | Rationale |
|----------|-----------|-----------|
| Binary replacement library | **`minio/selfupdate` v0.6.0** | ARCHITECTURE.md favored `creativeprojects/go-selfupdate` for its all-in-one approach, but for this project the extra capabilities (multi-source, archive detection, strict naming) are unused overhead. STACK.md and FEATURES.md agree on minio. MinIO's fork is actively maintained, has 812+ stars, minimal dependency tree, and is battle-tested in production. |
| GitHub API client | **Raw `net/http`** | STACK.md recommended `google/go-github/v74` for its type safety and pagination handling. FEATURES.md recommended raw HTTP for its simplicity. For a single endpoint call (get latest release), raw HTTP is the right trade-off: one struct, one GET, no transitive dependencies. If the project later needs more GitHub API interaction, go-github can be added then. |
| Restart strategy | **Self-spawn with port-binding retry** | ARCHITECTURE.md provides the most detailed restart analysis. Option A (self-spawn) is recommended: `exec.Command(exePath, os.Args[1:]...).Start()` then `os.Exit(0)`, using existing `windows.SysProcAttr` pattern. Port conflict mitigation: retry HTTP server binding with backoff (100ms, 200ms, 400ms) on startup. |

## Implications for Roadmap

Based on combined research, suggested phase structure:

### Phase 1: CI/CD Pipeline
**Rationale:** Must be first because it produces the GitHub Releases that subsequent phases will download from. No Go code changes required, can be validated independently by pushing a test tag.
**Delivers:** GoReleaser config + GitHub Actions workflow that produces GitHub Releases with Windows amd64 binary + checksums on `v*` tag push.
**Addresses:** Version embedding (ldflags), release asset production
**Avoids:** Pitfall 8 (workflow not triggering), Pitfall 9 (wrong GOOS/GOARCH), Pitfall 10 (version not injected)
**Build tasks:** `.goreleaser.yaml`, `.github/workflows/release.yml`, verify with test tag push

### Phase 2: Self-Update Core Component + Config
**Rationale:** Core logic must exist before it can be wired into HTTP handlers. Config is small enough to include here.
**Delivers:** `internal/selfupdate/` package with CheckLatest() and CheckAndUpdate() methods. SelfUpdateConfig in config.yaml. Unit tests with mock source.
**Uses:** `minio/selfupdate` v0.6.0, `net/http` for GitHub API, `crypto/sha256` for checksums
**Implements:** GitHub release checker, version comparison (proper semver), binary download + replacement + backup
**Avoids:** Pitfall 1 (Windows file lock -- handled by library), Pitfall 3 (rate limiting -- cache with TTL), Pitfall 5 (corrupted binary -- mandatory checksum), Pitfall 6 (version comparison -- proper semver parsing)

### Phase 3: HTTP API Integration + Notification
**Rationale:** Wire the core component into the existing HTTP server. Follows exact same pattern as TriggerHandler integration (Phase 28). Low risk, well-established patterns.
**Delivers:** `GET /api/v1/self-update/check` and `POST /api/v1/self-update` endpoints with Bearer Token auth. Pushover notifications on update start/complete. Help endpoint updates.
**Implements:** SelfUpdateHandler, route registration in server.go, interface-based mocking for tests
**Avoids:** Pitfall 4 (concurrent update -- atomic.Bool guard)

### Phase 4: Restart Mechanism + Safety Net
**Rationale:** Restart is the riskiest part of the feature (touches main shutdown path). Isolate it for focused testing. Safety features (rollback detection, .old cleanup) belong here.
**Delivers:** Restart channel in main.go, self-spawn logic with `windows.SysProcAttr`, `.old` file cleanup on startup, graceful shutdown integration.
**Implements:** `restartCh chan struct{}` in main.go select block, `restartProcess()` function
**Avoids:** Pitfall 2 (rollback failure -- startup validation), Pitfall 7 (.old accumulation -- startup cleanup), Pitfall 11 (instances during update -- stop before update)

### Phase 5: E2E Validation
**Rationale:** Final validation after all pieces are in place. Tests the complete flow: tag push -> release -> API check -> self-update -> restart -> new version running.
**Delivers:** End-to-end test confirming full self-update lifecycle works.
**Dependencies:** All previous phases

### Phase Ordering Rationale

- CI/CD first because it is independent of all code changes and produces the artifacts the rest of the feature consumes.
- Core component second because both the API handler and restart mechanism depend on the SelfUpdater type existing.
- HTTP API third because it is pure integration glue following well-established project patterns. Low risk.
- Restart mechanism fourth because it touches the critical shutdown path and must be tested in isolation.
- E2E last because it validates the complete pipeline from tag push to running new version.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 4 (Restart Mechanism):** The self-spawn + graceful shutdown interaction is the most complex integration point. Port-binding retry logic needs careful design. May benefit from researching Windows process lifecycle edge cases.
- **Phase 2 (Core Component):** Semver comparison implementation needs a decision: use `golang.org/x/mod/semver` (lightweight stdlib-adjacent) or implement a simpler comparison. The `dev` version default case needs explicit handling.

Phases with standard patterns (skip research-phase):
- **Phase 1 (CI/CD):** GoReleaser + GitHub Actions is extremely well-documented with abundant examples.
- **Phase 3 (HTTP API):** Follows exact same pattern as existing TriggerHandler. No research needed.
- **Phase 5 (E2E):** Integration testing using existing test patterns. No research needed.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Library choices verified against source code, go.mod files, and official documentation. minio/selfupdate Windows behavior confirmed from apply.go and hide_windows.go source. |
| Features | HIGH | Feature list derived from established self-update patterns in the Go ecosystem. All features have clear implementations and existing project patterns to follow. |
| Architecture | HIGH | Follows existing project patterns exactly (dependency injection, interface-based handlers, atomic concurrency). No novel architectural decisions. |
| Pitfalls | HIGH | Windows exe locking is well-documented. GitHub rate limits confirmed from official changelog. All pitfalls have concrete prevention strategies. |

**Overall confidence:** HIGH

### Gaps to Address

- **GitHub repository owner/name:** The config references `HQGroup/nanobot-auto-updater` but this needs confirmation during implementation. Affects both GoReleaser config and default config values.
- **Asset naming convention:** GoReleaser will produce `nanobot-auto-updater_windows_amd64.zip` by default. The download logic must match this exactly. Define and document the convention early.
- **Private vs public repository:** Research assumes a public repository (60 req/hr rate limit). If the repository is private, authentication via GitHub token is required. This affects config design and should be decided before Phase 2.
- **Existing `.old` files:** If any prior updates happened manually, there may be `.old` files in the application directory. The startup cleanup in Phase 4 should handle this gracefully.

## Sources

### Primary (HIGH confidence)
- [minio/selfupdate GitHub](https://github.com/minio/selfupdate) -- Source code analysis of apply.go, hide_windows.go, go.mod. Windows exe rename pattern verified.
- [minio/selfupdate apply.go](https://github.com/minio/selfupdate/blob/master/apply.go) -- CommitBinary function, rename trick, rollback, Windows hide file fallback.
- [creativeprojects/go-selfupdate go.mod](https://github.com/creativeprojects/go-selfupdate/blob/main/go.mod) -- Dependency tree analysis confirming heavy deps (go-github + gitea + gitlab + semver + xz).
- [GoReleaser Official Docs](https://goreleaser.com/customization/builds/) -- Build configuration reference.
- [GitHub Changelog May 2025: Rate limits](https://github.blog/changelog/2025-05-08-updated-rate-limits-for-unauthenticated-requests/) -- 60/hr unauthenticated, 5000/hr authenticated.
- [GitHub Changelog June 2025: Asset digests](https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/) -- Native SHA256 for release assets.
- Existing codebase: internal/api/trigger.go, internal/api/server.go, internal/config/config.go, cmd/nanobot-auto-updater/main.go -- Pattern verification.

### Secondary (MEDIUM confidence)
- [Stack Overflow: Self-update while running](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running) -- Windows rename trick confirmation.
- [Go Forum: Auto restart after self-update](https://forum.golangbridge.org/t/auto-restart-after-self-update/34792) -- Process restart patterns.
- [Rob Allen: GitHub Actions for Go binaries](https://akrabat.com/using-github-actions-to-add-go-binaries-to-a-release/) -- CI/CD patterns.

### Tertiary (LOW confidence)
- [Reddit r/golang: Self-updating binaries](https://www.reddit.com/r/golang/comments/1poccfb/selfupdating_binaries_what_is_current_stage_and/) -- Community consensus on library choices.

---
*Research completed: 2026-03-29*
*Ready for roadmap: yes*
