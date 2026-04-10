package lifecycle

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/network"
	"github.com/HQGroup/nanobot-auto-updater/internal/notification"
	"github.com/HQGroup/nanobot-auto-updater/internal/selfupdate"
	"github.com/robfig/cron/v3"
)

// Shutdownable is the interface for components that support graceful shutdown with a context.
type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

// Stoppable is the interface for components that can be stopped.
type Stoppable interface {
	Stop()
}

// Startable is the interface for components that can be started.
type Startable interface {
	Start()
}

// Closable is the interface for components that can be closed.
type Closable interface {
	Close() error
}

// Cleanupable is the interface for components with cleanup logic.
type Cleanupable interface {
	CleanupOldLogs() error
	LoadFromFile() error
}

// LogScheduler is the interface for the update logger.
// Combines cleanup, file loading, and close operations.
type LogScheduler interface {
	Cleanupable
	Closable
}

// APIServerControl is the interface for API server lifecycle.
// *api.Server satisfies this interface via duck typing.
type APIServerControl interface {
	Start() error
	Shutdownable
}

// HealthMonitorControl is the interface for health monitor lifecycle.
// *health.HealthMonitor satisfies this interface via duck typing.
type HealthMonitorControl interface {
	Startable
	Stoppable
}

// NotifySender is the interface for notification functionality.
// *notifier.Notifier satisfies this interface via duck typing.
// Combines the notification.Notifier interface methods needed by NotificationManager.
// Note: NotifyStartupResult is handled via the StartInstancesFunc callback,
// not through this interface, because its parameter type (*instance.AutoStartResult)
// is in a package that would create a circular import.
type NotifySender interface {
	IsEnabled() bool
	Notify(title, message string) error
}

// AppComponents holds all initialized application components.
// Fields are ordered by shutdown sequence (D-07, D-08).
//
// Fields use interfaces to avoid circular import dependencies:
// lifecycle cannot import api, instance, health, notifier, or updatelog
// because they all directly or indirectly import instance -> lifecycle.
type AppComponents struct {
	// Components with Stop/Close methods (shutdown order matters)
	NotificationManager *notification.NotificationManager
	NetworkMonitor      *network.NetworkMonitor
	HealthMonitor       HealthMonitorControl
	CleanupCron         *cron.Cron
	UpdateLogger        LogScheduler
	APIServer           APIServerControl

	// Internal dependencies (not shut down directly, but needed by other components)
	Notifier        NotifySender
	InstanceManager any // *instance.InstanceManager -- uses any to avoid circular import
	SelfUpdater     *selfupdate.Updater

	// AutoStartDone is closed when the auto-start goroutine completes.
	// Used for testing and graceful shutdown awareness.
	AutoStartDone chan struct{}
}

// AppShutdown performs ordered shutdown of all non-nil components (D-05, D-07).
// Components are shut down in the same order as the original main.go:
// notificationManager -> networkMonitor -> healthMonitor -> cleanupCron -> updateLogger -> apiServer.
// Internal dependencies (Notifier, InstanceManager, SelfUpdater) do not have Stop/Close methods.
func AppShutdown(ctx context.Context, c *AppComponents, logger *slog.Logger) {
	if c == nil {
		return
	}
	if c.NotificationManager != nil {
		c.NotificationManager.Stop()
	}
	if c.NetworkMonitor != nil {
		c.NetworkMonitor.Stop()
	}
	if c.HealthMonitor != nil {
		c.HealthMonitor.Stop()
	}
	if c.CleanupCron != nil {
		c.CleanupCron.Stop()
		logger.Info("Update log cleanup scheduler stopped")
	}
	if c.UpdateLogger != nil {
		if err := c.UpdateLogger.Close(); err != nil {
			logger.Error("Failed to close update logger", "error", err)
		}
	}
	if c.APIServer != nil {
		if err := c.APIServer.Shutdown(ctx); err != nil {
			logger.Error("API server shutdown error", "error", err)
		}
	}
	logger.Info("Shutdown completed")
}

// CreateComponentsFunc is the signature for creating circular-dependency components.
// Called by AppStartup to create the API server, health monitor, and instance manager.
// The caller (main.go) provides this function because lifecycle cannot import
// api, instance, health, notifier, or updatelog packages due to circular imports.
//
// Returns:
//   - instanceManager: *instance.InstanceManager (as any)
//   - healthMonitor: *health.HealthMonitor (as HealthMonitorControl)
//   - apiServer: *api.Server (as APIServerControl)
//   - error: non-nil if any component creation fails
type CreateComponentsFunc func(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	notif any,
	updateLogger any,
	selfUpdater *selfupdate.Updater,
) (instanceManager any, healthMonitor HealthMonitorControl, apiServer APIServerControl, err error)

// StartInstancesFunc is the signature for the auto-start callback.
// The caller (main.go) provides this function which captures concrete types
// from packages that have circular import dependencies with lifecycle.
type StartInstancesFunc func(ctx context.Context, instanceManager any, notif any)

// AppStartup initializes all application components and returns a container (D-10).
// On error, any partially-initialized components are cleaned up via AppShutdown (rollback).
// The caller should NOT call AppShutdown again on error -- rollback is already done.
// On success, the caller is responsible for calling AppShutdown when done.
//
// Parameters:
//   - cfg: Application configuration
//   - logger: Initialized structured logger
//   - version: Application version string
//   - updateLogger: *updatelog.UpdateLogger (as LogScheduler to avoid circular import)
//   - notif: *notifier.Notifier (as StartupNotifier to avoid circular import)
//   - createComponents: Factory for circular-dependency components. May be nil.
//   - startInstances: Callback for auto-starting instances. May be nil.
func AppStartup(
	cfg *config.Config,
	logger *slog.Logger,
	version string,
	updateLogger LogScheduler,
	notif NotifySender,
	createComponents CreateComponentsFunc,
	startInstances StartInstancesFunc,
) (*AppComponents, error) {
	c := &AppComponents{
		AutoStartDone: make(chan struct{}),
		UpdateLogger:  updateLogger,
		Notifier:      notif,
	}

	// Helper: on error, rollback partial components and return
	rollback := func(err error) (*AppComponents, error) {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		AppShutdown(cleanupCtx, c, logger)
		cancel()
		return nil, err
	}

	// Step 1: Startup cleanup + load history
	if c.UpdateLogger != nil {
		if err := c.UpdateLogger.CleanupOldLogs(); err != nil {
			logger.Error("Failed to cleanup old update logs", "error", err)
		}
		if err := c.UpdateLogger.LoadFromFile(); err != nil {
			logger.Error("Failed to load update logs from file", "error", err)
		}
	}

	// Step 2: CleanupCron (cron.New() never fails, AddFunc/Start never fail)
	c.CleanupCron = cron.New()
	c.CleanupCron.AddFunc("0 3 * * *", func() {
		if c.UpdateLogger != nil {
			if err := c.UpdateLogger.CleanupOldLogs(); err != nil {
				logger.Error("Scheduled update log cleanup failed", "error", err)
			}
		}
	})
	c.CleanupCron.Start()
	logger.Info("Update log cleanup scheduler started", "schedule", "0 3 * * *")

	// Step 3: SelfUpdater
	c.SelfUpdater = selfupdate.NewUpdater(
		selfupdate.SelfUpdateConfig{
			GithubOwner: cfg.SelfUpdate.GithubOwner,
			GithubRepo:  cfg.SelfUpdate.GithubRepo,
		},
		logger,
	)

	// Step 4: Create circular-dependency components via factory function
	if createComponents != nil {
		instanceManager, healthMonitor, apiServer, err := createComponents(
			cfg, logger, version, notif, updateLogger, c.SelfUpdater,
		)
		if err != nil {
			logger.Error("Failed to create components", "error", err)
			return rollback(err)
		}
		c.InstanceManager = instanceManager
		c.HealthMonitor = healthMonitor
		c.APIServer = apiServer

		// Start health monitor (if created)
		if c.HealthMonitor != nil {
			go c.HealthMonitor.Start()
			logger.Info("Health monitor started", "interval", cfg.HealthCheck.Interval)
		}
	}

	// Step 5: NetworkMonitor (always created, no failure path)
	c.NetworkMonitor = network.NewNetworkMonitor(
		"https://www.google.com",
		cfg.Monitor.Interval,
		cfg.Monitor.Timeout,
		logger,
	)
	go c.NetworkMonitor.Start()
	logger.Info("Network monitor started", "interval", cfg.Monitor.Interval)

	// Step 6: NotificationManager (always created, no failure path)
	c.NotificationManager = notification.NewNotificationManager(
		c.NetworkMonitor,
		notif,
		logger,
	)
	go c.NotificationManager.Start(cfg.Monitor.Interval)
	logger.Info("Notification manager started", "check_interval", cfg.Monitor.Interval)

	// Step 7: Auto-start goroutine
	// AppStartup owns launching this goroutine. The AutoStartDone channel is
	// closed when the goroutine finishes, allowing callers to wait if needed.
	if startInstances != nil {
		go func() {
			defer close(c.AutoStartDone)
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Auto-start goroutine panic",
						"panic", r,
						"stack", string(debug.Stack()))
				}
			}()

			autoStartTimeout := 5 * time.Minute
			autoStartCtx, cancel := context.WithTimeout(context.Background(), autoStartTimeout)
			defer cancel()

			logger.Info("Starting auto-start for all instances",
				"instance_count", len(cfg.Instances),
				"timeout", autoStartTimeout)

			startInstances(autoStartCtx, c.InstanceManager, notif)
		}()
	}

	return c, nil
}
