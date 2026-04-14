// Package nanobot provides utilities for managing nanobot instance configurations.
// Phase 52: NanobotConfigManager handles reading and writing nanobot config.json files
// for each managed instance, including path parsing from start_command, default config
// generation, and thread-safe file operations.
package nanobot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// configPathRegex extracts the --config parameter value from a start_command string.
// Matches: --config /path/to/file, --config "/path/with spaces", --config 'C:\path'
var configPathRegex = regexp.MustCompile(`--config\s+["']?([^"'\s]+)["']?`)

// ConfigManager manages nanobot config.json files for instances.
// It provides thread-safe file read/write operations and path resolution.
type ConfigManager struct {
	mu     sync.Mutex
	logger *slog.Logger
}

// NewConfigManager creates a new ConfigManager with the given logger.
func NewConfigManager(logger *slog.Logger) *ConfigManager {
	return &ConfigManager{
		logger: logger.With("source", "nanobot-config-manager"),
	}
}

// resolveWorkspace returns the workspace path for a nanobot instance based on its start_command.
// With --config: uses ~/.nanobot-{instanceName} (instance-specific directory).
// Without --config: uses ~/.nanobot (nanobot's default directory).
func resolveWorkspace(startCommand, instanceName string) string {
	matches := configPathRegex.FindStringSubmatch(startCommand)
	if len(matches) >= 2 {
		return "~/.nanobot-" + instanceName
	}
	return "~/.nanobot"
}

// ParseConfigPath extracts the nanobot config.json path from a start_command.
// D-01: Uses regex to extract --config parameter value from startCommand.
// D-02: Falls back to ~/.nanobot/config.json when --config is absent (nanobot gateway default).
// D-03: Resolves ~ using os.UserHomeDir() and constructs paths with filepath.Join.
func ParseConfigPath(startCommand, instanceName string) (string, error) {
	matches := configPathRegex.FindStringSubmatch(startCommand)
	if len(matches) >= 2 {
		configPath := matches[1]
		// Expand ~ to home directory using os.UserHomeDir()
		if len(configPath) > 0 && configPath[0] == '~' {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to resolve home directory: %w", err)
			}
			configPath = filepath.Join(homeDir, configPath[1:])
		}
		// Return absolute path
		absPath, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for %q: %w", configPath, err)
		}
		return absPath, nil
	}

	// Fallback: ~/.nanobot/config.json
	// When start_command has no --config (e.g., "nanobot gateway"), nanobot uses
	// ~/.nanobot/config.json as its default config path.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".nanobot", "config.json"), nil
}

// GenerateDefaultConfig creates a default nanobot configuration map.
// D-04: Full structure with agents, channels, providers, gateway, tools.
// D-05: gateway.port and agents.defaults.workspace are parameterized.
// D-06: Sensible defaults: model="glm-5-turbo", provider="zhipu", maxTokens=131072.
// D-07: Telegram channel disabled by default.
// D-08: Provider structures preserved with empty apiKeys.
func GenerateDefaultConfig(port uint32, workspace string) map[string]interface{} {
	return map[string]interface{}{
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"workspace":        workspace,
				"model":            "glm-5-turbo",
				"provider":         "zhipu",
				"maxTokens":        131072,
				"temperature":      0.7,
				"maxToolIterations": 100,
				"memoryWindow":     50,
			},
		},
		"channels": map[string]interface{}{
			"telegram": map[string]interface{}{
				"enabled":   false,
				"token":     "",
				"allowFrom": []interface{}{},
				"proxy":     nil,
			},
		},
		"providers": map[string]interface{}{
			"zhipu": map[string]interface{}{
				"apiKey":       "",
				"apiBase":      "https://open.bigmodel.cn/api/coding/paas/v4/",
				"extraHeaders": nil,
			},
			"groq": map[string]interface{}{
				"apiKey":       "",
				"apiBase":      nil,
				"extraHeaders": nil,
			},
			"aihubmix": map[string]interface{}{
				"apiKey":       "",
				"apiBase":      nil,
				"extraHeaders": nil,
			},
		},
		"gateway": map[string]interface{}{
			"host": "0.0.0.0",
			"port": port,
		},
		"tools": map[string]interface{}{
			"web": map[string]interface{}{
				"search": map[string]interface{}{
					"apiKey":     "",
					"maxResults": 5,
				},
			},
			"exec": map[string]interface{}{
				"timeout": 60,
			},
			"restrictToWorkspace": false,
			"mcpServers":          map[string]interface{}{},
		},
	}
}

// ReadConfig reads a nanobot config.json file and returns its content as a map.
// Returns nil and os.ErrNotExist if the file does not exist (caller decides behavior).
func (cm *ConfigManager) ReadConfig(configPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("failed to read nanobot config %q: %w", configPath, err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse nanobot config %q: %w", configPath, err)
	}

	return config, nil
}

// WriteConfig writes a nanobot config map to a file with mutex-protected file operations.
// D-13: Uses sync.Mutex for concurrent write safety.
// Creates parent directories if they do not exist.
func (cm *ConfigManager) WriteConfig(configPath string, data map[string]interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nanobot config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	if err := os.WriteFile(configPath, bytes, 0644); err != nil {
		return fmt.Errorf("failed to write nanobot config %q: %w", configPath, err)
	}

	cm.logger.Info("Nanobot config written", "path", configPath)
	return nil
}

// CreateDefaultConfig generates and writes a default nanobot config.json for an instance.
// NC-01: Auto-creates nanobot config directory and default config file.
// Uses ~/.nanobot-{instanceName} form for workspace in the config (nanobot reads this form),
// but resolves actual paths via os.UserHomeDir() for file operations.
func (cm *ConfigManager) CreateDefaultConfig(instanceName string, port uint32, startCommand string) error {
	configPath, err := ParseConfigPath(startCommand, instanceName)
	if err != nil {
		return fmt.Errorf("failed to parse config path for instance %q: %w", instanceName, err)
	}

	// Determine workspace based on whether start_command has --config.
	// With --config: workspace matches the config directory (~/.nanobot-{name}).
	// Without --config: nanobot default workspace is ~/.nanobot.
	matches := configPathRegex.FindStringSubmatch(startCommand)
	workspace := "~/.nanobot"
	if len(matches) >= 2 {
		workspace = "~/.nanobot-" + instanceName
	}

	defaultConfig := GenerateDefaultConfig(port, workspace)

	if err := cm.WriteConfig(configPath, defaultConfig); err != nil {
		return fmt.Errorf("failed to create default nanobot config for instance %q: %w", instanceName, err)
	}

	cm.logger.Info("Default nanobot config created", "instance", instanceName, "path", configPath)
	return nil
}

// CloneConfig clones a source instance's nanobot config to a target instance.
// NC-04: Copies nanobot config.json, updating gateway.port and agents.defaults.workspace.
// If source config file does not exist, generates a default config instead (assumption A2).
// The nanobot config.json does NOT have a top-level "name" field; only gateway.port
// and agents.defaults.workspace are updated during cloning.
func (cm *ConfigManager) CloneConfig(sourceStartCommand, sourceInstanceName, targetInstanceName string, targetPort uint32, targetStartCommand string) error {
	sourceConfigPath, err := ParseConfigPath(sourceStartCommand, sourceInstanceName)
	if err != nil {
		return fmt.Errorf("failed to parse source config path: %w", err)
	}

	targetConfigPath, err := ParseConfigPath(targetStartCommand, targetInstanceName)
	if err != nil {
		return fmt.Errorf("failed to parse target config path: %w", err)
	}

	// Safety guard: if source and target resolve to the same config file, skip cloning.
	// This prevents silently overwriting the source instance's config (port, workspace, skills).
	// The caller (HandleCopy) should normally prevent this by auto-generating a unique
	// --config path, but this guard protects against any remaining code path.
	if sourceConfigPath == targetConfigPath {
		cm.logger.Warn("CloneConfig: source and target config paths are identical, skipping clone to prevent corruption",
			"source_instance", sourceInstanceName,
			"target_instance", targetInstanceName,
			"config_path", sourceConfigPath)
		return nil
	}

	// Read source config; generate default if missing
	configData, err := cm.ReadConfig(sourceConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			cm.logger.Warn("Source nanobot config not found, generating default",
				"source_instance", sourceInstanceName, "source_path", sourceConfigPath)
			workspace := resolveWorkspace(targetStartCommand, targetInstanceName)
			configData = GenerateDefaultConfig(targetPort, workspace)
		} else {
			return fmt.Errorf("failed to read source nanobot config: %w", err)
		}
	}

	// Update gateway.port
	if gateway, ok := configData["gateway"].(map[string]interface{}); ok {
		gateway["port"] = targetPort
	}

	// Update agents.defaults.workspace to target instance name
	if agents, ok := configData["agents"].(map[string]interface{}); ok {
		if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
			defaults["workspace"] = resolveWorkspace(targetStartCommand, targetInstanceName)
		}
	}

	if err := cm.WriteConfig(targetConfigPath, configData); err != nil {
		return fmt.Errorf("failed to write cloned nanobot config: %w", err)
	}

	cm.logger.Info("Nanobot config cloned",
		"source_instance", sourceInstanceName,
		"target_instance", targetInstanceName,
		"target_path", targetConfigPath,
	)
	return nil
}

// UpdateStartCommandConfig replaces or appends the --config flag in a start_command.
// If the start_command already contains --config, the path is replaced.
// If not, --config <path> is appended to the command.
func UpdateStartCommandConfig(startCommand, configPath string) string {
	if configPathRegex.MatchString(startCommand) {
		// Replace existing --config value
		return configPathRegex.ReplaceAllString(startCommand, "--config "+configPath)
	}
	// Append --config to the command
	return startCommand + " --config " + configPath
}

// UpdateInstanceConfig updates a nanobot config when an instance's port or startCommand changes.
// If the config path changed (startCommand modified), the old config is read and written to the
// new location with updated port and workspace. The old config file is preserved (not deleted).
// If the config path is unchanged, only gateway.port and agents.defaults.workspace are updated.
func (cm *ConfigManager) UpdateInstanceConfig(instanceName string, oldPort uint32, oldStartCommand string, newPort uint32, newStartCommand string) error {
	oldPath, err := ParseConfigPath(oldStartCommand, instanceName)
	if err != nil {
		return fmt.Errorf("failed to parse old config path: %w", err)
	}

	newPath, err := ParseConfigPath(newStartCommand, instanceName)
	if err != nil {
		return fmt.Errorf("failed to parse new config path: %w", err)
	}

	// Read existing config from the old path (or the new path if they're the same)
	readPath := oldPath
	if oldPath == newPath {
		readPath = newPath
	}

	configData, err := cm.ReadConfig(readPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing config: generate a default at the new path
			cm.logger.Warn("Nanobot config not found during update, generating default",
				"instance", instanceName, "path", readPath)
			workspace := resolveWorkspace(newStartCommand, instanceName)
			configData = GenerateDefaultConfig(newPort, workspace)
		} else {
			return fmt.Errorf("failed to read nanobot config for update: %w", err)
		}
	}

	// Update gateway.port
	if gateway, ok := configData["gateway"].(map[string]interface{}); ok {
		gateway["port"] = newPort
	}

	// Update agents.defaults.workspace
	if agents, ok := configData["agents"].(map[string]interface{}); ok {
		if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
			defaults["workspace"] = resolveWorkspace(newStartCommand, instanceName)
		}
	}

	if err := cm.WriteConfig(newPath, configData); err != nil {
		return fmt.Errorf("failed to write updated nanobot config: %w", err)
	}

	cm.logger.Info("Nanobot config updated for instance",
		"instance", instanceName,
		"old_path", oldPath,
		"new_path", newPath,
		"new_port", newPort,
	)
	return nil
}

// CleanupConfig removes the nanobot config for an instance.
// For instance-specific directories (e.g., ~/.nanobot-{name}/), removes the entire directory.
// For the default ~/.nanobot/ directory, only removes the config.json file to preserve
// other nanobot data (workspace, etc.) that may be shared.
func (cm *ConfigManager) CleanupConfig(startCommand, instanceName string) error {
	configPath, err := ParseConfigPath(startCommand, instanceName)
	if err != nil {
		return fmt.Errorf("failed to parse config path: %w", err)
	}

	// Check if this is the default ~/.nanobot/config.json path
	homeDir, _ := os.UserHomeDir()
	defaultConfigPath := filepath.Join(homeDir, ".nanobot", "config.json")

	if configPath == defaultConfigPath {
		// Default path: only remove the config file, not the shared directory
		if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove nanobot config %s: %w", configPath, err)
		}
		cm.logger.Info("Nanobot config file removed", "instance", instanceName, "path", configPath)
	} else {
		// Instance-specific path: remove the entire directory
		configDir := filepath.Dir(configPath)
		if err := os.RemoveAll(configDir); err != nil {
			return fmt.Errorf("failed to remove nanobot config directory %s: %w", configDir, err)
		}
		cm.logger.Info("Nanobot config directory removed", "instance", instanceName, "path", configDir)
	}
	return nil
}
