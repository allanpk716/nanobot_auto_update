# Feature Landscape

**Domain:** Go Windows service self-update via GitHub Releases
**Researched:** 2026-03-29
**Scope:** v0.8 Self-Update milestone ONLY (CI/CD, self-update API, safe binary replacement, backup/rollback)
**Confidence:** HIGH

## Table Stakes

Features users expect for a self-updating Go service. Missing = update feels broken or unsafe.

| Feature | Why Expected | Complexity | Dependencies on Existing | Notes |
|---------|--------------|------------|--------------------------|-------|
| GitHub Actions CI/CD pipeline | Without automated builds, there are no releases to update TO. Tag push must produce a downloadable Windows amd64 binary. | Low | None | GoReleaser is the de facto standard. Simple `.goreleaser.yaml` + one workflow file. Trigger on `v*` tag push. |
| Version embedding via ldflags | Self-update needs to know its own version to compare against latest. Already partially exists (`var Version = "dev"` in main.go). | Low | Uses existing `Version` variable in `cmd/nanobot-auto-updater/main.go` | GoReleaser sets `-ldflags "-X main.Version={{.Version}}"` automatically. No code change needed beyond what already exists. |
| Check for latest release | Core capability: query GitHub API for the newest published release, compare semver against current version. | Medium | None | Use raw `net/http` call to `api.github.com/repos/{owner}/{repo}/releases/latest`. Unauthenticated rate limit is 60 req/hr which is plenty for manual triggers. Avoid heavy `google/go-github` library dependency. |
| Self-update HTTP API endpoint | User triggers update via `POST /api/v1/self-update`. Protected by existing Bearer Token auth. | Medium | Depends on existing `api.Server` router pattern, `AuthMiddleware`, `writeJSONError` helper | Must integrate into existing `NewServer()` mux registration pattern. Follow `TriggerHandler` style: interface for testability, JSON response, non-blocking notification. |
| Safe binary replacement (Windows) | Running `.exe` is locked on Windows. Must rename old binary, write new one to original path. | High | None | `minio/selfupdate` library handles this natively. Uses `os.Rename` pattern: `app.exe` -> `app.exe.old`, new file -> `app.exe`. Works because Windows allows renaming locked executables. |
| Backup current exe before replacement | If update fails, user needs a way back. Backup old exe alongside new one. | Medium | None | `minio/selfupdate.Apply()` supports `Options.OldSavePath` to save the old binary before replacement. Simple: save to `nanobot-auto-updater.exe.bak` in same directory. |
| Rollback on failure | If new version crashes or fails health check, restore backup exe automatically. | High | Depends on backup feature, may depend on process restart mechanism | Two strategies: (A) Crash-detection: watchdog detects crash and restores backup before restart. (B) Pre-commit: run a quick health check after update, rollback if it fails. Strategy B is simpler and more reliable. |
| Process restart after update | After binary swap, the new version must start running. Current process exits, new process launches. | Medium | Depends on how main.go handles shutdown | Standard Go pattern: `exec.Command(os.Args[0], os.Args[1:]...).Start()` then `os.Exit(0)`. Must ensure graceful shutdown of existing HTTP server, cron, monitors first. |
| Pushover notification for self-update | User needs to know when the updater updates itself. Follows existing notification pattern. | Low | Depends on existing `notifier.Notifier` interface | Reuse existing `Notifier` interface. Send "self-update started" and "self-update completed/failed" notifications via Pushover. Follow same async + panic recovery pattern as `TriggerHandler`. |
| Configuration for self-update source | User must configure which GitHub repo to check for updates (owner/repo). | Low | Depends on existing `config.Config` struct with viper/YAML | Add `self_update` section to `config.yaml`: `github_owner`, `github_repo`, optionally `enabled` flag. Follow existing viper + mapstructure pattern. |

## Differentiators

Features that elevate the self-update experience. Not expected, but valued.

| Feature | Value Proposition | Complexity | Dependencies on Existing | Notes |
|---------|-------------------|------------|--------------------------|-------|
| SHA256 checksum verification | Verify downloaded binary integrity. Prevents corrupted or tampered updates. | Low | None | GoReleaser generates `checksums.txt` automatically. `creativeprojects/go-selfupdate` has built-in `ChecksumValidator`. `minio/selfupdate` supports checksums via `Options.Checksum` or `Options.Signature`. |
| Update status query API | `GET /api/v1/self-update/status` returns current version, latest available version, last update timestamp. | Medium | Depends on self-update API infrastructure | Useful for monitoring dashboards and automation. Low cost to add alongside the update trigger endpoint. |
| Scheduled self-check | Periodically check for new versions (e.g., daily via cron). Notify user but do not auto-apply. | Medium | Depends on existing `robfig/cron` scheduler in main.go | Add a cron job like `0 4 * * *` that checks GitHub and sends Pushover notification if new version available. User triggers actual update manually via API. |
| Rollback API endpoint | `POST /api/v1/self-update/rollback` explicitly restores the backup exe. | Medium | Depends on backup feature | Useful when user discovers a problem with new version but the process did not crash. Requires that backup file still exists. |
| Download progress reporting | During update, report download progress via SSE or in the API response. | Medium | Depends on existing SSE infrastructure | Existing SSE pattern could stream download progress to connected clients. Adds complexity to the update flow. |
| Pre-release channel support | Allow users to opt into pre-release versions for testing. | Low | Depends on GitHub release API | GitHub API supports `prerelease` flag. Add `self_update.prerelease: true` config option. Trivial to implement once release checking is built. |

## Anti-Features

Features to explicitly NOT build. These are traps that add complexity without proportional value.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Auto-apply self-updates without user consent | Unattended updates of the updater itself is risky. A bad update could brick the monitoring service. | Always require explicit API trigger. Consider scheduled CHECK with notification, but manual APPLY. |
| Full `google/go-github` library dependency | The full `go-github` library is large (many types, transitive dependencies). For a single API call (get latest release), it is overkill. | Use a simple `net/http` call to `api.github.com/repos/{owner}/{repo}/releases/latest` with manual JSON unmarshaling. One struct, one HTTP GET. Much lighter. |
| Code signing verification (minisign) | Adds significant complexity (key management, signing pipeline). Not needed for internal tool. | SHA256 checksum verification is sufficient. If GoReleaser produces `checksums.txt`, validate against that. |
| Binary patching (bsdiff) | Only valuable for very large binaries or bandwidth-constrained environments. nanobot-auto-updater is likely under 20MB. | Full binary download is simpler and more reliable. Patching adds failure modes. |
| Multi-platform release support | Project is Windows-only (stated constraint). Building for Linux/macOS adds CI complexity with zero users. | GoReleaser config targets `windows/amd64` only. Keep it simple. |
| Docker/container-based update | Project runs as a native Windows exe, not in containers. | Native binary replacement is the correct approach for this project. |
| Update UI in web dashboard | The existing web UI is for log viewing. Adding update UI mixes concerns and increases scope significantly. | API-only update trigger. User uses curl, scripts, or monitoring tools to trigger updates. |
| Concurrent update protection at OS level (mutex, lock file) | Over-engineering. The existing `Atomic.Bool` pattern from trigger-update can be reused. | Reuse the `sync/atomic` concurrency control pattern already established in the codebase. |

## Feature Dependencies

```
GitHub Actions CI/CD (standalone, no code deps)
    |
    v
Config for self-update source (config.yaml)
    |
    v
Check for latest release (GitHub API)
    |
    +----> Self-update HTTP API endpoint (trigger)
    |           |
    |           +----> Safe binary replacement (minio/selfupdate)
    |           |           |
    |           |           +----> Backup current exe
    |           |           |
    |           |           +----> Process restart
    |           |                    |
    |           |                    +----> Rollback on failure
    |           |
    |           +----> Pushover notification
    |
    +----> Update status query API (optional differentiator)
    |
    +----> Scheduled self-check (optional differentiator)
```

## Library Comparison for Self-Update

Based on research, three viable approaches exist:

| Criterion | minio/selfupdate | creativeprojects/go-selfupdate | Raw HTTP + self-apply |
|-----------|------------------|-------------------------------|----------------------|
| Stars/maintainers | 812 stars, MinIO team | 400+ stars, active community | N/A (custom code) |
| Windows support | Yes (native) | Yes (tested) | Must handle manually |
| Backup old binary | Yes (`OldSavePath`) | Yes (built-in rollback) | Must implement manually |
| Rollback on failure | Manual (restore backup) | Built-in automatic | Must implement manually |
| GitHub Release integration | None (provides Apply only) | Built-in (Detect, Download, Apply) | Must implement manually |
| Checksum verification | Yes | Yes (ChecksumValidator) | Must implement manually |
| Dependency weight | Minimal | Moderate | Zero |
| Release cadence | v0.6.0 (Jan 2023), stable and battle-tested by MinIO | v1.x, actively maintained | N/A |

**Recommendation:** Use `minio/selfupdate` for the binary replacement layer (battle-tested by MinIO, minimal dependencies, Windows-native) combined with a simple raw HTTP call to GitHub API for release checking. This gives the best balance of reliability and simplicity.

Why NOT `creativeprojects/go-selfupdate` despite its richer feature set:
- It bundles GitHub/GitLab/Gitea source detection, archive extraction, version parsing -- much of which is unnecessary for this single-platform, single-source project
- The added dependency tree is heavier
- Its rollback is "automatic on Apply failure" which does not cover the case where the new binary starts but behaves incorrectly (crashes later)

The manual rollback approach (backup file + health check + explicit restore) is more robust for the actual failure mode of a service.

## MVP Recommendation

**Phase 1 (CI/CD -- no code changes needed):**
1. GoReleaser config targeting `windows/amd64`
2. GitHub Actions workflow triggered on `v*` tag push
3. Produces GitHub Release with binary + checksums

**Phase 2 (Self-update core):**
1. Config for GitHub owner/repo (`self_update` section in config.yaml)
2. GitHub latest release checker (raw HTTP, simple JSON decode)
3. `POST /api/v1/self-update` endpoint with Bearer Token auth
4. Binary replacement via `minio/selfupdate.Apply()`
5. Backup current exe via `Options.OldSavePath`
6. Process restart (`exec.Command` + `os.Exit`)
7. Pushover notifications (reuse existing Notifier)

**Phase 3 (Safety net):**
1. Post-update health verification
2. Automatic rollback on crash detection (restore backup exe)

**Defer:**
- SHA256 checksum verification: Nice-to-have, add in follow-up. Download corruption is rare over HTTPS.
- Scheduled self-check: Requires cron integration. Add after core update works.
- Rollback API endpoint: Requires backup file management. Add after backup/rollback is proven.
- Pre-release channel: Trivial addition but not needed for MVP.
- Download progress reporting: Adds SSE complexity. Not needed for small binaries.

## Integration Points with Existing Code

| New Feature | Existing Component | Integration Pattern |
|-------------|-------------------|---------------------|
| Self-update handler | `api.Server.NewServer()` | Add new handler to mux, wrapped in `AuthMiddleware` |
| Self-update logic | New `internal/selfupdate/` package | Separate from existing `internal/updater/` (nanobot update logic) |
| Version check | `main.go` `Version` variable | Compare `Version` against GitHub release tag |
| Config | `config.Config` struct | Add `SelfUpdate SelfUpdateConfig` field with mapstructure tags |
| Notification | `notifier.Notifier` interface | Reuse existing interface, inject into self-update handler |
| Concurrency control | `sync/atomic.Bool` pattern (from TriggerHandler) | Add separate `selfUpdateInProgress` atomic flag in self-update handler |
| Graceful restart | `main.go` shutdown sequence | Call `apiServer.Shutdown()` + cleanup, then `exec.Command` new process |
| Update log | `updatelog.UpdateLogger` | Record self-update operations in same JSONL log for audit trail |

## Sources

- [minio/selfupdate](https://github.com/minio/selfupdate) -- Primary library recommendation (HIGH confidence, verified from GitHub README)
- [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) -- Alternative considered, richer but heavier (HIGH confidence, verified from GitHub README)
- [GoReleaser GitHub Actions docs](https://goreleaser.com/customization/ci/actions/) -- CI/CD pattern (HIGH confidence, official docs)
- [GoReleaser Quick Start](https://goreleaser.com/getting-started/quick-start/) -- Configuration reference (HIGH confidence, official docs)
- [Windows exe locking workaround](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running) -- Rename-then-replace pattern (HIGH confidence, multiple sources agree)
- [Auto restart after self-update](https://forum.golangbridge.org/t/auto-restart-after-self-update/34792) -- Process restart pattern (MEDIUM confidence, community discussion)
- [GoReleaser .goreleaser.yaml example](https://github.com/goreleaser/goreleaser/blob/main/.goreleaser.yaml) -- Real-world config reference (HIGH confidence, source repo)
- [GitHub REST API rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) -- 60/hr unauthenticated, 5000/hr authenticated (HIGH confidence, official docs)
- [google/go-github](https://github.com/google/go-github) -- Considered and rejected for being overkill (HIGH confidence, verified)

---

*Feature research for: v0.8 Self-Update milestone*
*Researched: 2026-03-29*
