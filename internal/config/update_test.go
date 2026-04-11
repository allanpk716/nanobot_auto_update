package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateConfig_WritesInstancesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)
	require.Len(t, cfg.Instances, 1)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	WatchConfig(cfg, logger, &HotReloadCallbacks{})

	// Add a new instance via UpdateConfig
	err = UpdateConfig(func(c *Config) error {
		c.Instances = append(c.Instances, InstanceConfig{
			Name:           "new-instance",
			Port:           18791,
			StartCommand:   "nanobot gateway --config test",
			StartupTimeout: 30 * time.Second,
		})
		return nil
	})
	require.NoError(t, err)

	StopWatch()

	// Reset viperInstance so Load() reinitializes it from the file
	viperInstance = nil

	// Dump file content for debugging
	content, _ := os.ReadFile(configPath)
	t.Logf("File content after single write:\n%s", string(content))

	newCfg, err := Load(configPath)
	require.NoError(t, err)
	require.Len(t, newCfg.Instances, 2)
	assert.Equal(t, "new-instance", newCfg.Instances[1].Name)
	assert.Equal(t, uint32(18791), newCfg.Instances[1].Port)
}

func TestUpdateConfig_ReturnsErrorWhenConfigNotInitialized(t *testing.T) {
	// Ensure globalHotReload is nil
	StopWatch()

	err := UpdateConfig(func(cfg *Config) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not initialized")
}

func TestUpdateConfig_ReturnsErrorWhenViperNil(t *testing.T) {
	// Ensure viperInstance is nil (Load not called or reset)
	viperInstance = nil
	StopWatch()

	err := UpdateConfig(func(cfg *Config) error {
		return nil
	})
	require.Error(t, err)
	// When globalHotReload is nil, we get "config not initialized" first
	assert.Contains(t, err.Error(), "config not initialized")
}

func TestUpdateConfig_MutationErrorDoesNotWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	WatchConfig(cfg, logger, &HotReloadCallbacks{})
	t.Cleanup(func() { StopWatch() })

	// Mutation that returns an error
	err = UpdateConfig(func(c *Config) error {
		return fmt.Errorf("mutation rejected")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutation rejected")

	// Verify original config file is unchanged
	StopWatch()
	viperInstance = nil
	newCfg, err := Load(configPath)
	require.NoError(t, err)
	require.Len(t, newCfg.Instances, 1, "Original config should be unchanged after mutation error")
	assert.Equal(t, "existing", newCfg.Instances[0].Name)
}

func TestUpdateConfig_DeepCopyPreventsSharedStateCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	WatchConfig(cfg, logger, &HotReloadCallbacks{})
	t.Cleanup(func() { StopWatch() })

	// Mutation modifies the config but then returns an error
	err = UpdateConfig(func(c *Config) error {
		c.Instances[0].Name = "modified"
		return fmt.Errorf("fail")
	})
	require.Error(t, err)

	// Verify GetCurrentConfig still has the original name
	currentCfg := GetCurrentConfig()
	require.NotNil(t, currentCfg)
	assert.Equal(t, "existing", currentCfg.Instances[0].Name,
		"Deep copy should prevent mutation failure from corrupting live config")
}

func TestUpdateConfig_ConcurrentMutationsNoDataLoss(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	WatchConfig(cfg, logger, &HotReloadCallbacks{})

	var wg sync.WaitGroup
	const numGoroutines = 10
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := UpdateConfig(func(c *Config) error {
				c.Instances = append(c.Instances, InstanceConfig{
					Name:           fmt.Sprintf("concurrent-%d", idx),
					Port:           uint32(18800 + idx),
					StartCommand:   "nanobot test",
					StartupTimeout: 30 * time.Second,
				})
				return nil
			})
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify no errors
	for i, err := range errors {
		assert.NoError(t, err, "goroutine %d should not error", i)
	}

	// Stop watch and reload to check persisted state
	StopWatch()
	viperInstance = nil

	// Dump file content for debugging
	content, _ := os.ReadFile(configPath)
	t.Logf("File content after %d concurrent writes:\n%s", numGoroutines, string(content))

	newCfg, err := Load(configPath)
	require.NoError(t, err)

	// Should have 1 original + 10 new = 11 instances
	assert.Len(t, newCfg.Instances, 11, "all concurrent mutations should be preserved")
}

func TestUpdateConfig_ConcurrentPreservesOtherFields(t *testing.T) {
	// Verifies that after multiple UpdateConfig calls, the file still has
	// all non-instance fields intact (bearer_token, api.port, etc.)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	WatchConfig(cfg, logger, &HotReloadCallbacks{})

	err = UpdateConfig(func(c *Config) error {
		c.Instances = append(c.Instances, InstanceConfig{
			Name:           "new",
			Port:           18791,
			StartCommand:   "nanobot worker",
			StartupTimeout: 30 * time.Second,
		})
		return nil
	})
	require.NoError(t, err)

	StopWatch()
	viperInstance = nil
	newCfg, err := Load(configPath)
	require.NoError(t, err)

	// Verify non-instance fields survived
	assert.Equal(t, uint32(9999), newCfg.API.Port)
	assert.Equal(t, "test-token-123456789012345678901", newCfg.API.BearerToken)
}
