package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/HQGroup/nanobot-auto-updater/internal/api"
	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/health"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
	"github.com/HQGroup/nanobot-auto-updater/internal/network"
	"github.com/HQGroup/nanobot-auto-updater/internal/notification"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
	"github.com/robfig/cron/v3"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	// Define CLI flags using pflag
	configFile := flag.String("config", "./config.yaml", "Path to config file")
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
		fmt.Println("\nArchitecture: v0.3 HTTP API + Monitor Service")
		fmt.Println("  HTTP API: http://localhost:8080/api/v1/trigger-update")
		fmt.Println("  Authentication: Bearer Token (configured in config.yaml)")
		os.Exit(0)
	}

	// Load configuration with validation (CONF-06)
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPlease check your config.yaml file.\n")
		fmt.Fprintf(os.Stderr, "Required fields:\n")
		fmt.Fprintf(os.Stderr, "  - api.bearer_token (at least 32 characters for security)\n")
		fmt.Fprintf(os.Stderr, "Optional fields (have defaults):\n")
		fmt.Fprintf(os.Stderr, "  - api.port (default: 8080)\n")
		fmt.Fprintf(os.Stderr, "  - api.timeout (default: 30s)\n")
		fmt.Fprintf(os.Stderr, "  - monitor.interval (default: 15m)\n")
		fmt.Fprintf(os.Stderr, "  - monitor.timeout (default: 10s)\n")
		fmt.Fprintf(os.Stderr, "  - pushover.api_token (optional, for notifications)\n")
		fmt.Fprintf(os.Stderr, "  - pushover.user_key (optional, for notifications)\n")
		os.Exit(1)
	}

	// Create logs directory
	if err := os.MkdirAll("./logs", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logs directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := logging.NewLogger("./logs")
	slog.SetDefault(logger) // Set as default logger

	// Log configuration loaded (CONF-06, SEC-02)
	// Note: Do NOT log full Bearer Token for security
	slog.Info("Configuration loaded and validated",
		"api_port", cfg.API.Port,
		"api_timeout", cfg.API.Timeout,
		"monitor_interval", cfg.Monitor.Interval,
		"monitor_timeout", cfg.Monitor.Timeout,
		"bearer_token_configured", cfg.API.BearerToken != "",
		"bearer_token_length", len(cfg.API.BearerToken),
	)

	slog.Info("Application starting",
		"version", Version,
		"config", *configFile,
	)

	// Create UpdateLogger with file persistence (STORE-01, D-04)
	updateLogger := updatelog.NewUpdateLogger(logger, "./logs/updates.jsonl")

	// Startup cleanup: remove logs older than 7 days (STORE-02, D-06)
	if err := updateLogger.CleanupOldLogs(); err != nil {
		logger.Error("Failed to cleanup old update logs", "error", err)
		// Non-fatal: continue without cleanup
	}

	// Load history from JSONL file into memory (Phase 32: D-01)
	// Must run after CleanupOldLogs to ensure only valid records are loaded
	if err := updateLogger.LoadFromFile(); err != nil {
		logger.Error("Failed to load update logs from file", "error", err)
		// Non-fatal: continue with empty logs
	}

	// Schedule daily log cleanup at 3 AM (STORE-02, D-06)
	cleanupCron := cron.New()
	cleanupCron.AddFunc("0 3 * * *", func() {
		if err := updateLogger.CleanupOldLogs(); err != nil {
			logger.Error("Scheduled update log cleanup failed", "error", err)
		}
	})
	cleanupCron.Start()
	logger.Info("Update log cleanup scheduler started", "schedule", "0 3 * * *")

	// Create InstanceManager
	instanceManager := instance.NewInstanceManager(cfg, logger)

	// Create Notifier (MONITOR-04, MONITOR-05, UNOTIF-01, UNOTIF-02)
	// Created before API server so it can be injected into TriggerHandler
	notif := notifier.NewWithConfig(
		notifier.Config{
			ApiToken: cfg.Pushover.ApiToken,
			UserKey:  cfg.Pushover.UserKey,
		},
		logger,
	)

	// Create self-update Updater (Phase 39)
	selfUpdater := selfupdate.NewUpdater(
		selfupdate.SelfUpdateConfig{
			GithubOwner: cfg.SelfUpdate.GithubOwner,
			GithubRepo:  cfg.SelfUpdate.GithubRepo,
		},
		logger,
	)

	// Create API server (SSE-07: WriteTimeout=0)
	var apiServer *api.Server
	if cfg.API.Port != 0 {
		var err error
		apiServer, err = api.NewServer(&cfg.API, instanceManager, cfg, Version, logger, updateLogger, notif, selfUpdater)
		if err != nil {
			logger.Error("Failed to create API server", "error", err)
			os.Exit(1)
		}

		// Start API server in goroutine
		go func() {
			logger.Info("启动 API 服务器", "port", cfg.API.Port)
			if err := apiServer.Start(); err != nil {
				logger.Error("API 服务器错误", "error", err)
			}
		}()
	}

	// Start health monitor for all instances (HEALTH-01, HEALTH-04)
	var healthMonitor *health.HealthMonitor
	if len(cfg.Instances) > 0 {
		healthMonitor = health.NewHealthMonitor(
			cfg.Instances,
			cfg.HealthCheck.Interval,
			logger,
		)
		go healthMonitor.Start()
		logger.Info("健康监控已启动", "interval", cfg.HealthCheck.Interval)
	}

	// Start network monitor (MONITOR-01, MONITOR-06)
	networkMonitor := network.NewNetworkMonitor(
		"https://www.google.com",
		cfg.Monitor.Interval,
		cfg.Monitor.Timeout,
		logger,
	)
	go networkMonitor.Start()
	logger.Info("网络监控已启动", "interval", cfg.Monitor.Interval)

	// Start notification manager (MONITOR-04, MONITOR-05)
	notificationManager := notification.NewNotificationManager(
		networkMonitor,
		notif,
		logger,
	)
	go notificationManager.Start(cfg.Monitor.Interval)
	logger.Info("通知管理器已启动", "check_interval", cfg.Monitor.Interval)

	// Auto-start instances in goroutine (non-blocking)
	// AUTOSTART-01: Application starts all configured instances at startup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("自动启动 goroutine panic",
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()

		// Create context with timeout for auto-start
		// Total timeout: 5 minutes (adjust based on instance count)
		autoStartTimeout := 5 * time.Minute
		autoStartCtx, cancel := context.WithTimeout(context.Background(), autoStartTimeout)
		defer cancel()

		logger.Info("开始自动启动所有实例",
			"instance_count", len(cfg.Instances),
			"timeout", autoStartTimeout)

		// Execute auto-start
		instanceManager.StartAllInstances(autoStartCtx)
	}()

	// Setup graceful shutdown signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutdown signal received")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop notification manager first (before network monitor)
	if notificationManager != nil {
		notificationManager.Stop()
	}

	// Stop network monitor
	if networkMonitor != nil {
		networkMonitor.Stop()
	}

	// Stop health monitor
	if healthMonitor != nil {
		healthMonitor.Stop()
	}

	// Stop cleanup cron scheduler
	cleanupCron.Stop()
	logger.Info("Update log cleanup scheduler stopped")

	// Close UpdateLogger file handle (D-05)
	if err := updateLogger.Close(); err != nil {
		logger.Error("Failed to close update logger", "error", err)
	}

	// Shutdown API server
	if apiServer != nil {
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("API server shutdown error", "error", err)
		}
	}

	logger.Info("Shutdown completed")
}
