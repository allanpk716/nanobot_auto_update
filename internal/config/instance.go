package config

import (
	"fmt"
	"time"
)

// InstanceConfig holds configuration for a single nanobot instance.
type InstanceConfig struct {
	Name           string        `mapstructure:"name"`
	Port           uint32        `mapstructure:"port"`
	StartCommand   string        `mapstructure:"start_command"`
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`
}

// Validate validates the InstanceConfig values.
func (ic *InstanceConfig) Validate() error {
	// Validate name
	if ic.Name == "" {
		return fmt.Errorf("实例 %q 缺少必填字段 \"name\"", ic.Name)
	}

	// Validate port
	if ic.Port == 0 || ic.Port > 65535 {
		return fmt.Errorf("实例 %q 端口必须在 1-65535 范围内,当前值: %d", ic.Name, ic.Port)
	}

	// Validate start_command
	if ic.StartCommand == "" {
		return fmt.Errorf("实例 %q 缺少必填字段 \"start_command\"", ic.Name)
	}

	// Validate startup_timeout (only if non-zero)
	if ic.StartupTimeout != 0 && ic.StartupTimeout < 5*time.Second {
		return fmt.Errorf("实例 %q startup_timeout 必须至少 5 秒,当前值: %v", ic.Name, ic.StartupTimeout)
	}

	return nil
}
