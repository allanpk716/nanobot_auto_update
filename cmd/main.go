package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
	"github.com/HQGroup/nanobot-auto-updater/internal/scheduler"
	"github.com/HQGroup/nanobot-auto-updater/internal/updater"
)

// Version is set via ldflags at build time.
var Version = "dev"

// UpdateNowResult represents the JSON output for --update-now mode
type UpdateNowResult struct {
	Success  bool   `json:"success"`
	Version  string `json:"version,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// outputJSON writes the result as JSON to stdout (last line)
func outputJSON(result UpdateNowResult) {
	output, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
		return
	}
	fmt.Println(string(output))
}

func main() {
	// Define CLI flags using pflag
	configFile := flag.String("config", "./config.yaml", "Path to config file")
	cronExpr := flag.String("cron", "", "Cron expression (overrides config file)")
	updateNow := flag.Bool("update-now", false, "Execute immediate update and exit with JSON output")
	timeout := flag.Duration("timeout", 5*time.Minute, "Update timeout duration (e.g., '5m', '300s')")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.BoolP("help", "h", false, "Show help")

	flag.Parse()

	// Handle --version (exit immediately)
	if *showVersion {
		fmt.Printf("nanobot-auto-updater %s\n", Version)
		os.Exit(0)
	}

	// Handle --help (exit immediately)
	if help, _ := flag.CommandLine.GetBool("help"); help {
		fmt.Println("Usage: nanobot-auto-updater [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		fmt.Println("\nJSON Output Format (--update-now):")
		fmt.Println("  Success: {\"success\": true, \"version\": \"X.Y.Z\", \"source\": \"github|pypi\", \"message\": \"Update completed\"}")
		fmt.Println("  Failure: {\"success\": false, \"error\": \"description\", \"exit_code\": 1}")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override cron from CLI flag if provided
	if *cronExpr != "" {
		if err := config.ValidateCron(*cronExpr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		cfg.Cron = *cronExpr
	}

	// Daemonize if running in --update-now mode (called by nanobot)
	// This ensures the updater process survives when nanobot is terminated
	if *updateNow {
		if daemonized, err := lifecycle.MakeDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to daemonize: %v\n", err)
			// Continue anyway - may work if parent doesn't exit immediately
		} else if daemonized {
			// This process will exit, daemon has been started
			return
		}
	}

	// Create logs directory
	if err := os.MkdirAll("./logs", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logs directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := logging.NewLogger("./logs")
	slog.SetDefault(logger) // Set as default logger

	// Check UV installation
	logger.Info("Checking uv installation")
	if err := updater.CheckUvInstalled(); err != nil {
		logger.Error("uv installation check failed", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("uv is installed and available")

	logger.Info("Application starting",
		"version", Version,
		"config", *configFile,
		"cron", cfg.Cron,
		"update_now", *updateNow,
		"timeout", timeout.String(),
	)

	if *updateNow {
		logger.Info("Executing immediate update", "timeout", timeout.String())

		// Create context with configurable timeout
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		// Initialize lifecycle manager
		lifecycleCfg := lifecycle.Config{
			Port:           cfg.Nanobot.Port,
			StartupTimeout: cfg.Nanobot.StartupTimeout,
		}
		lifecycleMgr := lifecycle.NewManager(lifecycleCfg)

		result := UpdateNowResult{}

		// Stop nanobot before update
		if err := lifecycleMgr.StopForUpdate(ctx); err != nil {
			logger.Error("Failed to stop nanobot", "error", err.Error())
			result = UpdateNowResult{
				Success:  false,
				Error:    fmt.Sprintf("Failed to stop nanobot: %s", err.Error()),
				ExitCode: 1,
			}
			outputJSON(result)
			os.Exit(1)
		}

		// Execute update
		u := updater.NewUpdater(logger)
		updateResult, err := u.Update(ctx)
		if err != nil {
			logger.Error("Update failed", "result", updateResult, "error", err.Error())
			result = UpdateNowResult{
				Success:  false,
				Error:    err.Error(),
				ExitCode: 1,
			}
			outputJSON(result)
			os.Exit(1)
		}

		// Start nanobot after successful update
		if err := lifecycleMgr.StartAfterUpdate(ctx); err != nil {
			// Log warning but don't fail - update was successful
			logger.Warn("Failed to start nanobot after update", "error", err.Error())
		}

		result = UpdateNowResult{
			Success: true,
			Source:  string(updateResult),
			Message: "Update completed",
		}
		logger.Info("Update completed", "result", updateResult)
		outputJSON(result)
		os.Exit(0)
	}

	// Initialize notifier (logs warning if Pushover not configured)
	notif := notifier.New(logger)

	// Initialize scheduler with overlap prevention
	sched := scheduler.New(logger)

	// Create updater instance
	u := updater.NewUpdater(logger)

	// Register the update job
	err = sched.AddJob(cfg.Cron, func() {
		logger.Info("Starting scheduled update job")

		result, err := u.Update(context.Background())
		if err != nil {
			logger.Error("Scheduled update failed",
				"result", result,
				"error", err.Error())

			// Send failure notification
			if notifyErr := notif.NotifyFailure("Scheduled Update", err); notifyErr != nil {
				logger.Error("Failed to send failure notification", "error", notifyErr.Error())
			}
			return
		}

		logger.Info("Scheduled update completed successfully", "result", result)
	})
	if err != nil {
		logger.Error("Failed to register scheduled job", "error", err.Error())
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the scheduler
	sched.Start()
	logger.Info("Scheduler started", "cron", cfg.Cron, "pid", os.Getpid())

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Shutdown signal received", "signal", sig.String())

	// Gracefully stop scheduler
	sched.Stop()
	logger.Info("Application shutdown complete")
}

// Manual Test Cases:
//
// 1. Test default config loading:
//    go run ./cmd/main.go
//    Should log: cron="0 3 * * *", config="./config.yaml"
//
// 2. Test --cron override:
//    go run ./cmd/main.go -cron "*/5 * * * *"
//    Should log: cron="*/5 * * * *" (overridden)
//
// 3. Test --config flag:
//    Create test config with cron: "0 5 * * *"
//    go run ./cmd/main.go -config test-config.yaml
//    Should log: cron="0 5 * * *" (from test config)
//
// 4. Test --update-now flag:
//    go run ./cmd/main.go --update-now
//    Should execute update and output JSON to stdout
//
// 5. Test --timeout flag:
//    go run ./cmd/main.go --update-now --timeout 2m
//    Should use 2 minute timeout for update
//
// 6. Test invalid cron:
//    go run ./cmd/main.go -cron "invalid" 2>&1
//    Should exit with error about invalid cron expression
//
// 7. Test -h/--help:
//    go run ./cmd/main.go -h
//    go run ./cmd/main.go --help
//    Both should show usage information including JSON output format
//
// 8. Test --version:
//    go run ./cmd/main.go --version
//    Should show version and exit immediately
//
