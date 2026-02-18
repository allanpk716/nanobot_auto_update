# Pitfalls Research

**Domain:** Windows background service / CLI tool with auto-updater (Go)
**Researched:** 2026-02-18
**Confidence:** MEDIUM (WebSearch-based, verified across multiple sources)

## Critical Pitfalls

### Pitfall 1: Cannot Replace Running Binary on Windows

**What goes wrong:**
Attempting to replace the running executable while it's in use results in "Access Denied" errors. Unlike Unix systems where you can unlink and replace a running binary, Windows locks the executable file while it's running.

**Why it happens:**
Windows file locking prevents modification of any file that has an open handle. The OS maintains an exclusive lock on the executable until the process terminates.

**How to avoid:**
1. Download the new binary to a temporary location
2. Move/rename the old binary to a backup location (this works even when locked)
3. Move the new binary to the target location
4. On next startup, clean up the old backup file
5. Alternatively, use the "move and replace on reboot" pattern with `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT`

**Warning signs:**
- Update logic that tries to directly overwrite the executable
- No rename/backup step in update flow
- Error handling that ignores "file in use" errors

**Phase to address:**
Phase 1 (Core Update Logic) - This is foundational to the auto-updater and must be designed correctly from the start.

---

### Pitfall 2: Service Control Manager (SCM) Recovery Actions Don't Work as Expected

**What goes wrong:**
Windows service recovery settings (restart on failure) don't trigger when expected, or cause system hangs. Services get stuck in restart loops, or conversely, fail without restarting.

**Why it happens:**
The SCM has specific conditions for triggering recovery actions:
- The service must call `SetServiceStatus` with a non-zero exit code
- A reset period of 0 can cause system hangs (Windows bug)
- Recovery actions only trigger on unexpected failures, not graceful stops

**How to avoid:**
1. Set appropriate `WaitHint` and `CheckPoint` values in service status
2. Never set the reset period to 0 - use at least 1 second
3. Ensure service stop requests properly signal shutdown intent
4. Implement application-level health checks instead of relying solely on SCM recovery
5. Use a supervisor pattern with exponential backoff for restarts

**Warning signs:**
- Service crashes but SCM doesn't restart it
- System hangs when service crashes repeatedly
- Reset period configured as 0

**Phase to address:**
Phase 1 (Core Service Setup) - Recovery behavior must be tested during initial service implementation.

---

### Pitfall 3: Cron Scheduler Job Overlap and Pile-up

**What goes wrong:**
When a scheduled job takes longer than the interval, multiple instances pile up and run back-to-back, causing resource exhaustion and unexpected behavior.

**Why it happens:**
Most cron libraries (robfig/cron, go-co-op/gocron) will queue missed executions by default. If your job runs every 5 minutes but takes 7 minutes, you'll accumulate delayed executions that all trigger once the long job completes.

**How to avoid:**
1. Implement a mutex or flag to prevent concurrent job execution
2. Use `SkipIfStillRunning` option if available in your scheduler
3. Consider using a distributed lock for multi-instance scenarios
4. Log job start/end times to detect overlap issues
5. Design jobs to be idempotent and self-limiting

```go
// Example: Using a mutex to prevent overlap
var jobMutex sync.Mutex

func runUpdateJob() {
    if !jobMutex.TryLock() {
        log.Println("Previous job still running, skipping")
        return
    }
    defer jobMutex.Unlock()
    // actual job logic
}
```

**Warning signs:**
- Jobs taking longer than scheduled interval
- CPU/memory spikes after long-running job completes
- Logs showing multiple jobs starting simultaneously

**Phase to address:**
Phase 1 (Scheduling Logic) - Concurrency control must be built into the scheduling design.

---

### Pitfall 4: Command Prompt Window Flashes on Background Execution

**What goes wrong:**
Even when running as a background service, spawning subprocesses (like `uv` commands) causes visible command prompt windows to briefly appear, disrupting users.

**Why it happens:**
By default, `os/exec` on Windows creates a new console window for subprocesses. Even with `-ldflags -H=windowsgui` on the main binary, child processes can still create windows.

**How to avoid:**
Use `SysProcAttr` with appropriate flags to hide child process windows:

```go
cmd := exec.Command("uv", "pip", "install", "nanobot")
cmd.SysProcAttr = &syscall.SysProcAttr{
    HideWindow: true,
    CreationFlags: syscall.CREATE_NO_WINDOW,
}
```

**Warning signs:**
- User reports of command windows appearing briefly
- Testing only on developer machines where windows are expected
- Not setting `SysProcAttr` for any exec.Command calls

**Phase to address:**
Phase 1 (Core Update Logic) - All subprocess spawning must include window hiding from the start.

---

### Pitfall 5: Log Rotation Breaks Logging Mid-Flight

**What goes wrong:**
When log files rotate, some log entries are lost, or the logger continues writing to the old (rotated) file handle instead of the new file.

**Why it happens:**
Log rotation libraries like lumberjack work by closing and reopening files. If writes are in progress during rotation, or if the logger holds a stale file handle, entries can be lost or go to the wrong file.

**How to avoid:**
1. Use a rotation-aware logging library (lumberjack, or zap with lumberjack sink)
2. Ensure the logger interface is used correctly (don't hold onto writers)
3. Configure appropriate rotation thresholds (size/time) with safety margins
4. Test rotation under load during development
5. Consider using `Sync()` calls before operations that might trigger rotation

**Warning signs:**
- Log files with suspicious gaps
- Multiple log files being written simultaneously
- Log entries appearing in old rotated files

**Phase to address:**
Phase 1 (Logging Setup) - Log rotation must be tested as part of initial logging implementation.

---

### Pitfall 6: Configuration Zero-Value Bugs (Silent Failures)

**What goes wrong:**
Configuration parsing silently accepts invalid values and uses Go's zero values, leading to unexpected behavior that's hard to debug. A typo in YAML/JSON config results in `0`, `""`, or `false` instead of an error.

**Why it happens:**
Go's encoding/json and yaml.v3 will silently ignore unknown fields and use zero values for missing/invalid fields. This is "valid" parsing but semantically wrong.

**How to avoid:**
1. Use `mapstructure` with `squelch` tags for strict parsing
2. Implement explicit validation after parsing
3. Use configuration structs with `omitempty` carefully - missing required fields become zero values
4. Add a "debug config" command that dumps parsed configuration
5. Use environment variable overrides with explicit required field checks

```go
// Example: Explicit validation
type Config struct {
    UpdateInterval time.Duration `yaml:"update_interval"`
    LogPath        string        `yaml:"log_path"`
}

func (c *Config) Validate() error {
    if c.UpdateInterval == 0 {
        return errors.New("update_interval is required")
    }
    if c.LogPath == "" {
        return errors.New("log_path is required")
    }
    return nil
}
```

**Warning signs:**
- Config parsing never fails despite malformed input
- Default values (zero values) appearing unexpectedly
- No validation after config loading

**Phase to address:**
Phase 1 (Configuration) - Validation must be implemented alongside configuration loading.

---

### Pitfall 7: HTTP Client Default Timeout is None

**What goes wrong:**
HTTP requests hang indefinitely when the server is unresponsive, blocking the update process and potentially the entire service.

**Why it happens:**
Go's `http.Client` has no default timeout. A misbehaving server can leave connections hanging forever.

**How to avoid:**
1. Always create a custom `http.Client` with explicit timeouts
2. Set `Client.Timeout` for overall request timeout
3. Consider using `context.WithTimeout` for individual requests
4. Implement retry logic with exponential backoff and jitter
5. Handle both network errors and HTTP error status codes in retry logic

```go
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        ResponseHeaderTimeout: 10 * time.Second,
        IdleConnTimeout:       90 * time.Second,
    },
}
```

**Warning signs:**
- Using `http.DefaultClient` directly
- No timeout configuration for HTTP requests
- Network operations that occasionally hang indefinitely

**Phase to address:**
Phase 1 (Network Operations) - HTTP client configuration must be set up before any network calls.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip verification after update download | Faster development | Corrupted updates cause mysterious failures | Never - always verify checksums/signatures |
| Use global singletons for scheduler/logger | Less parameter passing | Hard to test, hard to reset between tests | Never in production code |
| Ignore `uv` exit codes | Simpler error handling | Silent update failures, stale versions | Never |
| Log to stdout only | Simple setup | Lost logs in service context, no rotation | Development only |
| Skip graceful shutdown | Faster exit | In-flight operations corrupted | Never for background services |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| uv package manager | Assuming uv is in PATH | Use full path or verify `uv` in PATH at startup |
| uv package manager | Ignoring python version compatibility issues | Check `requires-python` before installing |
| uv package manager | Not handling network failures during install | Implement retry with backoff for `uv pip install` |
| Windows Service | Not handling Session 0 isolation | Ensure no UI dependencies; test in Session 0 |
| Windows Service | Using current user's environment variables | Services run as SYSTEM - use service-specific config paths |
| File system | Hardcoding paths like `C:\Users\...` | Use `%PROGRAMDATA%` or `%ALLUSERSPROFILE%` for shared data |
| Notification webhooks | No timeout on webhook calls | Set client timeout; use fire-and-forget with queue for reliability |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| No connection pooling | Many TIME_WAIT sockets, slow requests | Use `http.Transport` with connection pooling | 10+ concurrent requests |
| Unbounded log file growth | Disk space exhaustion | Implement log rotation with size limits | Days/weeks of operation |
| Synchronous updates on startup | Slow service startup | Run updates in background; use cached version for startup | When updates take >30 seconds |
| No request queuing | Memory exhaustion under load | Implement bounded queue for operations | High update check frequency |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Downloading updates over HTTP | Man-in-the-middle attack delivers malicious binary | Always use HTTPS; verify TLS certificate |
| No signature verification on updates | Compromised CDN delivers malicious binary | Sign updates; verify signature before applying |
| Storing credentials in config file | Credential exposure if file is read | Use Windows Credential Manager or environment variables |
| Running service as SYSTEM unnecessarily | Privilege escalation if service is compromised | Use minimal privilege service account |
| Logging sensitive data | Credential/token exposure in logs | Redact sensitive fields before logging |
| No integrity check on downloaded binaries | Corrupted update breaks installation | Always verify checksum/hash before applying |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Silent updates with no notification | Users unaware of changes; surprised by behavior changes | Log update events; optionally notify on major updates |
| No rollback mechanism | Stuck on broken version | Keep previous binary; provide rollback command |
| Blocking updates during critical work | Disruption to user workflow | Defer updates; apply during idle periods |
| Cryptic error messages | Users can't troubleshoot or report issues | Include actionable error messages with context |
| No "check for update" command | Users forced to wait for scheduled check | Provide manual update trigger |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Update Process:** Often missing rollback capability - verify you can revert to previous version
- [ ] **Service Installation:** Often missing proper uninstall/cleanup - verify service can be cleanly removed
- [ ] **Log Rotation:** Often missing handling of log during rotation - verify no logs lost during rotation
- [ ] **Error Recovery:** Often missing retry after network failure - verify update retries on transient failures
- [ ] **Graceful Shutdown:** Often missing wait for in-flight operations - verify clean shutdown with pending operations
- [ ] **Configuration:** Often missing validation of required fields - verify startup fails on missing config
- [ ] **Windows Service:** Often missing testing in Session 0 - verify service works when started by SCM

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Corrupted update blocks service | HIGH | Manual intervention: stop service, delete corrupted binary, restore backup or reinstall |
| Log rotation broke logging | LOW | Restart service; logs resume to new file |
| Config error causes crash loop | MEDIUM | Boot into safe mode / use alternate config path; fix config file |
| Scheduler stuck in overlap | LOW | Restart service; clears job queue |
| Network timeout blocks update | LOW | Automatic: retry with backoff; manual: check network/connectivity |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Cannot Replace Running Binary | Phase 1 | Test update while service is running; verify rename-then-replace works |
| SCM Recovery Actions | Phase 1 | Kill service process; verify SCM restarts it |
| Cron Scheduler Job Overlap | Phase 1 | Schedule job at 1-minute interval; make job take 2 minutes; verify no overlap |
| Command Prompt Window Flashes | Phase 1 | Run service; trigger update; verify no visible windows |
| Log Rotation Breaks Logging | Phase 1 | Fill log to rotation threshold; verify logging continues to new file |
| Configuration Zero-Value Bugs | Phase 1 | Provide config with typo; verify startup fails with clear error |
| HTTP Client Default Timeout | Phase 1 | Point at server that never responds; verify request times out |

## Sources

- Microsoft Learn: "Descriptions of some best practices when you create Windows Services" - https://support.microsoft.com/en-us/topic/descriptions-of-some-best-practices-when-you-create-windows-services-13ca508e-231d-43e6-b960-3b04ccf79064
- Microsoft Learn: "Guidelines for Services" - https://learn.microsoft.com/en-us/windows/win32/rstmgr/guidelines-for-services
- InfoQ: "The Service and the Beast: Building a Windows Service that Does Not Fail to Restart" - https://infoq.com/articles/windows-services-reliable-restart
- Stephen Cleary Blog: "Win32 Service Gotcha: Recovery Actions" - https://blog.stephencleary.com/2020/06/servicebase-gotcha-recovery-actions.html
- GitHub go-co-op/gocron Issue #385: "CPU usage 100% after system time change" - https://github.com/go-co-op/gocron/issues/385
- Stack Overflow: "How to hide command prompt window when using Exec in Golang" - https://stackoverflow.com/questions/42500570
- GitHub golang/go Issue #69939: "syscall: special case cmd.exe /c in StartProcess" - https://github.com/golang/go/issues/69939
- GitHub natefinch/lumberjack Issues: Log rotation problems - https://github.com/natefinch/lumberjack/issues
- Medium: "Implementing Log File Rotation in Go: Insights from logrus, zap, and slog" - https://leapcell.io/blog/log-rotation-and-file-splitting-in-go
- Build Software Systems: "Go Config: Stop the Silent YAML Bug (Use mapstructure for Safety)" - https://buildsoftwaresystems.com/post/go-config-yaml-safer-mapstructure-fix/
- DEV Community: "Mastering Network Timeouts and Retries in Go" - https://dev.to/jones_charles_ad50858dbc0/mastering-network-timeouts-and-retries-in-go
- Lokal.so: "Comprehensive Guide on Golang Self-upgrading binary" - https://lokal.so/blog/comprehensive-guide-on-golang-go-self-upgrading-binary/
- GitHub creativeprojects/go-selfupdate - https://github.com/creativeprojects/go-selfupdate
- GitHub fynelabs/selfupdate - https://github.com/fynelabs/selfupdate
- Microsoft Tech Community: "Application Compatibility - Session 0 Isolation" - https://techcommunity.microsoft.com/blog/askperf/application-compatibility---session-0-isolation/372361
- Core Technologies Blog: "Investigating OneDrive Failures in Session 0 on Windows Server" - https://www.coretechnologies.com/blog/alwaysup/onedrive-fails-in-session-0/
- GitHub astral-sh/uv Issues: Various compatibility and behavior issues - https://github.com/astral-sh/uv/issues

---
*Pitfalls research for: Windows background service / CLI tool with auto-updater (Go)*
*Researched: 2026-02-18*
