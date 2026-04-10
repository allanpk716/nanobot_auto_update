package config

import (
	"fmt"
	"regexp"
)

// serviceNameRegex enforces alphanumeric-only service names (D-10).
var serviceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// ServiceConfig holds configuration for Windows Service mode (MGR-01).
type ServiceConfig struct {
	AutoStart   *bool  `yaml:"auto_start" mapstructure:"auto_start"`     // nil = false, unconfigured = current behavior (D-02)
	ServiceName string `yaml:"service_name" mapstructure:"service_name"`  // Windows service name (D-10: alphanumeric only)
	DisplayName string `yaml:"display_name" mapstructure:"display_name"`  // Windows service display name
}

// Validate validates the ServiceConfig values.
// Returns a single error for the first validation failure (matching SelfUpdateConfig.Validate() pattern).
// errors.Join aggregation is done only in Config.Validate() at the root level.
func (s *ServiceConfig) Validate() error {
	// Skip validation when auto_start is not explicitly true (D-12)
	if s.AutoStart == nil || !*s.AutoStart {
		return nil
	}

	// Validate ServiceName: must be non-empty and alphanumeric only (D-10)
	if s.ServiceName == "" || !serviceNameRegex.MatchString(s.ServiceName) {
		return fmt.Errorf("service.service_name must contain only alphanumeric characters, got %q", s.ServiceName)
	}

	// Validate ServiceName max length (defense-in-depth: SCM has 256-char limit)
	if len(s.ServiceName) > 256 {
		return fmt.Errorf("service.service_name must be at most 256 characters, got %d", len(s.ServiceName))
	}

	// Validate DisplayName: required when auto_start is true
	if len(s.DisplayName) == 0 {
		return fmt.Errorf("service.display_name is required when auto_start is true")
	}

	// Validate DisplayName max length (D-11)
	if len(s.DisplayName) > 256 {
		return fmt.Errorf("service.display_name must be at most 256 characters, got %d", len(s.DisplayName))
	}

	return nil
}
