package config

import (
	"fmt"
	"time"
)

// NanobotConfig holds configuration for nanobot lifecycle management.
type NanobotConfig struct {
	Port           uint32        `yaml:"port" mapstructure:"port"`
	StartupTimeout time.Duration `yaml:"startup_timeout" mapstructure:"startup_timeout"`
}

// Config holds the main application configuration.
type Config struct {
	Nanobot NanobotConfig `yaml:"nanobot"`
}

// defaults sets the default values for the configuration.
func (c *Config) defaults() {
	c.Nanobot.Port = 18790
	c.Nanobot.StartupTimeout = 30 * time.Second
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
	return c.Nanobot.Validate()
}

// New creates a new Config with default values.
func New() *Config {
	c := &Config{}
	c.defaults()
	return c
}
