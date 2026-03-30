# Phase 38: Self-Update Core - Research

**Researched:** 2026-03-30
**Domain:** Go self-updating binary via GitHub Releases (Windows amd64)
**Confidence:** HIGH

## Summary

Phase 38 creates `internal/selfupdate/` package implementing GitHub Release version checking, semver comparison, ZIP download+extraction, SHA256 checksum verification, and safe running-exe replacement using `minio/selfupdate`. The package operates as a standalone library with no HTTP API dependencies (that comes in Phase 39). All core technologies are already proven: `minio/selfupdate v0.6.0` was validated in Phase 36 PoC, GoReleaser ZIP+checksums pipeline is configured in Phase 37, and `golang.org/x/mod/semver` is already a transitive dependency for version comparison.

The key technical challenge is the checksum verification flow: download `checksums.txt` first, parse the SHA256 for the target ZIP, download and hash the ZIP, compare, then extract the exe from the ZIP in memory, and finally pass the exe bytes to `selfupdate.Apply()`. All of this uses standard library (`net/http`, `crypto/sha256`, `archive/zip`, `encoding/hex`) plus `minio/selfupdate` for the actual binary replacement.

**Primary recommendation:** Use `golang.org/x/mod/semver.Compare()` for version comparison (already a transitive dependency), `net/http` + `encoding/json` for GitHub API calls (raw HTTP, no SDK), `archive/zip` + `bytes.Reader` for in-memory ZIP extraction, and `minio/selfupdate.Apply()` with `Options{OldSavePath: exePath + ".old"}` for binary replacement.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Memory decompression -- download ZIP to memory (`bytes.Buffer`), use `archive/zip` to extract exe to `bytes.Buffer`, pass `io.Reader` to `selfupdate.Apply()`. No temp files on disk.
- **D-02:** checksums.txt dual verification -- download GoReleaser-generated `checksums.txt`, parse SHA256 for the ZIP file, compute actual downloaded ZIP's SHA256 and compare. Verify before extracting exe.
- **D-03:** Updater struct + method pattern. Public API:
  - `NewUpdater(cfg SelfUpdateConfig) *Updater` -- constructor accepting config
  - `CheckLatest() (*ReleaseInfo, error)` -- check GitHub latest Release (with cache)
  - `NeedUpdate(currentVersion string) (bool, *ReleaseInfo, error)` -- semver compare, dev version treated as needing update
  - `Update(currentVersion string) error` -- full update flow: download -> verify -> extract -> Apply
  - Cache and `http.Client` encapsulated inside struct
- **D-04:** Minimal config -- `config.yaml` new `self_update` section with only:
  - `github_owner` (string) -- GitHub repo owner (e.g. "HQGroup")
  - `github_repo` (string) -- GitHub repo name (e.g. "nanobot-auto-updater")
  - Other parameters hardcoded as package constants: cache TTL=1h, HTTP timeout=30s, User-Agent, etc.

### Claude's Discretion
- semver parsing implementation (stdlib or simple string comparison)
- Cache implementation details (struct field + time.Time)
- GitHub API error handling and retry strategy
- File splitting (whether to separate into checker.go, downloader.go, etc.)
- Testing strategy and mock approach

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UPDATE-01 | GitHub Release API check latest version (GET /repos/{owner}/{repo}/releases/latest) | GitHub API JSON response format documented below; use raw `net/http` + `encoding/json` |
| UPDATE-02 | Semver version comparison (current vs latest Release tag) | `golang.org/x/mod/semver.Compare()` -- already a transitive dep (v0.26.0); requires "v" prefix |
| UPDATE-03 | SHA256 checksum verification of downloaded binary integrity | GoReleaser `checksums.txt` format: `<hash>  <filename>` per line; `crypto/sha256` + `encoding/hex` |
| UPDATE-04 | minio/selfupdate safe replacement of running exe (Windows rename trick) | `minio/selfupdate v0.6.0` Apply() with io.Reader input; PoC validated in Phase 36 |
| UPDATE-05 | Backup current exe (Options.OldSavePath saves .old file) | `selfupdate.Options{OldSavePath: exePath + ".old"}`; PoC proven |
| UPDATE-06 | Release info cache (TTL 1 hour, avoid GitHub API rate limit 60/hr) | Simple struct field `cachedRelease *ReleaseInfo` + `cacheTime time.Time` |
| UPDATE-07 | Config file new self_update section (github_owner, github_repo) | Follow existing Config struct + viper pattern in `internal/config/config.go` |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/minio/selfupdate | v0.6.0 (in go.mod) | Running exe binary replacement | Phase 36 PoC validated. Handles Windows rename trick, .old backup, rollback on failure, kernel32.dll hide file. |
| golang.org/x/mod/semver | v0.26.0 (transitive) | Semver version comparison | Official Go team-maintained. `Compare()` returns -1/0/+1. Already in dependency tree via indirect. |
| net/http (stdlib) | Go 1.24 | GitHub API HTTP calls | User decision: raw HTTP, no SDK. Single endpoint, simple JSON decode. |
| crypto/sha256 (stdlib) | Go 1.24 | SHA256 checksum computation | `sha256.New()` + `io.Copy()` to hash ZIP bytes. Standard, battle-tested. |
| encoding/hex (stdlib) | Go 1.24 | Decode hex checksum from checksums.txt | `hex.DecodeString()` to convert hex string to bytes for comparison. |
| archive/zip (stdlib) | Go 1.24 | Extract exe from ZIP in memory | `zip.NewReader(bytes.NewReader(data), size)` -- no temp files per D-01. |
| encoding/json (stdlib) | Go 1.24 | Parse GitHub API JSON response | Simple struct unmarshal for Release+Assets. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/stretchr/testify | v1.11.1 (in go.mod) | Test assertions | All unit tests. `assert.Equal`, `require.NoError`, etc. |
| net/http/httptest (stdlib) | Go 1.24 | Mock HTTP server for tests | Create `httptest.NewServer` returning fake GitHub API JSON responses. |
| github.com/spf13/viper | v1.21.0 (in go.mod) | Config file binding | Add `self_update` section defaults + unmarshal into new `SelfUpdateConfig` struct. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang.org/x/mod/semver | hashicorp/go-version | go-version adds an external dep; x/mod is already transitive. x/mod requires "v" prefix which matches GitHub tags perfectly. |
| raw net/http + encoding/json | google/go-github SDK | SDK adds 5+ transitive deps for a single GET endpoint. User explicitly decided raw HTTP. |
| crypto/sha256 + manual compare | selfupdate.Options.Checksum | minio/selfupdate has built-in checksum but it hashes the io.Reader content (the exe). We need to verify the ZIP before extraction. Must do it manually. |

**Installation:**
```bash
# No new packages needed. All dependencies already in go.mod or stdlib.
# golang.org/x/mod v0.26.0 is indirect -- may need to add direct import
go get golang.org/x/mod/semver  # only if not already directly referenced
```

**Version verification:**
- `github.com/minio/selfupdate`: v0.6.0 (already in go.mod, verified)
- `golang.org/x/mod`: v0.26.0 (transitive, verified via `go list -m all`)
- `github.com/stretchr/testify`: v1.11.1 (already in go.mod)
- `github.com/spf13/viper`: v1.21.0 (already in go.mod)

## Architecture Patterns

### Recommended Project Structure
```
internal/
  selfupdate/           # NEW - Self-update core package
    selfupdate.go       # Updater struct, NewUpdater, CheckLatest, NeedUpdate, Update
    updater_test.go     # Unit tests with httptest mock server
    types.go            # ReleaseInfo, SelfUpdateConfig types (optional, could be in selfupdate.go)
internal/
  config/
    config.go           # MODIFY - add SelfUpdate SelfUpdateConfig field
    selfupdate.go       # NEW (optional) - SelfUpdateConfig struct + Validate
```

### Pattern 1: Updater Struct with Encapsulated State
**What:** All state (HTTP client, cache, config) encapsulated in Updater struct.
**When to use:** Always -- matches user decision D-03.
**Example:**
```go
// Source: CONTEXT.md D-03, project established patterns
type Updater struct {
    cfg           SelfUpdateConfig
    httpClient    *http.Client
    cachedRelease *ReleaseInfo
    cacheTime     time.Time
    logger        *slog.Logger
}

func NewUpdater(cfg SelfUpdateConfig, logger *slog.Logger) *Updater {
    return &Updater{
        cfg: cfg,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        logger: logger.With("source", "selfupdate"),
    }
}
```

### Pattern 2: Cache with TTL Check
**What:** Simple struct fields `cachedRelease` + `cacheTime`, checked in `CheckLatest()`.
**When to use:** Every `CheckLatest()` call.
**Example:**
```go
// Source: CONTEXT.md specifics
const cacheTTL = 1 * time.Hour

func (u *Updater) CheckLatest() (*ReleaseInfo, error) {
    if u.cachedRelease != nil && time.Since(u.cacheTime) < cacheTTL {
        return u.cachedRelease, nil
    }
    // Fetch from GitHub API...
    u.cachedRelease = release
    u.cacheTime = time.Now()
    return release, nil
}
```

### Pattern 3: In-Memory ZIP Extraction
**What:** Download ZIP to `[]byte`, use `bytes.NewReader` with `zip.NewReader`, extract exe to `io.Reader`.
**When to use:** During `Update()` flow.
**Example:**
```go
// Source: Go stdlib archive/zip, CONTEXT.md D-01
func extractExeFromZip(zipData []byte, exeName string) (io.Reader, error) {
    readerAt := bytes.NewReader(zipData)
    zipReader, err := zip.NewReader(readerAt, int64(len(zipData)))
    if err != nil {
        return nil, fmt.Errorf("zip open: %w", err)
    }
    for _, f := range zipReader.File {
        if f.Name == exeName {
            rc, err := f.Open()
            if err != nil {
                return nil, fmt.Errorf("zip entry open: %w", err)
            }
            defer rc.Close()
            var buf bytes.Buffer
            if _, err := io.Copy(&buf, rc); err != nil {
                return nil, fmt.Errorf("zip entry read: %w", err)
            }
            return &buf, nil
        }
    }
    return nil, fmt.Errorf("exe %q not found in zip", exeName)
}
```

### Pattern 4: Checksums.txt Parsing
**What:** Parse GoReleaser's `checksums.txt` format: `<sha256_hex>  <filename>\n`.
**When to use:** Before ZIP download/extraction.
**Example:**
```go
// Source: GoReleaser documentation, CONTEXT.md D-02
// checksums.txt format:
// a1b2c3d4e5f6...  nanobot-auto-updater_1.0.0_windows_amd64.zip

func parseChecksum(checksumsTxt []byte, filename string) ([]byte, error) {
    lines := strings.Split(string(checksumsTxt), "\n")
    for _, line := range lines {
        parts := strings.SplitN(line, "  ", 2)
        if len(parts) == 2 && parts[1] == filename {
            return hex.DecodeString(parts[0])
        }
    }
    return nil, fmt.Errorf("checksum for %q not found", filename)
}
```

### Pattern 5: minio/selfupdate Apply with OldSavePath
**What:** Use `selfupdate.Apply()` with `Options{OldSavePath}` to replace running exe.
**When to use:** Final step of `Update()`.
**Example:**
```go
// Source: tmp/poc_selfupdate.go (Phase 36 validated PoC)
func (u *Updater) applyUpdate(exeReader io.Reader) error {
    exePath, err := os.Executable()
    if err != nil {
        return fmt.Errorf("get exe path: %w", err)
    }
    opts := selfupdate.Options{
        OldSavePath: exePath + ".old",
    }
    err = selfupdate.Apply(exeReader, opts)
    if err != nil {
        if rerr := selfupdate.RollbackError(err); rerr != nil {
            return fmt.Errorf("update failed and rollback also failed: %w (rollback: %v)", err, rerr)
        }
        return fmt.Errorf("update failed (rolled back): %w", err)
    }
    return nil
}
```

### Pattern 6: Config Extension (Follow Existing Pattern)
**What:** New `SelfUpdateConfig` struct added to `Config`, with defaults and viper binding.
**When to use:** Config loading.
**Example:**
```go
// Source: internal/config/config.go pattern, CONTEXT.md D-04
type SelfUpdateConfig struct {
    GithubOwner string `yaml:"github_owner" mapstructure:"github_owner"`
    GithubRepo  string `yaml:"github_repo" mapstructure:"github_repo"`
}

// In Config struct:
type Config struct {
    // ... existing fields ...
    SelfUpdate SelfUpdateConfig `yaml:"self_update" mapstructure:"self_update"`
}

// In defaults():
c.SelfUpdate.GithubOwner = "HQGroup"
c.SelfUpdate.GithubRepo = "nanobot-auto-updater"

// In Load():
v.SetDefault("self_update.github_owner", cfg.SelfUpdate.GithubOwner)
v.SetDefault("self_update.github_repo", cfg.SelfUpdate.GithubRepo)
```

### Pattern 7: httptest Mock Server for Testing
**What:** Use `httptest.NewServer` to create a fake GitHub API for unit tests.
**When to use:** All unit tests for CheckLatest, checksums, and download.
**Example:**
```go
// Source: project test patterns (internal/api/trigger_test.go)
func TestCheckLatest(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "tag_name": "v1.0.0",
            "assets": []map[string]interface{}{
                {
                    "name":               "nanobot-auto-updater_1.0.0_windows_amd64.zip",
                    "browser_download_url": ts.URL + "/download/zip",
                    "size":               1024,
                },
                {
                    "name":               "nanobot-auto-updater_1.0.0_checksums.txt",
                    "browser_download_url": ts.URL + "/download/checksums",
                },
            },
        })
    }))
    defer ts.Close()
    // Pass ts.URL as base URL to Updater...
}
```

### Anti-Patterns to Avoid
- **Using google/go-github SDK:** User explicitly decided raw `net/http`. Single endpoint does not justify 5+ transitive dependencies.
- **Writing ZIP to disk:** D-01 explicitly requires in-memory decompression. No temp files.
- **Using selfupdate.Options.Checksum for ZIP verification:** The `Checksum` field hashes the io.Reader content (the exe passed to Apply). We need to verify the ZIP before extraction. Must compute SHA256 of the ZIP manually using `crypto/sha256`.
- **Forgetting RollbackError check:** Per minio/selfupdate docs, always call `selfupdate.RollbackError(err)` on non-nil Apply errors. Phase 36 PoC demonstrates this correctly.
- **Omitting OldSavePath:** Without `OldSavePath`, minio/selfupdate attempts to delete the old exe (fails on Windows -- running process locks it). With `OldSavePath`, it renames to .old which works on Windows. Phase 36 PoC validated this.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Running exe replacement | Custom rename + file copy logic | `minio/selfupdate.Apply()` | Handles Windows rename trick, rollback on failure, .old file management, kernel32.dll hide file. 400 lines of battle-tested code. |
| Semver comparison | String split + int parsing | `golang.org/x/mod/semver.Compare()` | Already in dependency tree. Handles pre-release, build metadata, edge cases. Returns -1/0/+1. |
| ZIP extraction from memory | Custom ZIP reader | `archive/zip.NewReader(bytes.NewReader(data), size)` | Stdlib handles all ZIP format complexity. No temp files needed. |
| SHA256 hex encoding/decoding | Manual hex string conversion | `encoding/hex.EncodeToString()` / `hex.DecodeString()` | Stdlib, handles all edge cases, proven correct. |
| GitHub API JSON parsing | Manual string search in response body | `encoding/json.Unmarshal()` into typed struct | Type-safe, handles nested structures, proper error reporting. |

**Key insight:** Every piece of the update flow has a stdlib or existing-dependency solution. The only external library is `minio/selfupdate` which handles the one thing you absolutely cannot hand-roll safely: replacing a running Windows exe.

## Common Pitfalls

### Pitfall 1: Missing "v" Prefix for semver.Compare
**What goes wrong:** `semver.Compare("1.0.0", "v1.0.0")` treats "1.0.0" as invalid (returns -1).
**Why it happens:** GitHub Release `tag_name` is "v1.0.0" but `main.Version` from ldflags is "1.0.0" (without "v").
**How to avoid:** Ensure version strings have "v" prefix before calling `Compare()`. Helper: `func ensureVPrefix(v string) string { if !strings.HasPrefix(v, "v") { return "v" + v }; return v }`.
**Warning signs:** Version comparison always returns "update needed" or "no update needed" regardless of actual versions.

### Pitfall 2: GitHub API Rate Limiting (60 requests/hour unauthenticated)
**What goes wrong:** Frequent `CheckLatest()` calls exhaust GitHub API quota, returning 403 errors.
**Why it happens:** Unauthenticated GitHub API allows 60 requests/hour. Without caching, every API call hits GitHub.
**How to avoid:** Implement 1-hour cache (D-06). `cachedRelease` + `cacheTime` prevents redundant calls. Already specified in CONTEXT.md.
**Warning signs:** Intermittent 403 errors from GitHub API, `X-RateLimit-Remaining: 0` header in responses.

### Pitfall 3: ZIP Entry Name Does Not Match Expected Pattern
**What goes wrong:** `extractExeFromZip()` cannot find the exe because the name inside the ZIP differs from expectation.
**Why it happens:** GoReleaser places `nanobot-auto-updater.exe` in the ZIP root (from `binary: nanobot-auto-updater` in .goreleaser.yaml + Windows auto-.exe). But code might search for just `nanobot-auto-updater` without `.exe`.
**How to avoid:** Search for `nanobot-auto-updater.exe` specifically. Verify with `go run github.com/goreleaser/goreleaser/v2 check` or inspect actual release assets.
**Warning signs:** "exe not found in zip" error despite successful ZIP download.

### Pitfall 4: checksums.txt Asset Not Found in Release
**What goes wrong:** Cannot find `checksums.txt` asset because the filename format includes version: `nanobot-auto-updater_1.0.0_checksums.txt`.
**Why it happens:** GoReleaser `.goreleaser.yaml` uses `name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"`. The version number is part of the filename.
**How to avoid:** When searching assets for the checksums file, match by suffix `"_checksums.txt"` or iterate assets looking for one containing "checksums". Do not hardcode the full filename.
**Warning signs:** "checksum asset not found" error.

### Pitfall 5: Empty OldSavePath Causes Windows Delete Failure
**What goes wrong:** `selfupdate.Apply()` fails because it tries to delete the old running exe (impossible on Windows).
**Why it happens:** If `OldSavePath` is empty string, minio/selfupdate attempts `os.Remove()` on the old exe instead of renaming it.
**How to avoid:** Always set `OldSavePath: exePath + ".old"` (non-empty string). Phase 36 PoC validated this.
**Warning signs:** Apply error on Windows: "access denied" or "file in use".

### Pitfall 6: Dev Version Detection Edge Case
**What goes wrong:** `NeedUpdate("dev", "v1.0.0")` does not return true.
**Why it happens:** If code only uses `semver.Compare()` and does not special-case "dev", the comparison treats "dev" as invalid semver (less than any valid version), which may or may not trigger update depending on comparison direction.
**How to avoid:** Explicit check: `if currentVersion == "dev" { return true, release, nil }` before semver comparison. Per UPDATE-02 requirement.
**Warning signs:** Dev builds never detect updates available.

### Pitfall 7: bytes.Reader Position After Read
**What goes wrong:** SHA256 hash of ZIP is computed correctly, but then ZIP extraction fails because `bytes.Reader` position is at EOF.
**Why it happens:** After computing SHA256 via `io.Copy(hasher, bytesReader)`, the reader position is at the end.
**How to avoid:** Use `bytes.NewReader(zipData)` separately for hash and for zip extraction, or `bytes.NewReader.Seek(0, io.SeekStart)` between operations.
**Warning signs:** ZIP extraction fails after successful checksum verification.

## Code Examples

Verified patterns from official sources and project PoC:

### GitHub API Response Parsing
```go
// Source: GitHub REST API docs (docs.github.com/en/rest/releases/releases)
// GET /repos/{owner}/{repo}/releases/latest

type githubRelease struct {
    TagName string           `json:"tag_name"`
    Name    string           `json:"name"`
    Body    string           `json:"body"`
    Assets  []githubAsset    `json:"assets"`
    HTMLURL string           `json:"html_url"`
}

type githubAsset struct {
    Name               string `json:"name"`
    BrowserDownloadURL string `json:"browser_download_url"`
    Size               int64  `json:"size"`
    ContentType        string `json:"content_type"`
}
```

### Semver Comparison with Dev Detection
```go
// Source: golang.org/x/mod/semver docs (pkg.go.dev)
import "golang.org/x/mod/semver"

func (u *Updater) NeedUpdate(currentVersion string) (bool, *ReleaseInfo, error) {
    release, err := u.CheckLatest()
    if err != nil {
        return false, nil, err
    }

    // Dev version always needs update (UPDATE-02)
    if currentVersion == "dev" {
        return true, release, nil
    }

    // Ensure "v" prefix for semver comparison
    current := "v" + strings.TrimPrefix(currentVersion, "v")
    latest := "v" + strings.TrimPrefix(release.Version, "v")

    result := semver.Compare(current, latest)
    return result < 0, release, nil
}
```

### SHA256 Checksum Verification of ZIP
```go
// Source: crypto/sha256 stdlib, CONTEXT.md D-02
func verifyChecksum(data []byte, expectedSHA256 []byte) bool {
    hash := sha256.Sum256(data)
    return bytes.Equal(hash[:], expectedSHA256)
}
```

### Full Update Flow (Update method outline)
```go
// Source: CONTEXT.md D-01, D-02, D-03 + minio/selfupdate docs
func (u *Updater) Update(currentVersion string) error {
    // 1. Check if update needed
    needsUpdate, release, err := u.NeedUpdate(currentVersion)
    if err != nil {
        return fmt.Errorf("check update: %w", err)
    }
    if !needsUpdate {
        return nil // already up to date
    }

    // 2. Download checksums.txt
    checksumsAsset := findAsset(release, "_checksums.txt")
    checksumsData, err := u.download(checksumsAsset.BrowserDownloadURL)
    if err != nil {
        return fmt.Errorf("download checksums: %w", err)
    }

    // 3. Download ZIP
    zipAsset := findAsset(release, "_windows_amd64.zip")
    zipData, err := u.download(zipAsset.BrowserDownloadURL)
    if err != nil {
        return fmt.Errorf("download zip: %w", err)
    }

    // 4. Verify ZIP checksum
    expectedHash, err := parseChecksum(checksumsData, zipAsset.Name)
    if err != nil {
        return fmt.Errorf("parse checksum: %w", err)
    }
    if !verifyChecksum(zipData, expectedHash) {
        return fmt.Errorf("checksum verification failed")
    }

    // 5. Extract exe from ZIP in memory
    exeReader, err := extractExeFromZip(zipData, "nanobot-auto-updater.exe")
    if err != nil {
        return fmt.Errorf("extract exe: %w", err)
    }

    // 6. Apply update
    return u.applyUpdate(exeReader)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| go-github SDK for API calls | Raw net/http + encoding/json | User decision (CONTEXT.md) | Minimal deps, simpler code for single endpoint |
| creativeprojects/go-selfupdate | minio/selfupdate + custom download logic | Phase 36 decision | Less opinionated, fine-grained control over download/verify pipeline |
| File-based ZIP extraction | In-memory ZIP extraction | CONTEXT.md D-01 | No temp file cleanup, no disk write exposure |
| No checksum verification | checksums.txt dual verification | CONTEXT.md D-02 | GoReleaser checksums.txt used as integrity source |

**Deprecated/outdated:**
- `inconshreveable/go-update`: Unmaintained since 2015. Use `minio/selfupdate` (maintained fork).
- `creativeprojects/go-selfupdate`: Evaluated and rejected. Too opinionated, heavy dep tree.
- `google/go-github`: Rejected for this project. Single endpoint does not justify the dependency weight.

## Open Questions

1. **Should golang.org/x/mod be added as a direct dependency?**
   - What we know: It's already an indirect dependency at v0.26.0 (via other packages).
   - What's unclear: Whether Go toolchain will preserve it if no direct import exists.
   - Recommendation: Add `golang.org/x/mod` as a direct require in go.mod to ensure stability. Run `go get golang.org/x/mod` after adding import.

2. **Should SelfUpdateConfig have a Validate() method?**
   - What we know: User decided config is minimal (owner + repo). Other configs in project have Validate().
   - What's unclear: Whether empty owner/repo should be a hard error or disable self-update silently.
   - Recommendation: Follow existing pattern -- add Validate() that returns error if owner or repo is empty when the section is present. This matches how APIConfig.Validate() works.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24 | Build | Yes | go1.24.11 windows/amd64 | -- |
| github.com/minio/selfupdate | Binary replacement | Yes (go.mod) | v0.6.0 | -- |
| golang.org/x/mod/semver | Version comparison | Yes (transitive) | v0.26.0 | -- |
| github.com/spf13/viper | Config loading | Yes (go.mod) | v1.21.0 | -- |
| github.com/stretchr/testify | Testing | Yes (go.mod) | v1.11.1 | -- |
| Internet access | GitHub API calls | Required at runtime | -- | CheckLatest returns error if offline |

**Missing dependencies with no fallback:**
- None. All required packages are available.

**Missing dependencies with fallback:**
- None needed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None -- standard Go test files |
| Quick run command | `go test ./internal/selfupdate/ -v -count=1` |
| Full suite command | `go test ./internal/... -v -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UPDATE-01 | CheckLatest returns version + download URL from GitHub API | unit | `go test ./internal/selfupdate/ -run TestCheckLatest -v` | Wave 0 |
| UPDATE-01 | CheckLatest handles GitHub API errors gracefully | unit | `go test ./internal/selfupdate/ -run TestCheckLatest_APIError -v` | Wave 0 |
| UPDATE-02 | NeedUpdate correctly compares semver versions | unit | `go test ./internal/selfupdate/ -run TestNeedUpdate -v` | Wave 0 |
| UPDATE-02 | NeedUpdate returns true for "dev" version | unit | `go test ./internal/selfupdate/ -run TestNeedUpdate_Dev -v` | Wave 0 |
| UPDATE-03 | SHA256 checksum verification passes for valid data | unit | `go test ./internal/selfupdate/ -run TestVerifyChecksum -v` | Wave 0 |
| UPDATE-03 | SHA256 checksum verification fails for corrupted data | unit | `go test ./internal/selfupdate/ -run TestVerifyChecksum_Invalid -v` | Wave 0 |
| UPDATE-04 | Update calls selfupdate.Apply with correct reader | unit | `go test ./internal/selfupdate/ -run TestUpdate -v` | Wave 0 |
| UPDATE-05 | OldSavePath set to exe path + ".old" | unit | `go test ./internal/selfupdate/ -run TestApplyUpdate_OldSavePath -v` | Wave 0 |
| UPDATE-06 | Cache returns same result within TTL | unit | `go test ./internal/selfupdate/ -run TestCache -v` | Wave 0 |
| UPDATE-06 | Cache refreshes after TTL expires | unit | `go test ./internal/selfupdate/ -run TestCache_Expiry -v` | Wave 0 |
| UPDATE-07 | SelfUpdateConfig loaded from yaml with defaults | unit | `go test ./internal/config/ -run TestSelfUpdateConfig -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/selfupdate/ -v -count=1`
- **Per wave merge:** `go test ./internal/... -v -count=1`
- **Phase gate:** `go test ./... -v -count=1`

### Wave 0 Gaps
- [ ] `internal/selfupdate/selfupdate_test.go` -- covers UPDATE-01 through UPDATE-06
- [ ] `internal/config/selfupdate_test.go` -- covers UPDATE-07 (config loading + defaults)
- [ ] No framework install needed -- Go test framework and testify already available

## Sources

### Primary (HIGH confidence)
- `github.com/minio/selfupdate v0.6.0` source -- Apply(), Options, RollbackError APIs verified via `go doc`
- `golang.org/x/mod/semver` docs (pkg.go.dev) -- Compare(), IsValid(), Canonical() verified
- `archive/zip` stdlib docs -- NewReader requires io.ReaderAt + int64 size, confirmed
- `crypto/sha256` stdlib docs -- Sum256() returns [32]byte, confirmed
- `tmp/poc_selfupdate.go` -- Phase 36 PoC code, validated Apply() pattern on Windows
- `.goreleaser.yaml` -- Verified ZIP archive format and checksums.txt naming template
- `.github/workflows/release.yml` -- Verified release trigger on v* tags
- `internal/config/config.go` -- Verified Config struct pattern, viper binding, defaults() pattern
- `internal/api/trigger_test.go` -- Verified httptest mock server pattern used in project

### Secondary (MEDIUM confidence)
- [GitHub REST API Releases docs](https://docs.github.com/en/rest/releases/releases) -- Response JSON format, tag_name, assets[].browser_download_url
- [GoReleaser Checksums docs](https://goreleaser.com/customization/checksum/) -- checksums.txt format: `<hash>  <filename>`
- [GoReleaser Archives docs](https://goreleaser.com/customization/archive/) -- Binary naming inside archives, .exe extension on Windows

### Tertiary (LOW confidence)
- None. All findings verified with primary or secondary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all packages already in go.mod or stdlib, verified via `go doc` and `go list`
- Architecture: HIGH -- follows established project patterns (config.go, trigger.go), PoC validated
- Pitfalls: HIGH -- most pitfalls discovered during Phase 36 PoC validation, verified with library source

**Research date:** 2026-03-30
**Valid until:** 2026-04-30 (stable domain -- stdlib + mature libraries)
