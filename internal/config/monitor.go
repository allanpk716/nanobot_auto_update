package config

import (
	"fmt"
	"time"
)

// MonitorConfig holds configuration for monitoring service.
type MonitorConfig struct {
	Interval time.Duration `yaml:"interval" mapstructure:"interval"` // Google 连通性检查间隔
	Timeout  time.Duration `yaml:"timeout" mapstructure:"timeout"`   // HTTP 请求超时
}

// Validate validates the MonitorConfig values.
func (mc *MonitorConfig) Validate() error {
	// Interval validation
	if mc.Interval < 1*time.Minute {
		return fmt.Errorf("monitor.interval must be at least 1 minute, got %v", mc.Interval)
	}

	// Timeout validation
	if mc.Timeout < 1*time.Second {
		return fmt.Errorf("monitor.timeout must be at least 1 second, got %v", mc.Timeout)
	}

	return nil
}
