package config

import (
	"fmt"
	"time"
)

// APIConfig holds configuration for HTTP API server.
type APIConfig struct {
	Port        uint32        `yaml:"port" mapstructure:"port"`
	BearerToken string        `yaml:"bearer_token" mapstructure:"bearer_token"`
	Timeout     time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

// Validate validates the APIConfig values.
func (ac *APIConfig) Validate() error {
	// Port validation
	if ac.Port == 0 || ac.Port > 65535 {
		return fmt.Errorf("api.port must be between 1 and 65535, got %d", ac.Port)
	}

	// Bearer Token validation (SEC-03)
	if len(ac.BearerToken) < 32 {
		return fmt.Errorf("api.bearer_token must be at least 32 characters for security, got %d", len(ac.BearerToken))
	}

	// Timeout validation
	if ac.Timeout < 5*time.Second {
		return fmt.Errorf("api.timeout must be at least 5 seconds, got %v", ac.Timeout)
	}

	return nil
}
