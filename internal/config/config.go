package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// NanobotConfig holds configuration for nanobot lifecycle management.
type NanobotConfig struct {
	Port           uint32        `yaml:"port" mapstructure:"port"`
	StartupTimeout time.Duration `yaml:"startup_timeout" mapstructure:"startup_timeout"`
	RepoPath       string        `yaml:"repo_path" mapstructure:"repo_path"`
}

// PushoverConfig holds configuration for Pushover notifications.
type PushoverConfig struct {
	ApiToken string `yaml:"api_token" mapstructure:"api_token"`
	UserKey  string `yaml:"user_key" mapstructure:"user_key"`
}

// Config holds the main application configuration.
type Config struct {
	Cron      string           `yaml:"cron" mapstructure:"cron"`
	Nanobot   NanobotConfig    `yaml:"nanobot" mapstructure:"nanobot"`
	Instances []InstanceConfig `yaml:"instances" mapstructure:"instances"`
	Pushover  PushoverConfig   `yaml:"pushover" mapstructure:"pushover"`
	API       APIConfig        `yaml:"api" mapstructure:"api"`         // HTTP API server config (CONF-02, CONF-03)
	Monitor   MonitorConfig    `yaml:"monitor" mapstructure:"monitor"` // Monitoring service config (CONF-04, CONF-05)
}

// defaults sets the default values for the configuration.
func (c *Config) defaults() {
	c.Cron = "0 3 * * *"
	// Note: Nanobot defaults are set in Validate() only when using legacy mode
	c.Nanobot.RepoPath = ""
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

// Validate validates the NanobotConfig values.
func (nc *NanobotConfig) Validate() error {
	if nc.Port == 0 || nc.Port > 65535 {
		return fmt.Errorf("port must be > 0 and <= 65535, got %d", nc.Port)
	}
	if nc.StartupTimeout < 5*time.Second {
		return fmt.Errorf("startup_timeout must be at least 5 seconds, got %v", nc.StartupTimeout)
	}
	return nil
}

// ValidateModeCompatibility checks if both legacy and new modes are configured.
func (c *Config) ValidateModeCompatibility() error {
	hasLegacyMode := c.Nanobot.Port != 0
	hasNewMode := len(c.Instances) > 0

	if hasLegacyMode && hasNewMode {
		return fmt.Errorf("配置错误: 不能同时使用 'nanobot' section 和 'instances' 数组,请选择其中一种配置模式")
	}
	return nil
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

	// Validate cron expression
	if err := ValidateCron(c.Cron); err != nil {
		errs = append(errs, err)
	}

	// Check mode compatibility
	if err := c.ValidateModeCompatibility(); err != nil {
		errs = append(errs, err)
	}

	// Validate based on mode
	if len(c.Instances) > 0 {
		// Multi-instance mode
		if err := validateUniqueNames(c.Instances); err != nil {
			errs = append(errs, err)
		}
		if err := validateUniquePorts(c.Instances); err != nil {
			errs = append(errs, err)
		}
		for i := range c.Instances {
			if err := c.Instances[i].Validate(); err != nil {
				errs = append(errs, err)
			}
		}
	} else {
		// Legacy mode - set defaults for Nanobot if needed
		if c.Nanobot.Port == 0 {
			c.Nanobot.Port = 18790
		}
		if c.Nanobot.StartupTimeout == 0 {
			c.Nanobot.StartupTimeout = 30 * time.Second
		}
		if err := c.Nanobot.Validate(); err != nil {
			errs = append(errs, err)
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

// ValidateCron validates a cron expression.
func ValidateCron(expr string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
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
	v.SetDefault("cron", cfg.Cron)
	// Note: Nanobot defaults are NOT set here to allow mode detection in Validate()
	v.SetDefault("nanobot.repo_path", cfg.Nanobot.RepoPath)
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
