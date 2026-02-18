# Stack Research

**Domain:** Windows Background Service / CLI Tool for Auto-Updating Python Tools
**Researched:** 2025-02-18
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Go** | 1.23+ | Primary language | Required by project specification. Excellent for CLI tools and background services with single-binary deployment, cross-compilation support, and minimal runtime dependencies. |
| **kardianos/service** | v1.2.4 | Cross-platform service management | The de facto standard for running Go programs as Windows services. Supports Windows XP+, Linux (systemd/Upstart/SysV), and macOS Launchd with a unified API. 4.8k stars, actively maintained, handles Windows service callbacks that are non-trivial to implement manually. |
| **robfig/cron** | v3.x | Cron-based job scheduling | Industry standard cron library for Go with 98.3 benchmark score. Supports standard 5-field cron expressions, timezone-aware scheduling (CRON_TZ prefix), predefined descriptors (@hourly, @daily), and @every intervals. Thread-safe with concurrent job execution in separate goroutines. |
| **go-yaml** | v4.x (go.yaml.in/yaml/v4) | YAML configuration parsing | Official YAML library for Go with active maintenance. Supports strict field validation via WithKnownFields() to catch typos in config files. Pure Go implementation with reliable performance. Use go.yaml.in/yaml/v4 import path. |
| **urfave/cli** | v3.x | Command-line argument parsing | Zero-dependency CLI framework with 23.8k stars and high activity (9.0). Provides declarative API for commands, subcommands, flags with aliases, shell completion for 4 shells, and built-in help generation. Simpler than Cobra for straightforward CLI tools. |
| **WQGroup/logger** | Latest | Structured logging | Project-specified requirement. Built on logrus with added features: log rotation (time + size based), automatic cleanup of old logs, hierarchical path storage (YYYY/MM/DD), YAML config support, and Windows GUI compatibility. Provides millisecond timestamps. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **gregdel/pushover** | v1.x | Pushover notification client | Required for failure alerts. Official Go wrapper for Pushover API with 154 stars. Simple API: create app with token, create recipient, send message. Supports message titles, priorities, and emergency priority with acknowledgment. |
| **os/exec** | stdlib | Execute uv commands | Use standard library to run `uv tool upgrade` and `uv self update` commands. Capture stdout/stderr for logging. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **go mod** | Dependency management | Standard Go module system. Initialize with `go mod init github.com/yourorg/nanobot-auto-update` |
| **golangci-lint** | Linting | Recommended linter for Go projects with comprehensive rule set |
| **GoReleaser** | Build/release automation | Optional but recommended for creating Windows executables with version info and code signing support |

## Installation

```bash
# Initialize Go module
go mod init nanobot-auto-update

# Core dependencies
go get github.com/kardianos/service@v1.2.4
go get github.com/robfig/cron/v3@latest
go get go.yaml.in/yaml/v4@latest
go get github.com/urfave/cli/v3@latest
go get github.com/WQGroup/logger@latest
go get github.com/gregdel/pushover@latest

# Development tools (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## uv Package Manager Integration

The **uv** package manager (by Astral) is used to update Python tools. Key commands:

| Command | Purpose |
|---------|---------|
| `uv tool upgrade <package>` | Upgrade a specific tool installed via uv |
| `uv tool install <package>` | Install a new tool |
| `uv self update` | Update uv itself |
| `uv --version` | Check installed version |

Execution pattern in Go:
```go
cmd := exec.Command("uv", "tool", "upgrade", "nanobot")
output, err := cmd.CombinedOutput()
if err != nil {
    log.Error("uv upgrade failed: %s", string(output))
    // Send Pushover notification
}
```

## Alternatives Considered

| Category | Recommended | Alternative | When to Use Alternative |
|----------|-------------|-------------|-------------------------|
| Service | kardianos/service | golang.org/x/sys/windows/svc | Use x/sys/windows/svc when you need lower-level control and are willing to handle Windows service callbacks manually. More complex but no external dependency. |
| Service | kardianos/service | judwhite/go-svc | Simpler alternative with 80 forks, good Linux compatibility. Choose if you prefer minimal API surface and don't need all platform features. |
| CLI | urfave/cli | spf13/cobra | Use Cobra (43k stars) for complex CLIs with many subcommands, generated documentation needs, or when building tools similar to kubectl/docker CLI patterns. More dependencies (~15) but more features. |
| CLI | urfave/cli | flag (stdlib) | Use standard library flag package for the simplest CLIs with no subcommand needs. Zero dependencies but limited features. |
| Cron | robfig/cron | aptible/supercronic | Use Supercronic for containerized environments where you want crontab-compatible syntax with graceful shutdown and structured logging for containers. |
| YAML | go.yaml.in/yaml/v4 | gopkg.in/yaml.v3 | Legacy import path still works but v4 (go.yaml.in) is the actively maintained version with bug fixes and new features. |
| Logging | WQGroup/logger | rs/zerolog | Use Zerolog for new projects without the WQGroup constraint. Fastest structured logger (30 ns/op, 0 allocs), chainable API, excellent DX. |
| Logging | WQGroup/logger | uber-go/zap | Use Zap for high-throughput systems where GC pauses must be minimized. 71 ns/op, 0 allocs, production-proven at Uber scale. |
| Logging | WQGroup/logger | log/slog (stdlib) | Use Go 1.21+ standard library slog for new projects wanting zero external dependencies. Native JSON support, good performance (174 ns/op). |
| Notifications | gregdel/pushover | nikoksr/notify | Use notify library when you need multi-channel notifications (Slack, Discord, Telegram, email) beyond just Pushover. Abstraction layer over 20+ services. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| sirupsen/logrus directly | Logrus is in maintenance-mode (no new features), 2231 ns/op (slowest among modern loggers), 23 allocs/op. Acceptable only when wrapped by WQGroup/logger which adds rotation/cleanup. | WQGroup/logger (required) or zerolog/zap for new projects |
| gopkg.in/yaml.v2 | Outdated, missing v3/v4 features like strict field validation and streaming support | go.yaml.in/yaml/v4 |
| robfig/cron v1/v2 | Lacks v3 improvements: Go modules support, job wrappers, panic recovery, structured logging | robfig/cron/v3 |
| urfave/cli v1/v2 | v3 has improved API, better shell completion, and active maintenance | urfave/cli/v3 |
| github.com/go-ole/go-ole for Windows services | Overly complex for this use case, requires deep COM knowledge | kardianos/service |
| exec.Command without timeout | Can hang indefinitely if uv process stalls | exec.CommandContext with context.WithTimeout |

## Stack Patterns by Variant

**If you need HTTP health checks:**
- Add: `net/http` (stdlib) for health endpoint
- Because: Services should expose health status for monitoring
- Pattern: Simple `/health` endpoint returning 200 when running

**If you need Windows event log integration:**
- Add: `golang.org/x/sys/windows/svc/eventlog`
- Because: Windows Event Log is standard for service diagnostics
- Integrate with WQGroup/logger via custom hook

**If you need graceful shutdown:**
- Add: `os/signal` and `context` (stdlib)
- Because: Services should handle SIGTERM/SIGINT cleanly
- Pattern: context cancellation propagated to cron jobs

**If you need configuration hot-reload:**
- Add: `github.com/fsnotify/fsnotify`
- Because: Allow config changes without service restart
- Pattern: Watch config.yaml, reload on modify

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| Go 1.23+ | kardianos/service v1.2.4 | Requires golang.org/x/sys v0.34.0 as dependency |
| robfig/cron v3 | Go 1.18+ | Uses generics in some optional features |
| urfave/cli v3 | Go 1.18+ | Requires Go modules |
| go.yaml.in/yaml/v4 | Go 1.19+ | Pure Go, no CGO required |
| WQGroup/logger | sirupsen/logrus, lestrrat-go/file-rotatelogs, t-tomalak/logrus-easy-formatter | Has internal dependencies on these packages |

## Architecture Integration

The stack components integrate as follows:

```
+------------------+
|   CLI Entry      | <- urfave/cli parses arguments, shows help
+--------+---------+
         |
+--------v---------+
| Service Manager  | <- kardianos/service handles Windows service lifecycle
+--------+---------+
         |
+--------v---------+
|  Cron Scheduler  | <- robfig/cron manages scheduled update jobs
+--------+---------+
         |
    +----+----+
    |         |
+---v---+ +---v--------+
|Logger | |  Update    | <- WQGroup/logger + exec.Command(uv)
+-------+ +-----+------+
                |
          +-----v------+
          | Pushover   | <- gregdel/pushover (on failure only)
          +------------+
```

## Sources

- `/robfig/cron` (Context7) - Cron expressions, timezone support, job management patterns
- `/yaml/go-yaml` (Context7) - YAML unmarshal, strict field validation, streaming API
- `/urfave/cli` (Context7) - CLI structure, flags, subcommands, shell completion
- `/sirupsen/logrus` (Context7) - Logrus formatters, hooks, log levels
- https://github.com/kardianos/service - Cross-platform service management, Windows service support (HIGH confidence)
- https://github.com/gregdel/pushover - Pushover Go API wrapper documentation (HIGH confidence)
- https://github.com/WQGroup/logger - Project-specified logger based on logrus (HIGH confidence)
- https://docs.astral.sh/uv/reference/cli/ - uv CLI commands reference (HIGH confidence)
- https://betterstack.com/community/guides/logging/best-golang-logging-libraries/ - Logging library benchmarks and comparisons (MEDIUM confidence)
- https://pkg.go.dev/github.com/kardianos/service - Package documentation (HIGH confidence)

---
*Stack research for: Windows Background Service / CLI Tool for Auto-Updating Python Tools*
*Researched: 2025-02-18*
