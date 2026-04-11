package config

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// PushoverConfig holds configuration for Pushover notifications.
type PushoverConfig struct {
	ApiToken string `yaml:"api_token" mapstructure:"api_token"`
	UserKey  string `yaml:"user_key" mapstructure:"user_key"`
}

// Config holds the main application configuration.
type Config struct {
	Instances  []InstanceConfig  `yaml:"instances" mapstructure:"instances"`
	Pushover   PushoverConfig    `yaml:"pushover" mapstructure:"pushover"`
	API        APIConfig         `yaml:"api" mapstructure:"api"`                    // HTTP API server config (CONF-02, CONF-03)
	Monitor    MonitorConfig     `yaml:"monitor" mapstructure:"monitor"`            // Monitoring service config (CONF-04, CONF-05)
	HealthCheck HealthCheckConfig `yaml:"health_check" mapstructure:"health_check"` // Instance health monitoring config (HEALTH-01)
	SelfUpdate SelfUpdateConfig  `yaml:"self_update" mapstructure:"self_update"`   // Self-update config (UPDATE-07)
	Service    ServiceConfig    `yaml:"service" mapstructure:"service"`           // Service mode config (MGR-01)
}

// defaults sets the default values for the configuration.
func (c *Config) defaults() {
	// Pushover defaults (optional)
	c.Pushover.ApiToken = ""
	c.Pushover.UserKey = ""

	// API defaults (CONF-01, CONF-02, CONF-03)
	c.API.Port = 8080
	c.API.BearerToken = "" // Required, no default (SEC-03)
	c.API.Timeout = 30 * time.Second

	// Monitor defaults (CONF-04, CONF-05)
	c.Monitor.Interval = 15 * time.Minute
	c.Monitor.Timeout = 10 * time.Second

	// HealthCheck defaults (HEALTH-01)
	c.HealthCheck.Interval = 1 * time.Minute

	// SelfUpdate defaults (UPDATE-07)
	c.SelfUpdate.GithubOwner = "allanpk716"
	c.SelfUpdate.GithubRepo = "nanobot_auto_update"

	// Service defaults (MGR-01, D-02, D-03)
	c.Service.AutoStart = nil // nil = false, unconfigured behaves same as current (D-02)
	c.Service.ServiceName = "NanobotAutoUpdater"
	c.Service.DisplayName = "Nanobot Auto Updater"
}

// validateUniqueNames checks for duplicate instance names.
func validateUniqueNames(instances []InstanceConfig) error {
	nameMap := make(map[string]int)
	for i, inst := range instances {
		if prevIndex, exists := nameMap[inst.Name]; exists {
			return fmt.Errorf("配置验证失败: 实例名称重复 - %q 出现在第 %d 和第 %d 个实例配置中",
				inst.Name, prevIndex+1, i+1)
		}
		nameMap[inst.Name] = i
	}
	return nil
}

// validateUniquePorts checks for duplicate instance ports.
func validateUniquePorts(instances []InstanceConfig) error {
	portMap := make(map[uint32]string)
	for _, inst := range instances {
		if prevName, exists := portMap[inst.Port]; exists {
			return fmt.Errorf("配置验证失败: 端口重复 - %d 出现在实例 %q 和 %q 中",
				inst.Port, prevName, inst.Name)
		}
		portMap[inst.Port] = inst.Name
	}
	return nil
}

// Validate validates the entire Config.
func (c *Config) Validate() error {
	var errs []error

	// Validate instances (required)
	if len(c.Instances) == 0 {
		errs = append(errs, fmt.Errorf("at least one instance must be configured in 'instances' array"))
	} else {
		// Validate unique names
		if err := validateUniqueNames(c.Instances); err != nil {
			errs = append(errs, err)
		}
		// Validate unique ports
		if err := validateUniquePorts(c.Instances); err != nil {
			errs = append(errs, err)
		}
		// Validate each instance
		for i := range c.Instances {
			if err := c.Instances[i].Validate(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Validate API config (CONF-06, SEC-03)
	if err := c.API.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate Monitor config (CONF-06)
	if err := c.Monitor.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate HealthCheck config (HEALTH-01)
	if err := c.HealthCheck.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate SelfUpdate config (UPDATE-07)
	if err := c.SelfUpdate.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate Service config (MGR-01)
	if err := c.Service.Validate(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// New creates a new Config with default values.
func New() *Config {
	c := &Config{}
	c.defaults()
	return c
}

var (
	// ErrHelpRequested is returned when help flag is requested.
	ErrHelpRequested = errors.New("help requested")
	// ErrVersionRequested is returned when version flag is requested.
	ErrVersionRequested = errors.New("version requested")
)

// viperInstance holds the viper instance used by Load().
// Exported so that hotreload.go can call WatchConfig/OnConfigChange on the same instance.
var viperInstance *viper.Viper

// GetViper returns the viper instance used for config loading.
// Returns nil if Load() has not been called yet.
func GetViper() *viper.Viper {
	return viperInstance
}

// ReloadConfig re-reads the config file and returns a new Config with validation.
// Returns the new Config on success, or the old config and error on failure.
func ReloadConfig(old *Config) (*Config, error) {
	if viperInstance == nil {
		return old, fmt.Errorf("viper not initialized")
	}
	if err := viperInstance.ReadInConfig(); err != nil {
		return old, fmt.Errorf("failed to re-read config file: %w", err)
	}
	newCfg := New()
	if err := viperInstance.Unmarshal(newCfg); err != nil {
		return old, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if err := newCfg.Validate(); err != nil {
		return old, fmt.Errorf("config validation failed: %w", err)
	}
	return newCfg, nil
}

// updateMu serializes the full read-modify-write cycle for config updates.
// This prevents concurrent API requests from causing data loss.
var updateMu sync.Mutex

// instanceConfigToMap converts an InstanceConfig to a map using mapstructure tag names as keys.
// This is needed because viper.Set() with struct values uses Go field names, which don't match
// the YAML keys that mapstructure uses for reading.
func instanceConfigToMap(ic InstanceConfig) map[string]interface{} {
	m := map[string]interface{}{
		"name":           ic.Name,
		"port":           ic.Port,
		"start_command":  ic.StartCommand,
		"startup_timeout": ic.StartupTimeout,
	}
	if ic.AutoStart != nil {
		m["auto_start"] = *ic.AutoStart
	}
	return m
}

// deepCopyConfig creates a deep copy of the Config struct.
// The Instances slice is recreated so mutations to it do not affect the original.
func deepCopyConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	c := *cfg
	c.Instances = make([]InstanceConfig, len(cfg.Instances))
	for i := range cfg.Instances {
		c.Instances[i] = cfg.Instances[i]
		// Deep copy AutoStart pointer
		if cfg.Instances[i].AutoStart != nil {
			val := *cfg.Instances[i].AutoStart
			c.Instances[i].AutoStart = &val
		}
	}
	return &c
}

// UpdateConfig atomically reads the current config, applies a mutation function, and persists the result.
// The mutation function receives a deep copy of the current config -- it is safe to modify freely.
// If the mutation function returns nil, the modified copy is written to disk via viper.
// If the mutation function returns an error, no write occurs and the original config is untouched.
// If the viper write fails, no write occurs and the original config is untouched.
//
// This function serializes all callers via updateMu, so concurrent API requests are safe.
//
// D-08: Uses viper.WriteConfig() to persist to the same file viper loaded from.
// D-09: After writing, hot-reload watcher detects the file change (500ms debounce).
func UpdateConfig(fn func(*Config) error) error {
	updateMu.Lock()
	defer updateMu.Unlock()

	// Read current config (this is a pointer to globalHotReload.current)
	current := GetCurrentConfig()
	if current == nil {
		return fmt.Errorf("config not initialized (WatchConfig not started)")
	}

	// Deep copy so mutations don't affect the live config
	copy := deepCopyConfig(current)

	// Apply the caller's mutation
	if err := fn(copy); err != nil {
		return fmt.Errorf("mutation failed: %w", err)
	}

	// Persist the modified copy
	v := GetViper()
	if v == nil {
		return fmt.Errorf("viper not initialized")
	}

	// Suppress hot-reload during write to prevent ReadInConfig() from
	// corrupting viper's internal state (which mixes v.Set() keys with
	// file-read keys). UpdateConfig updates globalHotReload.current
	// directly, making the file-watch reload unnecessary.
	if globalHotReload != nil {
		globalHotReload.skipReload = true
	}

	// Re-read the config file to ensure viper's internal state is fully
	// synchronized with the file before we overwrite it. Without this,
	// viper may lose keys (like api.bearer_token) that were not explicitly
	// set via v.Set() but were only present from the initial ReadInConfig().
	// This can happen because fsnotify events can trigger OnConfigChange
	// callbacks that call ReadInConfig() concurrently with WriteConfig(),
	// leading to stale internal state.
	if err := v.ReadInConfig(); err != nil {
		if globalHotReload != nil {
			globalHotReload.skipReload = false
		}
		return fmt.Errorf("failed to re-read config file: %w", err)
	}

	// Persist the modified copy.
	// Convert InstanceConfig structs to maps using mapstructure tag names as keys,
	// because v.Set("instances", structs) uses Go field names which don't match the YAML keys.
	instanceMaps := make([]interface{}, len(copy.Instances))
	for i, ic := range copy.Instances {
		instanceMaps[i] = instanceConfigToMap(ic)
	}
	v.Set("instances", instanceMaps)
	if err := v.WriteConfig(); err != nil {
		if globalHotReload != nil {
			globalHotReload.skipReload = false
		}
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Update the in-memory config so the next UpdateConfig call sees the latest state.
	// Without this, concurrent calls would all read the same stale globalHotReload.current
	// (hot-reload debounce is 500ms+, too slow for serialized API requests).
	globalHotReload.current = copy

	// Re-enable hot-reload after write completes.
	// The next file change (500ms+ from now) will trigger a normal reload.
	if globalHotReload != nil {
		globalHotReload.skipReload = false
	}

	return nil
}

// Load reads configuration from the specified YAML file.
// Returns Config with defaults applied, then file values, then validation.
func Load(configPath string) (*Config, error) {
	viperInstance = viper.New()
	viperInstance.SetConfigFile(configPath)
	viperInstance.SetConfigType("yaml")

	// Create config with defaults
	cfg := New()

	// Set defaults in viper (for fields not in file)
	// Pushover defaults (optional)
	viperInstance.SetDefault("pushover.api_token", cfg.Pushover.ApiToken)
	viperInstance.SetDefault("pushover.user_key", cfg.Pushover.UserKey)

	// Set defaults for API config (CONF-01, CONF-02, CONF-03)
	viperInstance.SetDefault("api.port", cfg.API.Port)
	viperInstance.SetDefault("api.timeout", cfg.API.Timeout)
	// Note: api.bearer_token has no default - it's required (SEC-03)

	// Set defaults for Monitor config (CONF-04, CONF-05)
	viperInstance.SetDefault("monitor.interval", cfg.Monitor.Interval)
	viperInstance.SetDefault("monitor.timeout", cfg.Monitor.Timeout)

	// Set defaults for HealthCheck config (HEALTH-01)
	viperInstance.SetDefault("health_check.interval", cfg.HealthCheck.Interval)

	// Set defaults for SelfUpdate config (UPDATE-07)
	viperInstance.SetDefault("self_update.github_owner", cfg.SelfUpdate.GithubOwner)
	viperInstance.SetDefault("self_update.github_repo", cfg.SelfUpdate.GithubRepo)

	// Set defaults for Service config (MGR-01)
	viperInstance.SetDefault("service.service_name", cfg.Service.ServiceName)
	viperInstance.SetDefault("service.display_name", cfg.Service.DisplayName)

	// Read config file (optional - use defaults if missing)
	if err := viperInstance.ReadInConfig(); err != nil {
		// If file doesn't exist, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// File not found is OK, use defaults
	}

	// Unmarshal to struct
	if err := viperInstance.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}
