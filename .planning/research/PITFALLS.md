# Domain Pitfalls: Self-Update via GitHub Releases

**Domain:** Adding self-update capability to a Windows Go application (nanobot-auto-updater)
**Researched:** 2026-03-29
**Overall confidence:** HIGH (verified with library source code and official docs)

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or bricked installations.

---

### Pitfall 1: Windows Running Executable File Lock

**What goes wrong:**
On Windows, a running `.exe` is memory-mapped by the OS and cannot be overwritten or deleted. Attempting `os.Rename(newExe, currentExe)` while the process is running will fail with "Access Denied" or "The process cannot access the file because it is being used by another process."

**Why it happens:**
- Windows locks the executable file for the entire lifetime of the process (not just during startup)
- Unlike POSIX systems, Windows does not allow unlinking an open file
- `go-update` works around this by renaming the running exe (which IS allowed) to `.exe.old`, then renaming the new exe into place

**Consequences:**
- Update silently fails, application stays on old version
- If retry logic is wrong, infinite retry loop consuming resources
- Partial replacement leaves filesystem in inconsistent state

**Prevention:**
Use `go-update` library which implements the correct Windows pattern internally:
1. Write new binary to `.exe.new`
2. Rename running `.exe` to `.exe.old` (Windows allows this even for running executables)
3. Rename `.exe.new` to original `.exe` name
4. On Windows, `.exe.old` cannot be deleted (still locked), so it is hidden instead

**Detection:**
- Log the result of every rename operation
- If `os.Rename` returns an access-denied error, the process is likely still holding the file
- Windows error code 32 (ERROR_SHARING_VIOLATION) indicates file-in-use

**Phase to address:** Phase implementing self-update binary replacement (core update logic)

**Sources:**
- [go-update apply.go source](https://github.com/inconshreveable/go-update/blob/master/apply.go) -- Read full source, confirms rename-old/rename-new pattern (HIGH confidence)
- [Stack Overflow: How to self-update application while running](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running) -- Confirms rename trick (HIGH confidence)
- [SuperUser: Why can I rename a running executable but not delete it](https://superuser.com/questions/488127/why-can-i-rename-a-running-executable-but-not-delete-it) -- Windows OS behavior explanation (HIGH confidence)

---

### Pitfall 2: Rollback Failure Leaves No Executable (Bricked Install)

**What goes wrong:**
The update process renames `app.exe` to `app.exe.old`, then tries to rename `app.new.exe` to `app.exe`. If the second rename fails (disk full, permission error, crash, power loss), there is NO `app.exe` at the expected path. The application cannot restart, and the rollback code never runs because the process has already exited or crashed.

**Why it happens:**
- `go-update`'s Apply() function handles this internally and attempts rollback
- But rollback itself can fail (double failure), leaving the filesystem with only `.exe.old`
- Power loss or process crash between step 2 and step 3 has no recovery path
- The `RollbackError(err)` function exists specifically to detect this double-failure case

**Consequences:**
- Application cannot start at all after update attempt
- Manual intervention required (rename `.exe.old` back to `.exe`)
- If running as a service, the service fails to start on every reboot
- Users may not know how to recover

**Prevention:**
1. Always use `go-update`'s `OldSavePath` option to keep the old binary at a known backup location (not just the default `.exe.old`)
2. After `Apply()`, ALWAYS check `RollbackError(err)` -- this is the only way to detect the double-failure state
3. Implement a startup validation check: on launch, check if a backup `.exe.bak` exists alongside the running exe. If the current version is invalid, the backup can be restored
4. Consider adding a minimal "watchdog" batch script or separate process that checks if the main exe starts successfully, and restores from backup if not

**Detection:**
- `RollbackError(err)` returns non-nil when both the update and rollback failed
- Application fails to start entirely after update
- Service recovery shows "executable not found" error

**Phase to address:** Phase implementing backup/rollback logic

**Sources:**
- [go-update apply.go source](https://github.com/inconshreveable/go-update/blob/master/apply.go) -- RollbackError type and double-failure handling documented in comments (HIGH confidence)
- [cosmofy Issue #58: self-update overwrites running executable](https://github.com/metaist/cosmofy/issues/58) -- Real-world report of this failure mode (MEDIUM confidence)

---

### Pitfall 3: GitHub API Rate Limiting (60 req/hr Unauthenticated)

**What goes wrong:**
The self-update check endpoint calls `api.github.com/repos/{owner}/{repo}/releases/latest` to find the latest version. Without authentication, GitHub allows only 60 requests per hour per IP address. If the check is called too frequently (health endpoint polling, multiple instances behind same NAT), the API returns 403 and update checks fail silently.

**Why it happens:**
- GitHub unauthenticated rate limit is 60 requests/hour per originating IP (confirmed May 2025)
- The application may have multiple components checking for updates independently
- Other tools on the same machine may also consume GitHub API quota
- No error handling for 403 rate-limit responses leads to "update check failed, no new version" being treated as "already up to date"

**Consequences:**
- Self-update feature silently stops working
- Users unaware that updates are available
- Debugging is difficult (no visible error, just no update detected)

**Prevention:**
1. Cache the latest release info locally with a TTL (e.g., check once per hour maximum, cache in a local file)
2. Add explicit error handling for HTTP 403 responses -- log rate limit remaining headers (`X-RateLimit-Remaining`)
3. Consider adding optional GitHub token support in config for higher limits (5000/hr authenticated)
4. The existing `cron` scheduler (already in the project) can control check frequency
5. Default to checking no more than once per hour, make the interval configurable

**Detection:**
- Log `X-RateLimit-Remaining` header from every GitHub API response
- If remaining count drops below 10, log a warning
- If response is 403 with rate-limit headers, log an error with reset time

**Phase to address:** Phase implementing GitHub release checker / version comparison

**Sources:**
- [GitHub API Rate Limits - Community Discussion #170662](https://github.com/orgs/community/discussions/170662) -- Confirms 60/hr unauthenticated, 5000/hr authenticated (HIGH confidence)
- [GitHub Changelog May 2025: Updated rate limits](https://github.blog/changelog/2025-05-08-updated-rate-limits-for-unauthenticated-requests/) -- Official announcement (HIGH confidence)

---

### Pitfall 4: Concurrent Self-Update Requests Race Condition

**What goes wrong:**
Two API calls to the self-update endpoint arrive simultaneously. Both check "am I on latest version?", both see "no", both download the new binary, both attempt to replace the running executable. The second replacement can interfere with the first, leading to corruption or the second attempt failing because the file layout changed mid-operation.

**Why it happens:**
- The project already uses `sync/atomic.Bool` for `updateInProgress` in the nanobot trigger-update handler (Phase 28)
- But the self-update endpoint is a NEW endpoint with its OWN concurrency control
- If the concurrency guard is forgotten or uses a different lock than the nanobot update guard, both operations could run simultaneously

**Consequences:**
- Two downloads of the same binary wasting bandwidth
- File corruption during replacement
- One update overwriting the other's backup file
- Inconsistent state (which version is actually running?)

**Prevention:**
1. Reuse the existing `Atomic.Bool` pattern from `internal/instance` for self-update guard
2. Consider: should nanobot update and self-update be mutually exclusive? (yes -- both may stop/start instances)
3. Use a SINGLE mutex for ALL update operations (nanobot update + self-update)
4. Return HTTP 409 Conflict immediately if another update is in progress (existing pattern from trigger-update handler)

**Detection:**
- `go test -race` catches data races in tests
- Log warnings when concurrent access is detected
- API returns 409 status code to caller

**Phase to address:** Phase implementing self-update HTTP API endpoint

**Sources:**
- Existing codebase: `internal/api/trigger.go` line ~106 already returns 409 for concurrent nanobot updates (HIGH confidence)
- Existing codebase: `internal/instance` uses `sync/atomic.Bool` for update guard (HIGH confidence)

---

### Pitfall 5: Download Verification Failure (Corrupted Binary)

**What goes wrong:**
The downloaded binary is corrupted (network error, partial download, man-in-the-middle) and the application replaces itself with a broken executable. The new version crashes on startup, and depending on the backup strategy, the user may have lost the working version.

**Why it happens:**
- Network interruptions during download can leave partial files
- DNS spoofing or MITM attacks could serve a malicious binary
- GitHub CDN redirects can fail mid-stream
- `go-update` does checksum verification but ONLY if `opts.Checksum` is provided -- it is optional, not mandatory

**Consequences:**
- Application replaced with corrupted binary
- Service enters crash loop
- Without proper backup, manual recovery required
- In worst case: malicious binary executed

**Prevention:**
1. ALWAYS pass `opts.Checksum` to `go-update`'s `Apply()` -- never skip verification
2. As of June 2025, GitHub Releases API returns `asset.digest` (SHA256) for every uploaded asset -- use this as the checksum source
3. If `asset.digest` is unavailable (older releases), also upload a `checksums.txt` file as a release asset
4. Verify checksum BEFORE calling `Apply()`, not after (go-update does this internally when Checksum is provided)
5. Download to a temp file first, verify hash, then pass to `Apply()`

**Detection:**
- `go-update` returns checksum mismatch error immediately
- Log the expected vs actual hash on failure
- Never proceed with update if hash verification fails

**Phase to address:** Phase implementing download + verification logic

**Sources:**
- [go-update doc.go](https://github.com/inconshreveable/go-update/blob/master/doc.go) -- "go-update validates SHA256 checksums by default" when Checksum is provided (HIGH confidence)
- [GitHub Changelog June 2025: Releases now expose digests](https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/) -- Official feature announcement (HIGH confidence)
- [GitHub Community Discussion #23512: Release checksums](https://github.com/orgs/community/discussions/23512) -- How to retrieve checksums via API (HIGH confidence)

---

### Pitfall 6: Version Comparison Logic Error

**What goes wrong:**
The application checks the latest GitHub Release tag and decides whether to update. String comparison of version numbers (`"v0.9" > "v0.10"` is TRUE in lexicographic order) leads to skipping valid updates or unnecessarily re-downloading the same version.

**Why it happens:**
- Lexicographic string comparison does not work for semantic versions
- Tag format may vary (`v0.8.0` vs `0.8.0` vs `v0.8`)
- Pre-release versions need special handling (`v0.9.0-alpha` vs `v0.9.0`)
- The `Version` variable in `main.go` is set via ldflags at build time -- if not set, it defaults to `"dev"`, which will always compare incorrectly

**Consequences:**
- Self-update never triggers (thinks it's already up to date)
- Self-update triggers every check (thinks it needs to update)
- Pre-release versions installed on production systems

**Prevention:**
1. Use `creativeprojects/go-selfupdate` which has built-in semver comparison, OR implement proper semver parsing with `golang.org/x/mod/semver`
2. Strip the `v` prefix before comparison
3. Always set `Version` via ldflags in CI/CD (`-X main.Version={{tag}}`)
4. Handle the `"dev"` case: if current version is `"dev"`, always allow update
5. Only consider full releases, not pre-releases (GitHub API supports `prerelease` flag)

**Detection:**
- Unit tests with version comparison edge cases: v0.9.0 vs v0.10.0, v0.8.0 vs v0.8.0
- Log the comparison result: "current=v0.7.0, latest=v0.8.0, update_needed=true"

**Phase to address:** Phase implementing version checker

**Sources:**
- [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) -- Built-in semver comparison (HIGH confidence)
- [Existing codebase: cmd/nanobot-auto-updater/main.go line 28](file://cmd/nanobot-auto-updater/main.go) -- `var Version = "dev"` (HIGH confidence)

---

## Moderate Pitfalls

### Pitfall 7: Windows `.exe.old` File Accumulation

**What goes wrong:**
After each successful self-update, Windows cannot delete the old `.exe.old` file because the process is still running. `go-update` hides it instead. Over time, hidden `.old` files accumulate in the application directory, consuming disk space. After many updates, this could be significant.

**Prevention:**
1. On application startup, check for `.exe.old` or `.exe.bak` files and attempt cleanup
2. The old file can be deleted on startup because the process is now running from the NEW binary, and the OLD binary is no longer in use
3. Implement startup cleanup: `os.Remove(exePath + ".old")` during initialization

**Sources:**
- [go-update apply.go source](https://github.com/inconshreveable/go-update/blob/master/apply.go) -- "On Windows, the removal of /path/to/target.old always fails, so instead Apply hides the old file" (HIGH confidence)
- [go-update Issue #1: old exe not deleted on Windows](https://github.com/inconshreveable/go-update/issues/1) -- Known issue (HIGH confidence)

---

### Pitfall 8: GitHub Actions Workflow Not Triggering on Tag Push

**What goes wrong:**
Developer pushes a tag (`git tag v0.8.0 && git push --tags`) but the GitHub Actions workflow never runs. The release binary is never built or published.

**Why it happens:**
- `on.push.tags` pattern does not match the tag format (e.g., using `v*` but pushing `0.8.0`)
- Tag pushed by another workflow (bot) does not re-trigger workflows (GitHub anti-loop protection)
- Missing `permissions: contents: write` in workflow YAML
- Using `on.release` instead of `on.push.tags` -- release must be created separately first

**Prevention:**
1. Use `on.push.tags: ['v*']` pattern (matches `v0.8.0`, `v0.9.0`, etc.)
2. Add `permissions: contents: write` to workflow
3. Test the workflow with a `workflow_dispatch` trigger before relying on tag push
4. Always push tags from local machine (`git tag v0.8.0 && git push origin v0.8.0`), not from CI

**Sources:**
- [GitHub Community Discussion #27028: Workflow not triggering with tag push](https://github.com/orgs/community/discussions/27028) -- Common issue with tag triggers (HIGH confidence)
- [GitHub Docs: Events that trigger workflows](https://docs.github.com/actions/using-workflows/events-that-trigger-workflows) -- Official docs (HIGH confidence)

---

### Pitfall 9: Missing `GOOS`/`GOARCH` in CI Build Step

**What goes wrong:**
GitHub Actions runs on Linux runners by default. Without explicitly setting `GOOS=windows GOARCH=amd64`, the build produces a Linux binary. The binary is uploaded to GitHub Release, downloaded by the self-updater, and the application crashes because it is not a Windows executable.

**Why it happens:**
- Default runner OS is `ubuntu-latest`
- `go build` without env vars builds for the host platform
- The binary LOOKS valid (same size, no obvious corruption) but won't run on Windows
- No validation step in the workflow to verify the binary is a Windows PE executable

**Prevention:**
1. Explicitly set `GOOS: windows` and `GOARCH: amd64` as env vars in the build step
2. Add a validation step: `file nanobot-auto-updater.exe` should show "PE32+ executable"
3. Consider using GoReleaser which handles cross-compilation automatically
4. For this project (single target: Windows amd64), manual `go build` with explicit env vars is simpler than GoReleaser

**Sources:**
- [Rob Allen: Using GitHub Actions to add Go binaries to a release](https://akrabat.com/using-github-actions-to-add-go-binaries-to-a-release/) -- Practical guide (MEDIUM confidence)
- [GoReleaser vs Manual Build Discussion](https://www.reddit.com/r/golang/comments/zwb5zl/is_goreleaser_still_the_way_to_deploy_a_binary_to/) -- Community consensus (MEDIUM confidence)

---

### Pitfall 10: Version Not Injected via ldflags in CI Build

**What goes wrong:**
The GitHub Actions workflow compiles the binary without setting the `Version` variable via ldflags. The binary reports version `"dev"` (the default in `main.go`). The self-update checker compares `"dev"` against GitHub Release versions and the comparison logic breaks or always triggers updates.

**Why it happens:**
- The `go build` command in CI does not include `-ldflags "-X main.Version=v0.8.0"`
- Developer forgets to extract the version from the git tag
- The tag name format differs from what the ldflags extraction expects

**Prevention:**
1. In the workflow, extract the tag name: `VERSION=${GITHUB_REF_NAME}` (available in tag-triggered workflows)
2. Pass to build: `go build -ldflags "-X main.Version=$VERSION" -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater`
3. Add a post-build verification step: run `./nanobot-auto-updater.exe --version` and check output matches the tag
4. Handle the `"dev"` default in version comparison: treat `"dev"` as always needing update

**Sources:**
- Existing codebase: `cmd/nanobot-auto-updater/main.go` line 28 -- `var Version = "dev"` (HIGH confidence)
- Existing codebase: `Makefile` or `build.ps1` may already have ldflags patterns (MEDIUM confidence)

---

### Pitfall 11: Self-Update Triggers During Nanobot Instance Operation

**What goes wrong:**
Self-update is triggered while nanobot instances are running and being managed. The updater replaces its own binary, then needs to restart. During restart, nanobot instances lose their parent process and may be left in an inconsistent state (running but unmanaged, or killed unexpectedly).

**Why it happens:**
- Self-update and nanobot instance management share the same process
- Restarting the updater means all goroutines (health monitor, network monitor, instance management) stop
- Instances started via `cmd /c` may or may not survive parent process exit depending on how they were launched

**Prevention:**
1. Before self-update, stop all managed nanobot instances gracefully (reuse existing `StopAllInstances` logic)
2. Perform the self-update binary replacement
3. Restart the updater process (which will auto-start instances on boot via existing Phase 24 logic)
4. Add a "maintenance mode" flag to prevent new operations during self-update
5. Document that self-update causes a brief downtime of instance management

**Sources:**
- Existing codebase: `internal/instance` -- `StartAllInstances` / `StopAllInstances` already exist (HIGH confidence)
- Existing codebase: `cmd/nanobot-auto-updater/main.go` -- Auto-start on launch is already implemented (HIGH confidence)

---

## Minor Pitfalls

### Pitfall 12: Antivirus/SmartScreen Blocking New Binary

**What goes wrong:**
Windows Defender or SmartScreen flags the newly downloaded binary as unrecognized and blocks execution. The self-updater replaces the exe, restarts, but the new version is quarantined or blocked.

**Prevention:**
- Log antivirus interference explicitly
- Consider code signing the binary (long-term)
- Document that users may need to whitelist the application
- The rollback mechanism should detect "binary quarantined" and restore from backup

---

### Pitfall 13: GitHub Release Asset Naming Convention Mismatch

**What goes wrong:**
The self-updater looks for an asset named `nanobot-auto-updater-windows-amd64.exe` in the GitHub Release, but the CI workflow uploads it as `nanobot-auto-updater.exe` or `nanobot-auto-updater_windows_x86_64.exe`. The download fails because the expected asset name does not exist.

**Prevention:**
1. Define the asset naming convention early and document it
2. Use the same naming pattern in both CI workflow (upload) and self-update checker (download)
3. Consider searching release assets by pattern rather than exact name
4. Test the full CI-to-self-update cycle end-to-end

---

### Pitfall 14: Process Restart After Self-Update

**What goes wrong:**
After replacing the binary, the application needs to restart to use the new version. Using `exec.Command(os.Args[0])` to relaunch itself may fail because the old process's binary path now points to the new binary. On Windows, the old process is still running from the old (renamed) binary, but `os.Args[0]` may point to the new name.

**Prevention:**
1. Use the existing binary path (from `os.Executable()`) to find the new binary location
2. Start the new process BEFORE exiting the old one
3. Ensure the new process starts successfully before the old process exits
4. Consider: `exec.Command(exePath, os.Args[1:]...)` where `exePath` is resolved from `os.Executable()`
5. On Windows, the new process can start while the old process is still running (different binary files)

**Sources:**
- [Go Forum: Auto restart after self-update](https://forum.golangbridge.org/t/auto-restart-after-self-update/34792) -- Community discussion with code examples (MEDIUM confidence)

---

### Pitfall 15: Config File Compatibility Across Versions

**What goes wrong:**
New version of the updater expects different config fields or structure. After self-update, the application fails to start because the existing `config.yaml` does not match the new version's expectations.

**Prevention:**
1. Maintain config backward compatibility (project already does this well with legacy mode detection)
2. Add new config fields with sensible defaults (never required without migration)
3. Test new version against OLD config files in CI
4. Document any config changes in release notes

**Sources:**
- Existing codebase: `internal/config` -- Already implements backward compatibility (HIGH confidence)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation | Severity |
|-------------|---------------|------------|----------|
| GitHub Actions CI/CD workflow | Missing `GOOS=windows GOARCH=amd64` (Pitfall 9) | Explicit env vars in build step + `file` validation | HIGH |
| GitHub Actions CI/CD workflow | Version not injected via ldflags (Pitfall 10) | Extract tag name, pass to `-ldflags`, verify output | HIGH |
| GitHub Actions CI/CD workflow | Workflow not triggering on tag push (Pitfall 8) | Use `on.push.tags: ['v*']`, test with `workflow_dispatch` | HIGH |
| GitHub Release checker | API rate limiting at 60 req/hr (Pitfall 3) | Cache release info with 1hr TTL, log rate limit headers | HIGH |
| GitHub Release checker | Version comparison logic error (Pitfall 6) | Use semver library, handle "dev" default | HIGH |
| Self-update binary replacement | Windows file lock on running exe (Pitfall 1) | Use `go-update` library which handles this correctly | CRITICAL |
| Self-update binary replacement | Rollback failure leaves no exe (Pitfall 2) | Use `OldSavePath`, check `RollbackError`, startup cleanup | CRITICAL |
| Self-update binary replacement | Download verification failure (Pitfall 5) | Always pass Checksum to `Apply()`, use `asset.digest` | CRITICAL |
| Self-update HTTP API | Concurrent update race condition (Pitfall 4) | Reuse `Atomic.Bool` pattern, mutual exclusion with nanobot update | HIGH |
| Self-update HTTP API | Process restart after update (Pitfall 14) | Use `os.Executable()` for path, start new before exit | MEDIUM |
| Backup/rollback | `.exe.old` file accumulation (Pitfall 7) | Cleanup old files on startup | LOW |
| Backup/rollback | Self-update during instance operation (Pitfall 11) | Stop instances first, restart after update | MEDIUM |
| Integration | Antivirus blocking new binary (Pitfall 12) | Document, log, rollback on failure | LOW |
| Integration | Asset naming mismatch (Pitfall 13) | Document naming convention, test full cycle | MEDIUM |
| Integration | Config incompatibility (Pitfall 15) | Backward-compatible config, test with old config | LOW |

## Integration Gotchas

Mistakes when connecting self-update to the existing system.

| Integration Point | Common Mistake | Correct Approach |
|-------------------|----------------|------------------|
| Self-update + nanobot update | Both run simultaneously | Single mutex for ALL update operations (both types) |
| Self-update + instance management | Update without stopping instances | Stop all instances before self-update, auto-start after restart |
| Self-update API + auth | New endpoint without auth | Reuse existing Bearer Token auth middleware from Phase 28 |
| Self-update + notifications | No notification on self-update | Reuse existing Pushover notifier for self-update success/failure |
| Self-update + update logs | Self-update not logged | Record self-update in existing UpdateLog system |
| CI/CD + version injection | Build without ldflags | Always pass `-ldflags "-X main.Version=$TAG"` |
| CI/CD + checksums | No checksum uploaded as release asset | Use GitHub native `asset.digest` (since June 2025) or upload `checksums.txt` |
| CI/CD + asset naming | Binary name mismatch between upload and download | Define and document naming convention: `nanobot-auto-updater-windows-amd64.exe` |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Self-update replaces binary** -- But does it verify checksum? Without `opts.Checksum`, go-update skips verification
- [ ] **Rollback restores old version** -- But does it check `RollbackError(err)`? The double-failure case is silent without this check
- [ ] **CI builds and uploads binary** -- But is `GOOS=windows GOARCH=amd64` set? Linux runners build Linux binaries by default
- [ ] **CI sets version via ldflags** -- But does `--version` output match the tag? Verify post-build
- [ ] **Self-update API works** -- But is it protected by Bearer Token auth? Unprotected endpoint = anyone can trigger update
- [ ] **Self-update checks for new version** -- But is it rate-limited? GitHub 60/hr limit can be hit quickly
- [ ] **Self-update restarts process** -- But do instances survive? Stop instances before self-update, auto-start after restart
- [ ] **Old binary cleaned up** -- But on Windows, `.exe.old` persists until next startup. Cleanup must happen on startup, not after update
- [ ] **Workflow triggers on tag push** -- But does `on.push.tags` pattern match? `v*` vs `V*` vs `*`

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Bricked install (no exe) | CRITICAL | 1. Locate `.exe.old` or `.exe.bak` in application directory<br>2. Rename back to original exe name<br>3. Start application<br>4. If backup missing: download manually from GitHub Releases |
| Corrupted binary (failed checksum) | LOW | 1. Update was rejected before replacement<br>2. Check network connectivity<br>3. Retry update<br>4. No recovery needed -- old binary untouched |
| Rate limited (GitHub 403) | LOW | 1. Wait for rate limit reset (check `X-RateLimit-Reset` header)<br>2. Add GitHub token to config for higher limits<br>3. Increase check interval |
| Concurrent update conflict | LOW | 1. API returns 409 immediately<br>2. Caller retries after current update completes<br>3. No data loss |
| CI build missing GOOS | MEDIUM | 1. Add env vars to workflow<br>2. Delete incorrect release asset<br>3. Re-push tag or manually trigger workflow |
| Process restart failure | MEDIUM | 1. Check if new process started: `tasklist`<br>2. If not, check logs for restart error<br>3. Manually start: `nanobot-auto-updater.exe`<br>4. Auto-start logic should handle this on reboot |

## Existing Patterns to Reuse

The project already has patterns that should be extended for self-update:

| Existing Pattern | Location | How to Reuse for Self-Update |
|-----------------|----------|------------------------------|
| `sync/atomic.Bool` update guard | `internal/instance` | Extend to guard self-update AND nanobot update together |
| Bearer Token auth middleware | `internal/api/auth.go` | Wrap self-update endpoint with same middleware |
| `TriggerUpdater` interface | `internal/api/trigger.go` | Define similar interface for self-update (mock-friendly testing) |
| Pushover notification | `internal/notifier` | Send notification on self-update success/failure |
| UpdateLog recording | `internal/updatelog` | Record self-update operations with `triggeredBy: "self-update"` |
| `--version` flag | `cmd/nanobot-auto-updater/main.go` | Already works with ldflags, just needs CI integration |
| Graceful shutdown | `cmd/nanobot-auto-updater/main.go` | Extend to handle self-update restart scenario |
| JSON error responses (RFC 7807) | `internal/api/trigger.go` | Use same error format for self-update API responses |

## Sources

**go-update Library:**
- [inconshreveable/go-update apply.go](https://github.com/inconshreveable/go-update/blob/master/apply.go) -- Full source code read, confirms Windows rename pattern and rollback logic (HIGH confidence)
- [inconshreveable/go-update doc.go](https://github.com/inconshreveable/go-update/blob/master/doc.go) -- SHA256 checksum verification documentation (HIGH confidence)
- [go-update Issue #1: old exe not deleted on Windows](https://github.com/inconshreveable/go-update/issues/1) -- Known Windows cleanup issue (HIGH confidence)

**Windows Executable Replacement:**
- [Stack Overflow: How to self-update application while running](https://stackoverflow.com/questions/55247194/how-to-self-update-application-while-running) -- Rename trick (HIGH confidence)
- [SuperUser: Why can I rename a running executable but not delete it](https://superuser.com/questions/488127/why-can-i-rename-a-running-executable-but-not-delete-it) -- OS behavior (HIGH confidence)
- [cosmofy Issue #58: self-update overwrites running executable](https://github.com/metaist/cosmofy/issues/58) -- Real-world failure report (MEDIUM confidence)

**GitHub API Rate Limits:**
- [GitHub Community Discussion #170662](https://github.com/orgs/community/discussions/170662) -- 60/hr unauthenticated limit (HIGH confidence)
- [GitHub Changelog May 2025](https://github.blog/changelog/2025-05-08-updated-rate-limits-for-unauthenticated-requests/) -- Official rate limit update (HIGH confidence)
- [GitHub Changelog June 2025: Releases now expose digests](https://github.blog/changelog/2025-06-03-releases-now-expose-digests-for-release-assets/) -- Native SHA256 for release assets (HIGH confidence)

**GitHub Actions:**
- [GitHub Docs: Events that trigger workflows](https://docs.github.com/actions/using-workflows/events-that-trigger-workflows) -- Official trigger documentation (HIGH confidence)
- [GitHub Community Discussion #27028: Workflow not triggering](https://github.com/orgs/community/discussions/27028) -- Common trigger issue (HIGH confidence)
- [Rob Allen: Using GitHub Actions for Go binaries](https://akrabat.com/using-github-actions-to-add-go-binaries-to-a-release/) -- Practical GOOS/GOARCH guide (MEDIUM confidence)

**Self-Update Patterns:**
- [creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate) -- Alternative library with built-in GitHub Releases support (HIGH confidence)
- [Reddit r/golang: Self-updating binaries discussion](https://www.reddit.com/r/golang/comments/1poccfb/selfupdating_binaries_what_is_current_stage_and/) -- Community best practices (MEDIUM confidence)
- [Go Forum: Auto restart after self-update](https://forum.golangbridge.org/t/auto-restart-after-self-update/34792) -- Restart pattern code examples (MEDIUM confidence)

**Existing Codebase (highest confidence):**
- `cmd/nanobot-auto-updater/main.go` -- Version variable, startup logic, graceful shutdown
- `internal/api/trigger.go` -- Update handler pattern, 409 conflict, auth middleware
- `internal/api/auth.go` -- Bearer Token middleware
- `internal/api/server.go` -- Route registration pattern
- `internal/updater/updater.go` -- Existing update logic (nanobot, not self)
- `internal/instance` -- StartAllInstances, StopAllInstances, Atomic.Bool guard

---
*Pitfalls research for: Self-Update via GitHub Releases for Windows Go Application*
*Researched: 2026-03-29*
