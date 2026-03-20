package config

import (
	"fmt"
	"time"
)

// HealthCheckConfig holds configuration for instance health monitoring.
type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval" mapstructure:"interval"` // 健康检查间隔
}

// Validate validates the HealthCheckConfig values.
func (h *HealthCheckConfig) Validate() error {
	// Interval validation
	if h.Interval < 10*time.Second {
		return fmt.Errorf("health_check.interval 必须至少 10 秒，当前值: %v", h.Interval)
	}

	if h.Interval > 10*time.Minute {
		return fmt.Errorf("health_check.interval 不能超过 10 分钟，当前值: %v", h.Interval)
	}

	return nil
}
