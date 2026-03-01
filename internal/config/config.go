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
}

// PushoverConfig holds configuration for Pushover notifications.
type PushoverConfig struct {
	ApiToken string `yaml:"api_token" mapstructure:"api_token"`
	UserKey  string `yaml:"user_key" mapstructure:"user_key"`
}

// Config holds the main application configuration.
type Config struct {
	Cron     string         `yaml:"cron" mapstructure:"cron"`
	Nanobot  NanobotConfig  `yaml:"nanobot" mapstructure:"nanobot"`
	Pushover PushoverConfig `yaml:"pushover" mapstructure:"pushover"`
}

// defaults sets the default values for the configuration.
func (c *Config) defaults() {
	c.Cron = "0 3 * * *"
	c.Nanobot.Port = 18790
	c.Nanobot.StartupTimeout = 30 * time.Second
	c.Pushover.ApiToken = ""
	c.Pushover.UserKey = ""
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

// Validate validates the entire Config.
func (c *Config) Validate() error {
	if err := ValidateCron(c.Cron); err != nil {
		return err
	}
	return c.Nanobot.Validate()
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
	v.SetDefault("nanobot.port", cfg.Nanobot.Port)
	v.SetDefault("nanobot.startup_timeout", cfg.Nanobot.StartupTimeout.String())
	v.SetDefault("pushover.api_token", cfg.Pushover.ApiToken)
	v.SetDefault("pushover.user_key", cfg.Pushover.UserKey)

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
