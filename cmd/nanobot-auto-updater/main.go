package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/HQGroup/nanobot-auto-updater/internal/api"
	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/health"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
	"github.com/HQGroup/nanobot-auto-updater/internal/logging"
	"github.com/HQGroup/nanobot-auto-updater/internal/network"
	"github.com/HQGroup/nanobot-auto-updater/internal/notification"
	"github.com/HQGroup/nanobot-auto-updater/internal/notifier"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
	"github.com/HQGroup/nanobot-auto-updater/internal/updatelog"
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

	// Detect Windows service mode (SVC-01, D-06)
	// This check runs before config loading -- service mode path does not need config.yaml.
	// Note: slog is NOT initialized yet at this point. Use fmt.Fprintf(os.Stderr, ...) for output.
	inService, err := lifecycle.IsServiceMode()
	if err != nil {
		// svc.IsWindowsService() returned an error (review concern #5).
		// Treat as console mode (false) with warning. Do NOT fatal exit --
		// the detection is best-effort and should not prevent the app from starting.
		fmt.Fprintf(os.Stderr, "Warning: failed to detect service mode: %v\n", err)
		// Continue as console mode
		inService = false
	}

	if inService {
		// Service mode: running under Windows SCM (D-06)
		// Phase 47 will implement full svc.Handler -- for now log and continue.
		// Using fmt.Fprintf(os.Stderr) because slog is not initialized yet.
		fmt.Fprintf(os.Stderr, "Detected Windows service mode\n")
	}

	if inService {
		// Fix working directory for service mode: SCM starts services with
		// C:\Windows\System32 as working directory. Change to exe's directory
		// so config.yaml and log files resolve correctly (Phase 49 ADPT-03).
		if exePath, err := os.Executable(); err == nil {
			if exeDir := filepath.Dir(exePath); exeDir != "" {
				os.Chdir(exeDir)
			}
		}
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

	// Check for leftover update state (D-04: .old cleanup/recovery)
	// Runs before server startup to ensure clean state
	lifecycle.CheckUpdateState(logger)

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

	// Handle service registration/unregistration based on auto_start config (MGR-02, MGR-03)

	// Case 1: Running as service but auto_start is disabled in config
	// This means the user changed config after service was registered.
	// The service will still run this session. The uninstall happens next time
	// the user runs in console mode with auto_start: false.
	if inService && (cfg.Service.AutoStart == nil || !*cfg.Service.AutoStart) {
		slog.Warn("Running as service but auto_start is not enabled in config",
			"service_name", cfg.Service.ServiceName,
			"action", "To stop running as a service: set auto_start: false in config.yaml, then run this program from a console (not as a service) to auto-uninstall",
		)
	}

	// Case 2: Console mode + auto_start: true -> register service and exit (MGR-02)
	if !inService && cfg.Service.AutoStart != nil && *cfg.Service.AutoStart {
		// Check admin privileges (D-08)
		if !lifecycle.IsAdmin() {
			slog.Error("Administrator privileges required to register Windows service",
				"hint", "Right-click the executable and select 'Run as administrator'",
			)
			os.Exit(1)
		}

		slog.Info("Registering as Windows service",
			"service_name", cfg.Service.ServiceName,
			"display_name", cfg.Service.DisplayName,
		)

		if err := lifecycle.RegisterService(cfg, logger); err != nil {
			slog.Error("Failed to register Windows service", "error", err)
			os.Exit(1)
		}

		slog.Info("Service registered successfully. Use Windows Services Manager to start it, or restart the computer.")
		os.Exit(2) // Exit code 2: service registered (signals to calling scripts)
	}

	// Case 3: Console mode + auto_start: false -> check and uninstall existing service (MGR-03)
	if !inService && (cfg.Service.AutoStart == nil || !*cfg.Service.AutoStart) {
		if err := lifecycle.UnregisterService(context.Background(), cfg, logger); err != nil {
			slog.Warn("Failed to unregister service (may not exist or insufficient privileges)",
				"error", err,
			)
			// Non-fatal: continue to console mode (D-06)
		} else {
			slog.Info("Service uninstalled, switched to console mode",
				"service_name", cfg.Service.ServiceName,
			)
		}
		// Continue to normal console mode execution (D-06)
	}

	slog.Info("Application starting",
		"version", Version,
		"config", *configFile,
	)

	// Shared mutable state for hot-reloadable values.
	// The token is read by AuthMiddleware on every request via a closure.
	// Updated by OnBearerTokenChange callback when config changes.
	// Protected by tokenMu to prevent data race between HTTP handler
	// goroutines (readers) and hot-reload timer goroutine (writer).
	var tokenMu sync.RWMutex
	currentBearerToken := cfg.API.BearerToken

	// Create components that require circular-dependency packages (D-05, D-10)
	// lifecycle.AppStartup cannot import api/instance/notifier/updatelog directly
	// due to circular imports, so main.go creates these and passes them in.
	updateLogger := updatelog.NewUpdateLogger(logger, "./logs/updates.jsonl")
	notif := notifier.NewWithConfig(
		notifier.Config{
			ApiToken: cfg.Pushover.ApiToken,
			UserKey:  cfg.Pushover.UserKey,
		},
		logger,
	)

	// createComponents creates the API server, health monitor, and instance manager.
	// These packages (api, health, instance) cannot be imported by the lifecycle package
	// due to circular import constraints.
	createComponents := func(
		cfg *config.Config,
		logger *slog.Logger,
		version string,
		notif any,
		updateLogger any,
		selfUpdater *selfupdate.Updater,
	) (instanceManager any, healthMonitor lifecycle.HealthMonitorControl, apiServer lifecycle.APIServerControl, err error) {
		// Cast parameters back to concrete types
		concreteNotif := notif.(*notifier.Notifier)
		concreteUpdateLogger := updateLogger.(*updatelog.UpdateLogger)

		// Create InstanceManager (needs Notifier)
		im := instance.NewInstanceManager(cfg, logger, concreteNotif)
		instanceManager = im

		// Create API server (conditional, can fail)
		if cfg.API.Port != 0 {
			apiSrv, apiErr := api.NewServer(&cfg.API, im, cfg, version, logger, concreteUpdateLogger, concreteNotif, selfUpdater, func() string {
					tokenMu.RLock()
					defer tokenMu.RUnlock()
					return currentBearerToken
				})
			if apiErr != nil {
				logger.Error("Failed to create API server", "error", apiErr)
				err = apiErr
				return
			}
			apiServer = apiSrv

			// Start API server in goroutine
			go func() {
				logger.Info("API server starting", "port", cfg.API.Port)
				if startErr := apiSrv.Start(); startErr != nil {
					logger.Error("API server error", "error", startErr)
				}
			}()
		}

		// Create health monitor (conditional)
		if len(cfg.Instances) > 0 {
			hm := health.NewHealthMonitor(
				func() []health.InstanceStatus {
					statuses := im.GetInstanceStatuses()
					result := make([]health.InstanceStatus, len(statuses))
					for i, s := range statuses {
						result[i] = health.InstanceStatus{
							Name:    s.Name,
							Port:    s.Port,
							Running: s.Running,
							PID:     s.PID,
						}
					}
					return result
				},
				cfg.HealthCheck.Interval,
				logger,
			)
			healthMonitor = hm
		}

		return
	}

	// startInstances auto-starts all configured instances and sends notification.
	startInstances := func(ctx context.Context, instanceManager any, notif any) {
		concreteIM := instanceManager.(*instance.InstanceManager)
		concreteNotif := notif.(*notifier.Notifier)

		result := concreteIM.StartAllInstances(ctx)
		if err := concreteNotif.NotifyStartupResult(result); err != nil {
			slog.Error("Failed to send startup notification", "error", err)
		}
	}

	// Start all application components (D-05, D-10)
	// Service mode: run via Windows SCM (D-09)
	if inService {
		slog.Info("Starting in service mode")
					// [HIGH-2] shared pointer: onReady sets it, hot reload callbacks read it
			var hotReloadComponents *lifecycle.AppComponents

			onReady := func(components *lifecycle.AppComponents) {
				hotReloadComponents = components

				callbacks := &config.HotReloadCallbacks{
					OnMonitorChange: func(newCfg *config.Config) {
						if hotReloadComponents.NetworkMonitor != nil {
							hotReloadComponents.NetworkMonitor.Stop()
						}
						if hotReloadComponents.NotificationManager != nil {
							hotReloadComponents.NotificationManager.Stop()
						}
						hotReloadComponents.NetworkMonitor = network.NewNetworkMonitor(
							"https://www.google.com",
							newCfg.Monitor.Interval,
							newCfg.Monitor.Timeout,
							logger,
						)
						go hotReloadComponents.NetworkMonitor.Start()
						hotReloadComponents.NotificationManager = notification.NewNotificationManager(
							hotReloadComponents.NetworkMonitor,
							notif,
							logger,
						)
						go hotReloadComponents.NotificationManager.Start(newCfg.Monitor.Interval)
						slog.Info("hot reload: monitor + notification manager rebuilt")
					},

					OnPushoverChange: func(newCfg *config.Config) {
						notif = notifier.NewWithConfig(
							notifier.Config{
								ApiToken: newCfg.Pushover.ApiToken,
								UserKey:  newCfg.Pushover.UserKey,
							},
							logger,
						)
						if hotReloadComponents.NotificationManager != nil {
							hotReloadComponents.NotificationManager.Stop()
							hotReloadComponents.NotificationManager = notification.NewNotificationManager(
								hotReloadComponents.NetworkMonitor,
								notif,
								logger,
							)
							go hotReloadComponents.NotificationManager.Start(newCfg.Monitor.Interval)
						}
						slog.Info("hot reload: notifier + notification manager rebuilt")
					},

					OnSelfUpdateChange: func(newCfg *config.Config) {
						slog.Warn("self_update config changed but requires service restart to apply",
							"new_owner", newCfg.SelfUpdate.GithubOwner,
							"new_repo", newCfg.SelfUpdate.GithubRepo,
						)
					},

					OnHealthCheckChange: func(newCfg *config.Config) {
						if hotReloadComponents.HealthMonitor != nil {
							hotReloadComponents.HealthMonitor.Stop()
						}
						im := hotReloadComponents.InstanceManager.(*instance.InstanceManager)
						hm := health.NewHealthMonitor(
							func() []health.InstanceStatus {
								statuses := im.GetInstanceStatuses()
								result := make([]health.InstanceStatus, len(statuses))
								for i, s := range statuses {
									result[i] = health.InstanceStatus{
										Name:    s.Name,
										Port:    s.Port,
										Running: s.Running,
										PID:     s.PID,
									}
								}
								return result
							},
							newCfg.HealthCheck.Interval,
							logger,
						)
						hotReloadComponents.HealthMonitor = hm
						go hm.Start()
						slog.Info("hot reload: health monitor rebuilt")
					},

					OnBearerTokenChange: func(newCfg *config.Config) {
						tokenMu.Lock()
						currentBearerToken = newCfg.API.BearerToken
						tokenMu.Unlock()
						slog.Info("hot reload: bearer token updated")
					},

					OnInstancesChange: func(newCfg *config.Config) {
						im := hotReloadComponents.InstanceManager.(*instance.InstanceManager)
						slog.Info("hot reload: instances config changed, performing full replace",
							"old_count", len(im.GetInstanceNames()),
							"new_count", len(newCfg.Instances),
						)
						stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
						lifecycle.StopAllNanobots(stopCtx, 5*time.Second, logger)
						stopCancel()
						newIM := instance.NewInstanceManager(newCfg, logger, notif)
						hotReloadComponents.InstanceManager = newIM
						startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Minute)
						newIM.StartAllInstances(startCtx)
						startCancel()
						slog.Info("hot reload: instances fully replaced and restarted",
							"instance_count", len(newCfg.Instances),
						)
					},
				}

				config.WatchConfig(cfg, logger, callbacks)
			}

			if err := lifecycle.RunService(cfg, logger, Version, updateLogger, notif, createComponents, startInstances, onReady); err != nil {
			logger.Error("Service execution failed", "error", err)
			os.Exit(1)
		}
		// svc.Run blocks until Execute returns. After RunService returns,
		// the service handler has completed its lifecycle -- exit cleanly.
		return
	}

	components, err := lifecycle.AppStartup(cfg, logger, Version, updateLogger, notif, createComponents, startInstances)
	if err != nil {
		logger.Error("Failed to start application", "error", err)
		// AppStartup already cleaned up partial components via rollback
		os.Exit(1)
	}

	// Console mode: wait for shutdown signal (D-06, D-11)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("Shutdown signal received")

	// Graceful shutdown with 10-second timeout (D-06)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lifecycle.AppShutdown(shutdownCtx, components, logger)
}
