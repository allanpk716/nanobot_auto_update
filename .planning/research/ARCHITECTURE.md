# Architecture Research

**Domain:** Windows Background Service / CLI Tool (Go)
**Researched:** 2025-02-18
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
+-------------------------------------------------------------+
|                    Entry Point Layer                         |
|  +-------------------------------------------------------+  |
|  |              cmd/nanobot-auto-update/                  |  |
|  |  +---------+  +---------+  +----------+  +----------+  |  |
|  |  | main.go |  | root.go |  | run.go   |  | install.go|  |  |
|  |  +----+----+  +----+----+  +-----+----+  +-----+----+  |  |
|  +-------|----------|---------------|-------------|-------+  |
+----------|----------|---------------|-------------|----------+
           |          |               |             |
+----------|----------|---------------|-------------|----------+
|          v          v               v             v          |
|                    Core Application Layer                     |
|  +-------------------------------------------------------+  |
|  |              internal/app/service.go                   |  |
|  |  +--------------------------------------------------+  |  |
|  |  |              Service Interface                    |  |  |
|  |  |  Start() | Stop() | Run(ctx context.Context)     |  |  |
|  |  +--------------------------------------------------+  |  |
|  +-------------------------------------------------------+  |
|                                                              |
|  +------------+  +-------------+  +----------+  +---------+  |
|  | Scheduler  |  |   Config    |  |  Logger  |  | Notifier|  |
|  | (cron/v3)  |  |  (viper)    |  |  (slog)  |  |(pushover)|  |
|  +-----+------+  +------+------+  +----+-----+  +----+----+  |
|        |                |              |             |        |
+--------|----------------|--------------|-------------|--------+
         |                |              |             |
+--------|----------------|--------------|-------------|--------+
|        v                v              v             v        |
|                    Infrastructure Layer                       |
|  +------------+  +-------------+  +----------+  +---------+  |
|  |  Executor  |  |  Config     |  |  File    |  |  HTTP   |  |
|  | (os/exec)  |  |  Loader     |  |  Logger  |  | Client  |  |
|  +-----+------+  +------+------+  +----+-----+  +----+----+  |
|        |                |              |             |        |
+--------|----------------|--------------|-------------|--------+
         |                |              |             |
         v                v              v             v
    +---------+      +---------+    +---------+   +---------+
    |  uv CLI |      | YAML/ENV|    | log.txt |   | Pushover|
    | Process |      | Config  |    | stdout  |   |   API   |
    +---------+      +---------+    +---------+   +---------+
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **Entry Point (cmd/)** | Parse CLI args, initialize dependencies, start service | `spf13/cobra` for CLI structure |
| **Service Wrapper** | Bridge between OS service manager and application logic | `kardianos/service` for cross-platform support |
| **Scheduler** | Execute tasks at configured intervals | `robfig/cron/v3` with cron expressions |
| **Configuration** | Load and merge settings from multiple sources | `spf13/viper` (YAML + ENV + CLI flags) |
| **Logger** | Structured logging with configurable output | `log/slog` (Go 1.21+) or `zerolog` |
| **Command Executor** | Run external commands (`uv` tool) safely | `os/exec` with proper argument handling |
| **Notifier** | Send push notifications on events | `gregdel/pushover` library |
| **Dependency Checker** | Verify `uv` is installed and accessible | `os/exec.LookPath()` |

## Recommended Project Structure

```
nanobot-auto-update/
+-- cmd/
|   +-- nanobot-auto-update/
|       +-- main.go              # Entry point, wire dependencies
|       +-- root.go              # Root cobra command
|       +-- run.go               # Run command (foreground)
|       +-- install.go           # Install/uninstall service commands
|       +-- version.go           # Version command
|
+-- internal/
|   +-- app/
|   |   +-- service.go           # Service interface and implementation
|   |   +-- updater.go           # Core update logic
|   |   +-- checker.go           # Update availability checker
|   |
|   +-- config/
|   |   +-- config.go            # Configuration struct and loader
|   |   +-- defaults.go          # Default configuration values
|   |
|   +-- executor/
|   |   +-- executor.go          # Command execution wrapper
|   |   +-- uv.go                # uv-specific commands
|   |
|   +-- notify/
|   |   +-- notifier.go          # Notification interface
|   |   +-- pushover.go          # Pushover implementation
|   |   +-- mock.go              # Mock for testing
|   |
|   +-- scheduler/
|   |   +-- scheduler.go         # Cron scheduler wrapper
|   |   +-- jobs.go              # Job definitions
|   |
|   +-- logger/
|       +-- logger.go            # Logger initialization
|       +-- format.go            # Custom formatters
|
+-- pkg/                         # (Optional) Public packages
|
+-- configs/
|   +-- config.yaml              # Default configuration file
|   +-- config.example.yaml      # Example configuration
|
+-- .goreleaser.yml              # Release configuration
+-- go.mod
+-- go.sum
+-- main.go                      # (Alternative simple entry)
+-- Makefile
+-- README.md
```

### Structure Rationale

- **cmd/:** Standard Go convention for executable entry points. Multiple subdirectories for different binaries if needed.
- **internal/:** Enforced privacy by Go compiler. Contains all application-specific code that should not be imported externally.
- **config/:** Centralized configuration management with clear separation of loading vs. defaults.
- **executor/:** Isolates external command execution for testability and security (prevents command injection).
- **notify/:** Interface-based design allows swapping notification backends (Pushover, Slack, email) without changing core logic.
- **scheduler/:** Wraps cron library to provide application-specific job management.

## Architectural Patterns

### Pattern 1: Service Wrapper Pattern

**What:** Use `kardianos/service` to run the application as a native OS service while maintaining CLI usability.

**When to use:** When the application needs to run as a Windows service AND as a standalone CLI tool for development/debugging.

**Trade-offs:**
- Pros: Single codebase works as service and CLI; cross-platform support; proper lifecycle management
- Cons: Additional abstraction layer; service debugging requires extra steps

**Example:**
```go
package main

import (
    "context"
    "log"

    "github.com/kardianos/service"
)

type program struct {
    ctx    context.Context
    cancel context.CancelFunc
    app    *Application
}

func (p *program) Start(s service.Service) error {
    // Start must not block
    p.ctx, p.cancel = context.WithCancel(context.Background())
    go p.run()
    return nil
}

func (p *program) run() {
    // Main application logic runs here
    p.app.Run(p.ctx)
}

func (p *program) Stop(s service.Service) error {
    // Clean shutdown
    p.cancel()
    return nil
}

func main() {
    svcConfig := &service.Config{
        Name:        "NanobotAutoUpdate",
        DisplayName: "Nanobot Auto Updater",
        Description: "Automatically updates the nanobot AI agent tool",
    }

    app := NewApplication()
    prg := &program{app: app}

    s, err := service.New(prg, svcConfig)
    if err != nil {
        log.Fatal(err)
    }

    // Run as service (or interactively if not managed by service manager)
    if err := s.Run(); err != nil {
        log.Fatal(err)
    }
}
```

### Pattern 2: Configuration Hierarchy Pattern

**What:** Load configuration from multiple sources with explicit precedence: defaults < config file < environment variables < CLI flags.

**When to use:** When the application needs flexible configuration across development, testing, and production environments.

**Trade-offs:**
- Pros: Sensible defaults; environment-specific overrides; no hardcoded values
- Cons: More complex initialization; potential for confusion about value sources

**Example:**
```go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    Schedule      string `mapstructure:"schedule"`
    UVPath        string `mapstructure:"uv_path"`
    CheckOnStart  bool   `mapstructure:"check_on_start"`
    LogFile       string `mapstructure:"log_file"`
    LogLevel      string `mapstructure:"log_level"`
    Notifications NotifyConfig `mapstructure:"notifications"`
}

type NotifyConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    Provider string `mapstructure:"provider"`
    Token    string `mapstructure:"token"`
    User     string `mapstructure:"user"`
}

func Load(configPath string) (*Config, error) {
    // 1. Set defaults (lowest priority)
    viper.SetDefault("schedule", "0 */6 * * *")  // Every 6 hours
    viper.SetDefault("uv_path", "uv")
    viper.SetDefault("check_on_start", true)
    viper.SetDefault("log_level", "info")

    // 2. Config file
    if configPath != "" {
        viper.SetConfigFile(configPath)
    } else {
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
        viper.AddConfigPath(".")
        viper.AddConfigPath("$HOME/.nanobot-auto-update")
        viper.AddConfigPath("/etc/nanobot-auto-update")
    }

    // 3. Environment variables (override config file)
    viper.SetEnvPrefix("NANOBOT")
    viper.AutomaticEnv()

    // Read config
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
        }
        // Config file not found is okay, use defaults/env
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

### Pattern 3: Repository/Interface Pattern for External Services

**What:** Define interfaces for external dependencies (notifications, command execution) to enable testing with mocks.

**When to use:** When you need to test business logic without making real API calls or executing real commands.

**Trade-offs:**
- Pros: Testable; swappable implementations; clean separation
- Cons: More boilerplate; potential over-abstraction for simple cases

**Example:**
```go
package notify

// Notifier interface allows different notification backends
type Notifier interface {
    Send(ctx context.Context, msg Message) error
}

type Message struct {
    Title   string
    Body    string
    Priority int
}

// Pushover implementation
type PushoverNotifier struct {
    client *pushover.Pushover
    recipient *pushover.Recipient
}

func (p *PushoverNotifier) Send(ctx context.Context, msg Message) error {
    message := pushover.NewMessageWithTitle(msg.Body, msg.Title)
    message.Priority = msg.Priority
    _, err := p.client.SendMessage(message, p.recipient)
    return err
}

// Mock for testing
type MockNotifier struct {
    Sent []Message
}

func (m *MockNotifier) Send(ctx context.Context, msg Message) error {
    m.Sent = append(m.Sent, msg)
    return nil
}
```

### Pattern 4: Graceful Shutdown Pattern

**What:** Handle shutdown signals properly to complete in-flight operations.

**When to use:** For any long-running service that performs operations that should complete cleanly.

**Trade-offs:**
- Pros: No data loss on shutdown; clean resource release
- Cons: Slightly more complex main loop

**Example:**
```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Handle shutdown in separate goroutine
    go func() {
        sig := <-sigChan
        log.Printf("Received signal %v, shutting down...", sig)
        cancel()
    }()

    // Run application
    if err := app.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Data Flow

### Update Check Flow

```
[Cron Scheduler]
    |
    v (trigger at scheduled time)
[Update Checker]
    |
    +--> [Executor: uv --version]
    |         |
    |         v
    |     [Current Version]
    |
    +--> [Executor: uv self update --check]
    |         |
    |         v
    |     [Available Update?]
    |
    v
[Update Available?]
    |
    +-- No --> [Log: No update needed] --> [Wait for next schedule]
    |
    +-- Yes --> [Executor: uv self update]
                   |
                   v
               [Update Result]
                   |
                   +-- Success --> [Notifier: Update successful]
                   |                    |
                   |                    v
                   |               [Logger: Update complete]
                   |
                   +-- Failure --> [Notifier: Update failed]
                                        |
                                        v
                                   [Logger: Error details]
```

### Configuration Load Flow

```
[Application Start]
    |
    v
[Viper Config Loader]
    |
    +--> [Load Defaults]
    |         |
    |         v
    |     schedule: "0 */6 * * *"
    |     uv_path: "uv"
    |     log_level: "info"
    |
    +--> [Read YAML Config File]
    |         |
    |         v
    |     Merge with defaults
    |
    +--> [Read Environment Variables]
    |         |
    |         v
    |     NANOBOT_SCHEDULE overrides
    |     NANOBOT_UV_PATH overrides
    |
    +--> [Read CLI Flags]
    |         |
    |         v
    |     --schedule overrides all
    |     --config specifies path
    |
    v
[Final Config Object]
    |
    v
[Initialize Components]
```

### Key Data Flows

1. **Configuration Flow:** CLI flags > Environment variables > Config file > Defaults. Viper handles this hierarchy automatically when properly configured.

2. **Update Execution Flow:** Scheduler triggers -> Checker validates -> Executor runs `uv self update` -> Notifier sends result -> Logger records outcome.

3. **Error Propagation Flow:** Errors bubble up from infrastructure layer through application layer to CLI layer where they are logged and optionally surfaced to user via notifications.

4. **Log Flow:** All components write to structured logger -> Logger writes to configured outputs (file, stdout, or both) with proper formatting.

## Component Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| CLI Layer <-> Core Application | Direct function calls | CLI commands instantiate and call app methods |
| Scheduler <-> Updater | Function callbacks | Scheduler invokes registered job functions |
| Updater <-> Executor | Interface methods | Executor interface enables mocking in tests |
| Updater <-> Notifier | Interface methods | Notifier interface enables different backends |
| All Components <-> Logger | Direct calls | Logger is passed via dependency injection or context |
| Config -> All Components | Struct passing | Config is loaded once and passed to constructors |

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| uv CLI | `os/exec` command execution | Must handle PATH lookup, argument escaping, timeout |
| Pushover API | HTTP via `gregdel/pushover` | Simple POST to API endpoint with token/user credentials |
| Windows SCM | `kardianos/service` | Abstracts Windows service control manager API |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| CLI <-> Service | Interface via `kardianos/service` | Same binary works interactively and as service |
| Config -> Components | Constructor injection | All components receive config at creation time |

## Build Order Recommendations

### Phase 1: Core Infrastructure (Foundation)
- **Logger** - Required by all other components for debugging
- **Configuration** - Required for all component initialization
- **Executor** - Required by updater and dependency checker

**Rationale:** These have no dependencies on other custom components and are needed by everything else.

### Phase 2: Core Application Logic
- **Dependency Checker** - Verifies uv presence (depends on Executor)
- **Update Checker** - Checks for updates (depends on Executor)
- **Updater** - Performs update (depends on Executor, Checker)
- **Service Core** - Main run loop (depends on Updater)

**Rationale:** Builds on infrastructure to implement the core update functionality.

### Phase 3: Notifications and Scheduling
- **Notifier Interface** - Define interface first
- **Pushover Notifier** - Implement interface
- **Scheduler** - Cron-based job scheduling (depends on Updater)

**Rationale:** Notifications and scheduling are independent and can be developed in parallel after core logic exists.

### Phase 4: CLI and Service Integration
- **Cobra Commands** - CLI structure (depends on all above)
- **Service Wrapper** - `kardianos/service` integration (depends on Service Core)
- **Install/Uninstall Commands** - Service management

**Rationale:** CLI ties everything together and should be last.

### Phase 5: Polish and Distribution
- **Configuration Examples** - Sample YAML files
- **Documentation** - README, usage examples
- **Release Automation** - GoReleaser configuration

**Rationale:** These depend on the complete application being functional.

## Anti-Patterns

### Anti-Pattern 1: Blocking in Service Start

**What people do:** Implement long-running logic directly in the `Start()` method of the service.

**Why it's wrong:** Windows Service Control Manager expects `Start()` to return quickly. Blocking causes timeout errors (Error 1053: "The service did not respond to the start or control request in a timely fashion").

**Do this instead:** Spawn a goroutine from `Start()` for the main logic and return immediately:
```go
func (p *program) Start(s service.Service) error {
    go p.run()  // Run in background
    return nil  // Return immediately
}
```

### Anti-Pattern 2: Command Injection via String Concatenation

**What people do:** Build command strings with user input using string concatenation or `fmt.Sprintf`.

**Why it's wrong:** Allows arbitrary command execution if input contains shell metacharacters like `; rm -rf /`.

**Do this instead:** Use `exec.Command` with separate arguments - Go handles escaping automatically:
```go
// BAD - vulnerable to injection
cmd := exec.Command("sh", "-c", "uv " + userInput)

// GOOD - arguments are safely separated
cmd := exec.Command("uv", "self", "update")
```

### Anti-Pattern 3: Hardcoded Configuration Paths

**What people do:** Hardcode configuration file paths like `/etc/myapp/config.yaml`.

**Why it's wrong:** Fails on Windows, makes testing difficult, prevents user-specific installations.

**Do this instead:** Use Viper's multi-path config search or XDG/Base Directory specifications:
```go
viper.AddConfigPath(".")
viper.AddConfigPath("$HOME/.nanobot-auto-update")
viper.AddConfigPath("/etc/nanobot-auto-update")  // Linux
viper.AddConfigPath(filepath.Join(os.Getenv("APPDATA"), "NanobotAutoUpdate"))  // Windows
```

### Anti-Pattern 4: Global Logger State

**What people do:** Use a global logger variable that components access directly.

**Why it's wrong:** Makes testing difficult, prevents per-component log levels, hides dependencies.

**Do this instead:** Pass logger via dependency injection:
```go
type Updater struct {
    logger   *slog.Logger
    executor Executor
}

func NewUpdater(logger *slog.Logger, exec Executor) *Updater {
    return &Updater{
        logger:   logger,
        executor: exec,
    }
}
```

## Scaling Considerations

| Concern | Single Instance | Multiple Instances | Notes |
|---------|-----------------|-------------------|-------|
| Update checking | Single cron schedule | Coordinator needed | Multiple instances should not all update simultaneously |
| Notification sending | Direct | Rate limiting | Pushover has rate limits; add backoff for failures |
| Logging | Local file | Centralized logging | Consider log rotation for long-running services |
| Configuration | Local file | Shared config | For complex setups, consider remote config (Consul, etcd) |

### Scaling Priorities

1. **First bottleneck:** Log file size - Implement log rotation or use rotating file handler from the start.
2. **Second bottleneck:** Notification rate limits - If sending many notifications, implement batching or rate limiting.

## Sources

- golang.org/x/sys/windows/svc - Official Go Windows service package (https://pkg.go.dev/golang.org/x/sys/windows/svc)
- github.com/kardianos/service - Cross-platform service management (https://github.com/kardianos/service)
- github.com/spf13/cobra - CLI framework documentation (https://cobra.dev)
- github.com/spf13/viper - Configuration management (https://github.com/spf13/viper)
- github.com/robfig/cron - Cron scheduling library (https://github.com/robfig/cron)
- log/slog - Go 1.21+ structured logging (https://pkg.go.dev/log/slog)
- github.com/gregdel/pushover - Pushover API client (https://github.com/gregdel/pushover)
- dev.to - "Writing a Windows Service in Go" (https://dev.to/cosmic_predator/writing-a-windows-service-in-go-1d1m)
- Medium - "Building Cross-Platform System Services in Go" (https://medium.com/@ansxuman/building-cross-platform-system-services-in-go-a-step-by-step-guide-5784f96098b4)

---
*Architecture research for: Windows Background Service / CLI Tool in Go*
*Researched: 2025-02-18*
