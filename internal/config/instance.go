package config

import "time"

// InstanceConfig holds configuration for a single nanobot instance.
type InstanceConfig struct {
	Name           string        `mapstructure:"name"`
	Port           uint32        `mapstructure:"port"`
	StartCommand   string        `mapstructure:"start_command"`
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`
}

// Validate validates the InstanceConfig values.
func (ic *InstanceConfig) Validate() error {
	// TODO: implement validation
	return nil
}
