package config

import (
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// HotReloadCallbacks holds functions to rebuild specific components when their config changes.
// Each function receives the new config and returns nothing.
// Functions are optional (nil-safe) -- if a callback is nil, that config section is skipped.
type HotReloadCallbacks struct {
	// OnMonitorChange rebuilds NetworkMonitor + NotificationManager when monitor config changes.
	// Old components are stopped before calling; caller starts new ones after return.
	OnMonitorChange func(newCfg *Config)

	// OnPushoverChange rebuilds Notifier when pushover config changes.
	OnPushoverChange func(newCfg *Config)

	// OnSelfUpdateChange logs that self_update config changed.
	// Does NOT rebuild SelfUpdater to avoid stale reference in SelfUpdateHandler.
	// Self-update config changes (github_owner/repo) require a service restart.
	OnSelfUpdateChange func(newCfg *Config)

	// OnHealthCheckChange rebuilds HealthMonitor when health_check config changes.
	OnHealthCheckChange func(newCfg *Config)

	// OnBearerTokenChange updates the dynamic token getter when api.bearer_token changes.
	OnBearerTokenChange func(newCfg *Config)

	// OnInstancesChange handles instance config changes (full replace: stop all -> recreate -> start all).
	OnInstancesChange func(newCfg *Config)
}

// hotReloadState manages the config file watcher lifecycle.
// [HIGH-1 fix] All component rebuilds are serialized via mu mutex.
// Only one rebuild can be in progress at any time.
type hotReloadState struct {
	mu            sync.Mutex
	viper         *viper.Viper
	logger        *slog.Logger
	current       *Config
	callbacks     *HotReloadCallbacks
	running       bool
	debounceTimer *time.Timer // [MED-1 fix] debounce rapid fsnotify events
}

var globalHotReload *hotReloadState

// WatchConfig starts watching the config file for changes (D-04).
// Only call this in service mode. Console mode does not use hot reload.
// The callbacks define how each config section is rebuilt when changed.
func WatchConfig(currentCfg *Config, logger *slog.Logger, callbacks *HotReloadCallbacks) {
	if globalHotReload != nil {
		logger.Warn("config hot reload already active")
		return
	}

	v := GetViper()
	if v == nil {
		logger.Error("cannot start config watch: viper not initialized (Load() not called)")
		return
	}

	state := &hotReloadState{
		viper:     v,
		logger:    logger.With("source", "config-hotreload"),
		current:   currentCfg,
		callbacks: callbacks,
		running:   true,
	}
	globalHotReload = state

	// [MED-1 fix] OnConfigChange only resets debounce timer.
	// Actual reload happens 500ms after the last event to coalesce rapid changes
	// (e.g., Windows editors that create temp file + rename on save).
	v.OnConfigChange(func(e fsnotify.Event) {
		state.mu.Lock()
		defer state.mu.Unlock()

		if !state.running {
			return
		}

		state.logger.Info("config file change detected",
			"file", e.Name,
			"operation", e.Op.String(),
		)

		// Reset debounce timer: coalesce events within 500ms window
		if state.debounceTimer != nil {
			state.debounceTimer.Stop()
		}
		state.debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
			state.doReload()
		})
	})

	v.WatchConfig()
	logger.Info("config file watcher started (service mode, 500ms debounce)")
}

// doReload performs the actual config reload after debounce period.
// [HIGH-1 fix] Runs under state.mu to serialize all rebuilds.
func (s *hotReloadState) doReload() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("executing debounced config reload")

	newCfg, err := ReloadConfig(s.current)
	if err != nil {
		s.logger.Error("config reload failed, keeping current config", "error", err)
		return
	}

	s.handleConfigChange(s.current, newCfg)
	s.current = newCfg
}

// handleConfigChange compares old and new config, invoking only the callbacks
// for sections that actually changed (D-07).
func (s *hotReloadState) handleConfigChange(oldCfg, newCfg *Config) {
	// D-06: api.port and service config are NOT hot-reloaded
	// Only reload sections listed in D-05

	// Monitor (interval/timeout) -> rebuild NetworkMonitor + NotificationManager
	if !reflect.DeepEqual(oldCfg.Monitor, newCfg.Monitor) {
		s.logger.Info("monitor config changed, triggering rebuild",
			"old_interval", oldCfg.Monitor.Interval,
			"new_interval", newCfg.Monitor.Interval,
		)
		if s.callbacks.OnMonitorChange != nil {
			s.callbacks.OnMonitorChange(newCfg)
		}
	}

	// Pushover (api_token/user_key) -> rebuild Notifier
	if !reflect.DeepEqual(oldCfg.Pushover, newCfg.Pushover) {
		s.logger.Info("pushover config changed, triggering rebuild",
			"token_changed", oldCfg.Pushover.ApiToken != newCfg.Pushover.ApiToken,
		)
		if s.callbacks.OnPushoverChange != nil {
			s.callbacks.OnPushoverChange(newCfg)
		}
	}

	// SelfUpdate -- [MED-2 fix] log only, do not rebuild to avoid stale reference
	if !reflect.DeepEqual(oldCfg.SelfUpdate, newCfg.SelfUpdate) {
		s.logger.Info("self_update config changed -- restart service to apply",
			"old_owner", oldCfg.SelfUpdate.GithubOwner,
			"new_owner", newCfg.SelfUpdate.GithubOwner,
		)
		if s.callbacks.OnSelfUpdateChange != nil {
			s.callbacks.OnSelfUpdateChange(newCfg)
		}
	}

	// HealthCheck (interval) -> rebuild HealthMonitor
	if !reflect.DeepEqual(oldCfg.HealthCheck, newCfg.HealthCheck) {
		s.logger.Info("health_check config changed, triggering rebuild",
			"old_interval", oldCfg.HealthCheck.Interval,
			"new_interval", newCfg.HealthCheck.Interval,
		)
		if s.callbacks.OnHealthCheckChange != nil {
			s.callbacks.OnHealthCheckChange(newCfg)
		}
	}

	// API bearer_token -> update via dynamic getter
	if oldCfg.API.BearerToken != newCfg.API.BearerToken {
		s.logger.Info("api.bearer_token changed, triggering update")
		if s.callbacks.OnBearerTokenChange != nil {
			s.callbacks.OnBearerTokenChange(newCfg)
		}
	}

	// Instances (add/remove/modify) -> [HIGH-2 fix] full replace
	if !reflect.DeepEqual(oldCfg.Instances, newCfg.Instances) {
		s.logger.Info("instances config changed, triggering full replace",
			"old_count", len(oldCfg.Instances),
			"new_count", len(newCfg.Instances),
		)
		if s.callbacks.OnInstancesChange != nil {
			s.callbacks.OnInstancesChange(newCfg)
		}
	}
}

// StopWatch stops the config file watcher.
func StopWatch() {
	if globalHotReload == nil {
		return
	}
	globalHotReload.mu.Lock()
	defer globalHotReload.mu.Unlock()

	globalHotReload.running = false
	if globalHotReload.debounceTimer != nil {
		globalHotReload.debounceTimer.Stop()
	}
	globalHotReload = nil
}

// GetCurrentConfig returns the most recently loaded config (may be hot-reloaded).
// Returns nil if WatchConfig has not been started.
func GetCurrentConfig() *Config {
	if globalHotReload == nil {
		return nil
	}
	globalHotReload.mu.Lock()
	defer globalHotReload.mu.Unlock()
	return globalHotReload.current
}
