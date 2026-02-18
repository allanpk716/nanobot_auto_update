# Phase 01: Infrastructure - Research

**Researched:** 2026-02-18
**Domain:** Go application infrastructure (logging, configuration, CLI, subprocess execution)
**Confidence:** HIGH

## Summary

This phase establishes the foundational infrastructure for a Windows-only Go application that automates nanobot updates. The research covers four key domains: structured logging with file rotation, YAML configuration management, command-line flag parsing, and safe subprocess execution on Windows.

The Go ecosystem provides mature, well-supported libraries for each domain. The standard library `log/slog` package (Go 1.21+) handles structured logging, `lumberjack` provides log rotation, `viper` manages configuration, and `pflag` handles POSIX-style command-line flags. Windows subprocess hiding is already implemented using `golang.org/x/sys/windows.SysProcAttr`.

**Primary recommendation:** Use the standard library slog with lumberjack for logging, viper with pflag integration for configuration/CLI, and extend existing config patterns with cron field support.

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFR-01 | Program supports custom log format output (2024-01-01 12:00:00.123 - [INFO]: message) | slog TextHandler with ReplaceAttr for custom time format |
| INFR-02 | Logs stored in ./logs/ directory with 24-hour rotation, keeping 7 days | lumberjack.Logger with MaxAge=7, rotation managed by MaxSize or time-based |
| INFR-03 | Load configuration from ./config.yaml | viper.SetConfigName/SetConfigType/AddConfigPath pattern |
| INFR-04 | Configuration file supports cron field (default "0 3 * * *") | Add Cron string field to Config struct, validate with robfig/cron parser |
| INFR-05 | Support -config flag to specify config file path | pflag.StringVar + viper.BindPFlag pattern |
| INFR-06 | Support -cron flag to override cron expression in config | pflag.StringVar + viper.BindPFlag pattern |
| INFR-07 | Support -run-once flag to execute one update and exit | pflag.BoolVar for run-once mode detection |
| INFR-08 | Support -version flag to display version info | pflag.BoolVar, print version and exit before other processing |
| INFR-09 | Support help flag to display usage information | pflag.BoolVarP with shorthand -h, --help |
| INFR-10 | Hide command window when executing uv commands | Already implemented: windows.SysProcAttr with HideWindow=true, CREATE_NO_WINDOW |

</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| log/slog | Go 1.21+ (stdlib) | Structured logging | Official Go standard library, zero dependencies,高性能 |
| gopkg.in/natefinch/lumberjack.v2 | v2.0 | Log file rotation | De-facto standard for Go log rotation, io.Writer interface |
| github.com/spf13/viper | v1.20+ | Configuration management | Most popular config library, supports YAML/env/flags |
| github.com/spf13/pflag | v1.0+ | CLI flag parsing | POSIX/GNU-style flags, compatible with flag package |
| github.com/robfig/cron/v3 | v3.0+ | Cron expression parsing | Most popular cron library for Go, validates expressions |
| golang.org/x/sys/windows | latest | Windows syscall support | Already in use for CREATE_NO_WINDOW |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/mitchellh/mapstructure | v1.5+ | Struct unmarshaling | When using viper.Unmarshal with config structs |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| log/slog | zap, zerolog | slog is stdlib, sufficient for requirements; zap/zerolog add complexity |
| lumberjack | custom rotation | lumberjack handles edge cases (compression, backup management) |
| viper | koanf | viper has larger ecosystem, better pflag integration |
| pflag | cobra | cobra overkill for single-command app; pflag is simpler |

**Installation:**
```bash
go get gopkg.in/natefinch/lumberjack.v2
go get github.com/spf13/viper
go get github.com/spf13/pflag
go get github.com/robfig/cron/v3
```

## Architecture Patterns

### Recommended Project Structure

```
nanobot_auto_update/
├── cmd/
│   └── main.go              # Entry point, CLI parsing, initialization
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, loading, validation
│   ├── logging/
│   │   └── logging.go       # Logger initialization with rotation
│   └── lifecycle/           # (already exists)
│       ├── detector.go
│       ├── stopper.go
│       ├── starter.go
│       └── manager.go
├── logs/                    # Log directory (created at runtime)
├── config.yaml              # Configuration file
├── go.mod
└── go.sum
```

### Pattern 1: Slog with Lumberjack Integration

**What:** Combine Go's standard structured logging with rotating file output.

**When to use:** All application logging that needs file persistence with rotation.

**Example:**
```go
// Source: https://medium.com/@piusalfred/logs-rotation-using-golang-slog-package-9579621c7ed9
package logging

import (
    "io"
    "log/slog"
    "os"

    "gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger creates a slog logger with file rotation.
// Format: 2024-01-01 12:00:00.123 - [INFO]: message
func NewLogger(logDir string) *slog.Logger {
    // Ensure log directory exists
    if err := os.MkdirAll(logDir, 0755); err != nil {
        panic(err)
    }

    // Configure lumberjack for rotation
    logFile := &lumberjack.Logger{
        Filename:   logDir + "/app.log",
        MaxSize:    100, // MB - rotation by size
        MaxBackups: 3,   // Number of old log files
        MaxAge:     7,   // Days - retention policy
        Compress:   false,
        LocalTime:  true,
    }

    // Custom handler with specific time format
    opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
        ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
            // Format time as: 2024-01-01 12:00:00.123
            if a.Key == slog.TimeKey {
                if t, ok := a.Value.Any().(time.Time); ok {
                    a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
                }
            }
            return a
        },
    }

    // Write to both file and stdout
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    handler := slog.NewTextHandler(multiWriter, opts)

    return slog.New(handler)
}
```

### Pattern 2: Viper with Pflag Integration

**What:** Unified configuration management with CLI flag support and YAML file loading.

**When to use:** When application needs config file with CLI overrides.

**Example:**
```go
// Source: https://github.com/spf13/viper
package config

import (
    "fmt"
    "time"

    flag "github.com/spf13/pflag"
    "github.com/spf13/viper"
)

type Config struct {
    Cron    string        `mapstructure:"cron"`
    Nanobot NanobotConfig `mapstructure:"nanobot"`
}

type NanobotConfig struct {
    Port           uint32        `mapstructure:"port"`
    StartupTimeout time.Duration `mapstructure:"startup_timeout"`
}

// Load reads configuration from file and flags
func Load() (*Config, error) {
    // Define CLI flags
    configFile := flag.String("config", "./config.yaml", "Path to config file")
    cronExpr := flag.String("cron", "", "Cron expression (overrides config file)")
    runOnce := flag.Bool("run-once", false, "Run update once and exit")
    showVersion := flag.Bool("version", false, "Show version information")
    flag.BoolP("help", "h", false, "Show help")

    flag.Parse()

    // Handle --version
    if *showVersion {
        fmt.Println("nanobot-auto-updater v1.0.0")
        return nil, ErrVersionRequested
    }

    // Handle --help
    if help, _ := flag.CommandLine.GetBool("help"); help {
        flag.PrintDefaults()
        return nil, ErrHelpRequested
    }

    // Setup viper
    v := viper.New()
    v.SetConfigFile(*configFile)
    v.SetConfigType("yaml")

    // Set defaults
    v.SetDefault("cron", "0 3 * * *")

    // Read config file
    if err := v.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    // Bind CLI flags to viper (CLI takes precedence)
    if err := v.BindPFlag("cron", flag.Lookup("cron")); err != nil {
        return nil, err
    }

    // Unmarshal to struct
    cfg := &Config{}
    if err := v.Unmarshal(cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    return cfg, nil
}
```

### Pattern 3: Cron Expression Validation

**What:** Validate cron expressions at startup to fail fast.

**When to use:** When loading cron expression from config or CLI.

**Example:**
```go
// Source: https://context7.com/robfig/cron/llms.txt
package config

import (
    "fmt"

    "github.com/robfig/cron/v3"
)

func ValidateCron(expr string) error {
    parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
    schedule, err := parser.Parse(expr)
    if err != nil {
        return fmt.Errorf("invalid cron expression %q: %w", expr, err)
    }
    // Schedule is valid
    _ = schedule
    return nil
}
```

### Pattern 4: Custom Slog Format Handler

**What:** Create a custom text handler for the specific format requirement.

**When to use:** When TextHandler's ReplaceAttr is insufficient.

**Example:**
```go
// For format: 2024-01-01 12:00:00.123 - [INFO]: message
// This can be achieved with ReplaceAttr in TextHandler
// No custom handler needed

opts := &slog.HandlerOptions{
    Level: slog.LevelInfo,
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        if a.Key == slog.TimeKey {
            if t, ok := a.Value.Any().(time.Time); ok {
                // Custom time format with milliseconds
                a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
            }
        }
        if a.Key == slog.LevelKey {
            // Format level as [INFO], [WARN], [ERROR]
            level := a.Value.Any().(slog.Level)
            a.Value = slog.StringValue(fmt.Sprintf("[%s]", level.String()))
        }
        return a
    },
}
```

### Anti-Patterns to Avoid

- **Using log package instead of slog:** log is unstructured, harder to parse and analyze
- **Embedding slog.Handler in custom handler:** Loggers and handlers are tightly coupled; implement all four methods
- **Not handling flag.Parse() errors:** Can cause confusing behavior with invalid flags
- **Global logger without initialization:** Makes testing difficult; use dependency injection
- **Hardcoded paths in production:** Always allow config file path override

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Log rotation | Custom file rotation logic | lumberjack | Handles edge cases: file locks, compression, backup naming, cross-platform |
| Config loading | Manual YAML parsing | viper | Handles defaults, env vars, CLI flags, multiple formats, hot reload |
| Flag parsing | Custom argument parsing | pflag | Handles shorthands, types, help generation, POSIX compliance |
| Cron validation | Regex-based cron checking | robfig/cron | Handles all cron syntax, timezones, intervals |

**Key insight:** These infrastructure problems have been solved many times. Custom implementations introduce subtle bugs around edge cases (file locking, timezone handling, escaping).

## Common Pitfalls

### Pitfall 1: 24-Hour Rotation vs MaxAge Confusion

**What goes wrong:** Expecting lumberjack to rotate exactly every 24 hours, but it rotates by size (MaxSize).

**Why it happens:** Lumberjack's MaxAge controls retention (how long to keep old files), not rotation frequency. MaxSize triggers rotation.

**How to avoid:** For daily rotation, set MaxSize to a value that will be reached in ~24 hours based on log volume. Or use a time-based rotation library like `github.com/lestrrat-go/file-rotatelogs`.

**Warning signs:** Log files larger than expected, or no rotation happening.

**Resolution for INFR-02:** Since requirement is "24-hour rotation, keeping 7 days", set:
- `MaxAge: 7` (keeps 7 days of logs)
- Set `MaxSize` appropriately for log volume to trigger daily rotation
- Or accept size-based rotation with 7-day retention

### Pitfall 2: Flag Precedence Not Applied

**What goes wrong:** Config file values override CLI flags instead of vice versa.

**Why it happens:** Calling viper.BindPFlag after setting defaults or reading config.

**How to avoid:** Always bind flags AFTER setting defaults but BEFORE checking viper.Get values. Use `viper.BindPFlag` for proper precedence (flag > env > config > default).

```go
// Correct order:
v.SetDefault("cron", "0 3 * * *")  // 1. Set defaults
v.ReadInConfig()                    // 2. Read config file
v.BindPFlag("cron", flag.Lookup("cron")) // 3. Bind flags (highest precedence)
```

### Pitfall 3: Log Directory Not Created

**What goes wrong:** Application crashes when trying to write to ./logs/ if directory doesn't exist.

**Why it happens:** lumberjack creates the log file but not parent directories.

**How to avoid:** Always create log directory before initializing logger:

```go
if err := os.MkdirAll("./logs", 0755); err != nil {
    return fmt.Errorf("failed to create log directory: %w", err)
}
```

### Pitfall 4: Cron Expression Not Validated Early

**What goes wrong:** Invalid cron expression causes runtime panic when scheduler starts.

**Why it happens:** Not validating cron expression at startup.

**How to avoid:** Validate cron expression immediately after loading config:

```go
if err := ValidateCron(cfg.Cron); err != nil {
    return fmt.Errorf("invalid cron configuration: %w", err)
}
```

### Pitfall 5: Version/Help Flags Still Run Main Logic

**What goes wrong:** --version or --help prints info but then continues to execute main logic.

**Why it happens:** Not returning early after handling these flags.

**How to avoid:** Return sentinel errors or use os.Exit(0) immediately:

```go
if *showVersion {
    fmt.Println("v1.0.0")
    os.Exit(0)  // or return ErrVersionRequested
}
```

## Code Examples

Verified patterns from official sources:

### Slog with Custom Time Format

```go
// Source: Go standard library + Context7 research
package main

import (
    "log/slog"
    "os"
    "time"
)

func main() {
    opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
        ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
            if a.Key == slog.TimeKey {
                t := a.Value.Any().(time.Time)
                a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
            }
            return a
        },
    }
    handler := slog.NewTextHandler(os.Stdout, opts)
    logger := slog.New(handler)

    logger.Info("Application started", "version", "1.0.0")
    // Output: time=2024-01-01 12:00:00.123 level=INFO msg=Application started version=1.0.0
}
```

### Pflag with Help and Version

```go
// Source: https://context7.com/spf13/pflag/llms.txt
package main

import (
    "fmt"
    "os"

    flag "github.com/spf13/pflag"
)

var (
    version = "1.0.0"
)

func main() {
    configFile := flag.String("config", "./config.yaml", "Path to config file")
    cronExpr := flag.String("cron", "", "Cron expression (overrides config file)")
    runOnce := flag.Bool("run-once", false, "Run update once and exit")
    showVersion := flag.Bool("version", false, "Show version information")

    flag.BoolP("help", "h", false, "Show help")
    flag.Parse()

    if *showVersion {
        fmt.Printf("nanobot-auto-updater %s\n", version)
        os.Exit(0)
    }

    if help, _ := flag.CommandLine.GetBool("help"); help {
        fmt.Println("Usage: nanobot-auto-updater [options]")
        fmt.Println("\nOptions:")
        flag.PrintDefaults()
        os.Exit(0)
    }

    // Main logic here...
    fmt.Printf("Config: %s, Cron: %s, RunOnce: %v\n", *configFile, *cronExpr, *runOnce)
}
```

### Viper YAML Config Loading

```go
// Source: https://github.com/spf13/viper
package main

import (
    "fmt"
    "log"

    "github.com/spf13/viper"
)

type Config struct {
    Cron string `mapstructure:"cron"`
}

func main() {
    v := viper.New()
    v.SetConfigName("config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    v.SetDefault("cron", "0 3 * * *")

    if err := v.ReadInConfig(); err != nil {
        log.Fatalf("Failed to read config: %v", err)
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        log.Fatalf("Failed to unmarshal config: %v", err)
    }

    fmt.Printf("Cron: %s\n", cfg.Cron)
}
```

### Lumberjack with Slog

```go
// Source: https://pkg.go.dev/gopkg.in/natefinch/lumberjack.v2
package main

import (
    "io"
    "log/slog"
    "os"

    "gopkg.in/natefinch/lumberjack.v2"
)

func main() {
    // Create logs directory
    os.MkdirAll("./logs", 0755)

    // Configure rotation
    logFile := &lumberjack.Logger{
        Filename:   "./logs/app.log",
        MaxSize:    100, // MB
        MaxBackups: 3,
        MaxAge:     7, // days
        Compress:   false,
        LocalTime:  true,
    }

    // Write to both file and stdout
    multiWriter := io.MultiWriter(os.Stdout, logFile)
    handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    logger := slog.New(handler)

    logger.Info("Application started")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| log package (unstructured) | log/slog (structured) | Go 1.21 (2023) | Better parsing, filtering, analysis |
| Custom flag parsing | pflag library | ~2015 | POSIX compliance, shorthand support |
| Manual YAML parsing | viper with mapstructure | ~2014 | Type-safe config, multiple sources |
| Custom log rotation | lumberjack | ~2014 | Battle-tested, handles edge cases |

**Deprecated/outdated:**
- `github.com/Sirupsen/logrus`: Now `sirupsen/logrus`, but prefer slog for new projects
- `golang.org/x/exp/slog`: Merged into stdlib in Go 1.21, use `log/slog`

## Open Questions

1. **Time-based vs Size-based Rotation**
   - What we know: Lumberjack rotates by size (MaxSize), not time
   - What's unclear: Requirement says "24-hour rotation" - is this strict time-based or daily approximation?
   - Recommendation: Start with size-based rotation (MaxSize with MaxAge=7). If strict time-based rotation needed, consider `github.com/lestrrat-go/file-rotatelogs`.

2. **Log Format Exact Match**
   - What we know: Required format is "2024-01-01 12:00:00.123 - [INFO]: message"
   - What's unclear: Standard TextHandler outputs "time=... level=... msg=..."
   - Recommendation: Use ReplaceAttr to customize time format. For exact format match (with " - " separators), may need custom handler or post-processing.

## Sources

### Primary (HIGH confidence)
- Context7 /natefinch/lumberjack - Logger configuration, Write method, rotation
- Context7 /spf13/pflag - Flag definition, parsing, shorthands
- Context7 /spf13/viper - Config file reading, pflag binding, unmarshaling
- Context7 /robfig/cron - Cron expression parsing, validation, scheduling
- Go standard library log/slog documentation - Handler interface, ReplaceAttr

### Secondary (MEDIUM confidence)
- Medium article: "Logs rotation using Golang slog package" (2024) - slog+lumberjack integration
- GitHub jordan-rash/slog-handler - Custom time format example
- SigNoz guide: "Complete Guide to Logging in Golang with slog" (2024) - Handler customization
- Official Viper README - Flag binding patterns

### Tertiary (LOW confidence)
- Various blog posts on viper/pflag integration patterns (verified against official docs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries are mature, well-documented, with official sources
- Architecture: HIGH - Patterns are well-established in Go community
- Pitfalls: HIGH - Common issues well-documented in issues and discussions

**Research date:** 2026-02-18
**Valid until:** 30 days - Stack is stable, patterns unlikely to change significantly
