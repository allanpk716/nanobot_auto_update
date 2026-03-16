package config

import (
	"errors"
	"fmt"
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
	Instances []InstanceConfig `yaml:"instances" mapstructure:"instances"`
	Pushover  PushoverConfig   `yaml:"pushover" mapstructure:"pushover"`
	API       APIConfig        `yaml:"api" mapstructure:"api"`         // HTTP API server config (CONF-02, CONF-03)
	Monitor   MonitorConfig    `yaml:"monitor" mapstructure:"monitor"` // Monitoring service config (CONF-04, CONF-05)
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

// Load reads configuration from the specified YAML file.
// Returns Config with defaults applied, then file values, then validation.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Create config with defaults
	cfg := New()

	// Set defaults in viper (for fields not in file)
	// Pushover defaults (optional)
	v.SetDefault("pushover.api_token", cfg.Pushover.ApiToken)
	v.SetDefault("pushover.user_key", cfg.Pushover.UserKey)

	// Set defaults for API config (CONF-01, CONF-02, CONF-03)
	v.SetDefault("api.port", cfg.API.Port)
	v.SetDefault("api.timeout", cfg.API.Timeout)
	// Note: api.bearer_token has no default - it's required (SEC-03)

	// Set defaults for Monitor config (CONF-04, CONF-05)
	v.SetDefault("monitor.interval", cfg.Monitor.Interval)
	v.SetDefault("monitor.timeout", cfg.Monitor.Timeout)

	// Read config file (optional - use defaults if missing)
	if err := v.ReadInConfig(); err != nil {
		// If file doesn't exist, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// File not found is OK, use defaults
	}

	// Unmarshal to struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}
