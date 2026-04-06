package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfUpdateConfig_Defaults(t *testing.T) {
	cfg := New()
	assert.Equal(t, "allanpk716", cfg.SelfUpdate.GithubOwner)
	assert.Equal(t, "nanobot_auto_update", cfg.SelfUpdate.GithubRepo)
}

func TestSelfUpdateConfig_ViperLoad(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	yamlContent := []byte(`
self_update:
  github_owner: "MyOrg"
  github_repo: "my-repo"
api:
  bearer_token: "a-really-long-bearer-token-that-is-at-least-32-chars"
instances:
  - name: test
    port: 8081
    install_path: "C:\\test"
    start_command: "echo test"
`)
	require.NoError(t, os.WriteFile(configPath, yamlContent, 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "MyOrg", cfg.SelfUpdate.GithubOwner)
	assert.Equal(t, "my-repo", cfg.SelfUpdate.GithubRepo)
}

func TestSelfUpdateConfig_EmptyValues(t *testing.T) {
	s := SelfUpdateConfig{}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github_owner is required")
}

func TestSelfUpdateConfig_ValidValues(t *testing.T) {
	s := SelfUpdateConfig{
		GithubOwner: "test",
		GithubRepo:  "repo",
	}
	err := s.Validate()
	assert.NoError(t, err)
}
