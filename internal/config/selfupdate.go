package config

import "fmt"

// SelfUpdateConfig holds configuration for self-update functionality.
type SelfUpdateConfig struct {
	GithubOwner string `yaml:"github_owner" mapstructure:"github_owner"`
	GithubRepo  string `yaml:"github_repo" mapstructure:"github_repo"`
}

// Validate validates the SelfUpdateConfig values.
func (s *SelfUpdateConfig) Validate() error {
	if s.GithubOwner == "" {
		return fmt.Errorf("self_update.github_owner is required")
	}
	if s.GithubRepo == "" {
		return fmt.Errorf("self_update.github_repo is required")
	}
	return nil
}
