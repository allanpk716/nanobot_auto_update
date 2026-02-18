package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	flag "github.com/spf13/pflag"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
	"github.com/HQGroup/nanobot-auto-updater/internal/scheduler"
	"github.com/HQGroup/nanobot-auto-updater/internal/updater"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	// Define CLI flags using pflag
	configFile := flag.String("config", "./config.yaml", "Path to config file")
	cronExpr := flag.String("cron", "", "Cron expression (overrides config file)")
	runOnce := flag.Bool("run-once", false, "Run update once and exit")
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
		"run_once", *runOnce,
	)

	if *runOnce {
		logger.Info("Executing one-time update")
		u := updater.NewUpdater(logger)
		result, err := u.Update(context.Background())
		if err != nil {
			logger.Error("Update failed", "result", result, "error", err.Error())
			os.Exit(1)
		}
		logger.Info("Update completed", "result", result)
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
// 4. Test --run-once flag:
//    go run ./cmd/main.go -run-once
//    Should log: run_once=true
//
// 5. Test invalid cron:
//    go run ./cmd/main.go -cron "invalid" 2>&1
//    Should exit with error about invalid cron expression
//
// 6. Test -h/--help:
//    go run ./cmd/main.go -h
//    go run ./cmd/main.go --help
//    Both should show usage information
//
// 7. Test --version:
//    go run ./cmd/main.go --version
//    Should show version and exit immediately
