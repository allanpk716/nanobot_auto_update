package nanobot

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseConfigPath tests ---

func TestParseConfigPath_WithConfigFlag(t *testing.T) {
	path, err := ParseConfigPath("nanobot gateway --config C:/Users/test/.nanobot-helper/config.json --port 18792", "test")
	require.NoError(t, err)
	// filepath.Abs normalizes slashes on Windows
	assert.Equal(t, filepath.FromSlash("C:/Users/test/.nanobot-helper/config.json"), path)
}

func TestParseConfigPath_WithTildePath(t *testing.T) {
	path, err := ParseConfigPath("nanobot gateway --config ~/.nanobot-test/config.json", "test")
	require.NoError(t, err)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.Join(homeDir, ".nanobot-test", "config.json")
	assert.Equal(t, expected, path)
}

func TestParseConfigPath_WithoutConfigFlag(t *testing.T) {
	path, err := ParseConfigPath("nanobot gateway", "my-instance")
	require.NoError(t, err)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.Join(homeDir, ".nanobot-my-instance", "config.json")
	assert.Equal(t, expected, path)
}

func TestParseConfigPath_WithQuotedPath(t *testing.T) {
	path, err := ParseConfigPath(`nanobot gateway --config "C:/path_with_spaces/config.json"`, "test")
	require.NoError(t, err)
	assert.Equal(t, filepath.FromSlash("C:/path_with_spaces/config.json"), path)
}

func TestParseConfigPath_EmptyCommand(t *testing.T) {
	path, err := ParseConfigPath("", "test")
	require.NoError(t, err)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.Join(homeDir, ".nanobot-test", "config.json")
	assert.Equal(t, expected, path)
}

func TestParseConfigPath_WindowsBackslashPath(t *testing.T) {
	path, err := ParseConfigPath(`nanobot gateway --config C:\Users\test\.nanobot-helper\config.json`, "test")
	require.NoError(t, err)
	assert.Equal(t, `C:\Users\test\.nanobot-helper\config.json`, path)
}

func TestParseConfigPath_WindowsForwardSlashInCommand(t *testing.T) {
	path, err := ParseConfigPath("nanobot gateway --config C:/Users/test/.nanobot-helper/config.json", "test")
	require.NoError(t, err)
	// filepath.Abs normalizes to backslash on Windows
	assert.Equal(t, filepath.FromSlash("C:/Users/test/.nanobot-helper/config.json"), path)
}

// --- GenerateDefaultConfig tests ---

func TestGenerateDefaultConfig_FullStructure(t *testing.T) {
	cfg := GenerateDefaultConfig(18790, "~/.nanobot-test")

	// Verify all top-level keys exist
	_, hasAgents := cfg["agents"]
	_, hasChannels := cfg["channels"]
	_, hasProviders := cfg["providers"]
	_, hasGateway := cfg["gateway"]
	_, hasTools := cfg["tools"]

	assert.True(t, hasAgents, "expected 'agents' key")
	assert.True(t, hasChannels, "expected 'channels' key")
	assert.True(t, hasProviders, "expected 'providers' key")
	assert.True(t, hasGateway, "expected 'gateway' key")
	assert.True(t, hasTools, "expected 'tools' key")
}

func TestGenerateDefaultConfig_ParameterizedFields(t *testing.T) {
	cfg := GenerateDefaultConfig(18792, "~/.nanobot-test")

	// Verify gateway.port
	gateway, ok := cfg["gateway"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, uint32(18792), gateway["port"])

	// Verify agents.defaults.workspace
	agents, ok := cfg["agents"].(map[string]interface{})
	require.True(t, ok)
	defaults, ok := agents["defaults"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "~/.nanobot-test", defaults["workspace"])
}

func TestGenerateDefaultConfig_EmptySecrets(t *testing.T) {
	cfg := GenerateDefaultConfig(18790, "~/.nanobot-test")

	// Verify all apiKey fields are ""
	providers, ok := cfg["providers"].(map[string]interface{})
	require.True(t, ok)
	for name, provider := range providers {
		p, ok := provider.(map[string]interface{})
		require.True(t, ok, "provider %q should be a map", name)
		if apiKey, exists := p["apiKey"]; exists {
			assert.Equal(t, "", apiKey, "provider %q apiKey should be empty", name)
		}
	}

	// Verify telegram token is "" and enabled is false
	channels, ok := cfg["channels"].(map[string]interface{})
	require.True(t, ok)
	telegram, ok := channels["telegram"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "", telegram["token"])
	assert.Equal(t, false, telegram["enabled"])

	// Verify tools.web.search.apiKey is ""
	tools, ok := cfg["tools"].(map[string]interface{})
	require.True(t, ok)
	web, ok := tools["web"].(map[string]interface{})
	require.True(t, ok)
	search, ok := web["search"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "", search["apiKey"])
}

func TestGenerateDefaultConfig_DefaultValues(t *testing.T) {
	cfg := GenerateDefaultConfig(18790, "~/.nanobot-test")

	agents, ok := cfg["agents"].(map[string]interface{})
	require.True(t, ok)
	defaults, ok := agents["defaults"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "glm-5-turbo", defaults["model"])
	assert.Equal(t, "zhipu", defaults["provider"])
	assert.Equal(t, 131072, defaults["maxTokens"])
	assert.Equal(t, 0.7, defaults["temperature"])

	gateway, ok := cfg["gateway"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "0.0.0.0", gateway["host"])
}

func TestGenerateDefaultConfig_NoNameField(t *testing.T) {
	cfg := GenerateDefaultConfig(18790, "~/.nanobot-test")

	// Verify there is NO top-level "name" field
	_, hasName := cfg["name"]
	assert.False(t, hasName, "nanobot config should NOT have a top-level 'name' field")
}

// --- ReadConfig/WriteConfig tests ---

func TestWriteAndReadConfig_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	original := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port": float64(18790), // JSON numbers decode as float64
			"host": "0.0.0.0",
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": "glm-5-turbo",
			},
		},
	}

	err := cm.WriteConfig(configPath, original)
	require.NoError(t, err)

	readback, err := cm.ReadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, original["gateway"], readback["gateway"])
	assert.Equal(t, original["agents"], readback["agents"])
}

func TestReadConfig_FileNotFound(t *testing.T) {
	logger := newTestLogger()
	cm := NewConfigManager(logger)

	_, err := cm.ReadConfig("/nonexistent/path/config.json")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestWriteConfig_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "nested", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	data := map[string]interface{}{"test": "value"}
	err := cm.WriteConfig(configPath, data)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestConfigManager_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := map[string]interface{}{
				"writer": idx,
				"gateway": map[string]interface{}{
					"port": 18790 + idx,
				},
			}
			_ = cm.WriteConfig(configPath, data)
		}(i)
	}
	wg.Wait()

	// Verify final file is valid JSON (not corrupted)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err, "concurrent writes should produce valid JSON")
}

// --- CreateDefaultConfig tests ---

func TestCreateDefaultConfig_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".nanobot-test-instance", "config.json")
	startCommand := "nanobot gateway --config " + configPath

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	err := cm.CreateDefaultConfig("test-instance", 18792, startCommand)
	require.NoError(t, err)

	// Verify directory exists
	dirInfo, err := os.Stat(filepath.Dir(configPath))
	require.NoError(t, err)
	assert.True(t, dirInfo.IsDir())

	// Verify file exists
	fileInfo, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.False(t, fileInfo.IsDir())

	// Verify content is valid JSON with correct port
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	gateway, ok := config["gateway"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(18792), gateway["port"])
}

func TestCreateDefaultConfig_UsesUserHomeDir(t *testing.T) {
	// Verify workspace field uses ~/.nanobot-{name} form
	cfg := GenerateDefaultConfig(18790, "~/.nanobot-mynode")
	agents := cfg["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	assert.Equal(t, "~/.nanobot-mynode", defaults["workspace"])
}

// --- CloneConfig tests ---

func TestCloneConfig_CopiesAndUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source", "config.json")
	targetPath := filepath.Join(tmpDir, "target", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	// Create source config with port 18790
	sourceData := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port": 18790,
			"host": "0.0.0.0",
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"workspace": "~/.nanobot-source",
				"model":     "glm-5-turbo",
			},
		},
	}
	err := cm.WriteConfig(sourcePath, sourceData)
	require.NoError(t, err)

	// Clone with target port 18791
	err = cm.CloneConfig(
		"nanobot gateway --config "+sourcePath, "source",
		"target", 18791,
		"nanobot gateway --config "+targetPath,
	)
	require.NoError(t, err)

	// Verify target file has port 18791 and updated workspace
	targetData, err := cm.ReadConfig(targetPath)
	require.NoError(t, err)

	gateway := targetData["gateway"].(map[string]interface{})
	assert.Equal(t, float64(18791), gateway["port"])

	agents := targetData["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	assert.Equal(t, "~/.nanobot-target", defaults["workspace"])

	// Verify source data like model is preserved
	assert.Equal(t, "glm-5-turbo", defaults["model"])
}

func TestCloneConfig_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "nonexistent", "config.json")
	targetPath := filepath.Join(tmpDir, "target", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	// Clone from nonexistent source -- should generate default config for target
	err := cm.CloneConfig(
		"nanobot gateway --config "+sourcePath, "source",
		"target", 18791,
		"nanobot gateway --config "+targetPath,
	)
	require.NoError(t, err)

	// Verify target was created with default config
	targetData, err := cm.ReadConfig(targetPath)
	require.NoError(t, err)
	gateway := targetData["gateway"].(map[string]interface{})
	assert.Equal(t, float64(18791), gateway["port"])
}

func TestCloneConfig_OnlyUpdatesPortAndWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source", "config.json")
	targetPath := filepath.Join(tmpDir, "target", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	// Create source config with extra data that should be preserved
	sourceData := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port": 18790,
			"host": "0.0.0.0",
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"workspace":        "~/.nanobot-source",
				"model":            "glm-5-turbo",
				"provider":         "zhipu",
				"maxTokens":        131072,
				"temperature":      0.7,
				"maxToolIterations": 100,
			},
		},
		"providers": map[string]interface{}{
			"zhipu": map[string]interface{}{
				"apiKey": "some-test-key",
			},
		},
	}
	err := cm.WriteConfig(sourcePath, sourceData)
	require.NoError(t, err)

	err = cm.CloneConfig(
		"nanobot gateway --config "+sourcePath, "source",
		"target", 18795,
		"nanobot gateway --config "+targetPath,
	)
	require.NoError(t, err)

	targetData, err := cm.ReadConfig(targetPath)
	require.NoError(t, err)

	// Verify NO top-level "name" field
	_, hasName := targetData["name"]
	assert.False(t, hasName, "cloned config should NOT have a top-level 'name' field")

	// Verify only port and workspace changed
	gateway := targetData["gateway"].(map[string]interface{})
	assert.Equal(t, float64(18795), gateway["port"])
	assert.Equal(t, "0.0.0.0", gateway["host"]) // preserved

	agents := targetData["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	assert.Equal(t, "~/.nanobot-target", defaults["workspace"]) // updated
	assert.Equal(t, "glm-5-turbo", defaults["model"])           // preserved
	assert.Equal(t, "zhipu", defaults["provider"])              // preserved
	assert.Equal(t, float64(131072), defaults["maxTokens"])     // preserved (JSON roundtrip: int -> float64)

	providers := targetData["providers"].(map[string]interface{})
	zhipu := providers["zhipu"].(map[string]interface{})
	assert.Equal(t, "some-test-key", zhipu["apiKey"]) // preserved
}

// --- CleanupConfig tests ---

func TestCleanupConfig_RemovesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".nanobot-test-cleanup", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	// Create a config file
	err := cm.WriteConfig(configPath, map[string]interface{}{"test": "data"})
	require.NoError(t, err)

	// Verify directory exists
	_, err = os.Stat(filepath.Dir(configPath))
	require.NoError(t, err)

	// Cleanup
	err = cm.CleanupConfig("nanobot gateway --config "+configPath, "test-cleanup")
	require.NoError(t, err)

	// Verify directory no longer exists
	_, err = os.Stat(filepath.Dir(configPath))
	assert.True(t, os.IsNotExist(err), "directory should be removed after CleanupConfig")
}

func TestCleanupConfig_NonexistentDirectory(t *testing.T) {
	// CleanupConfig for a nonexistent directory should not error
	// since os.RemoveAll does not return error for nonexistent paths
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "config.json")

	logger := newTestLogger()
	cm := NewConfigManager(logger)

	err := cm.CleanupConfig("nanobot gateway --config "+configPath, "nonexistent")
	assert.NoError(t, err)
}

// --- Helper ---

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
