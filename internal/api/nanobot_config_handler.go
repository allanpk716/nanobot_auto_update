package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/nanobot"
)

// NanobotConfigHandler handles GET/PUT operations for nanobot config.json per instance.
// Phase 52: NC-02 (GET), NC-03 (PUT).
type NanobotConfigHandler struct {
	manager   *nanobot.ConfigManager
	getConfig func() *config.Config
	logger    *slog.Logger
}

// NewNanobotConfigHandler creates a new NanobotConfigHandler.
func NewNanobotConfigHandler(manager *nanobot.ConfigManager, getConfig func() *config.Config, logger *slog.Logger) *NanobotConfigHandler {
	return &NanobotConfigHandler{
		manager:   manager,
		getConfig: getConfig,
		logger:    logger.With("source", "api-nanobot-config"),
	}
}

// findInstanceByNameForNanobotConfig finds an instance by name, returning its pointer.
// Returns nil if not found.
func findInstanceByNameForNanobotConfig(cfg *config.Config, name string) *config.InstanceConfig {
	for i := range cfg.Instances {
		if cfg.Instances[i].Name == name {
			return &cfg.Instances[i]
		}
	}
	return nil
}

// HandleGet handles GET /api/v1/instances/{name}/nanobot-config (NC-02).
// Returns the nanobot config.json content for a valid instance.
// Includes lazy-creation fallback: if the nanobot config file is missing for a
// known instance, auto-creates a default config and returns it.
func (h *NanobotConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	cfg := h.getConfig()
	if cfg == nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Config not initialized")
		return
	}

	ic := findInstanceByNameForNanobotConfig(cfg, name)
	if ic == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
		return
	}

	configPath, err := nanobot.ParseConfigPath(ic.StartCommand, ic.Name)
	if err != nil {
		h.logger.Error("Failed to parse nanobot config path", "instance", ic.Name, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve nanobot config path")
		return
	}

	configData, err := h.manager.ReadConfig(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// LAZY-CREATION FALLBACK: auto-create default nanobot config for known instance
			h.logger.Warn("Nanobot config missing for known instance, auto-creating default config",
				"instance", ic.Name, "path", configPath)
			if createErr := h.manager.CreateDefaultConfig(ic.Name, ic.Port, ic.StartCommand); createErr != nil {
				h.logger.Error("Failed to auto-create nanobot config", "instance", ic.Name, "error", createErr)
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to create nanobot config")
				return
			}
			// Re-read the freshly created config
			configData, err = h.manager.ReadConfig(configPath)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to read nanobot config")
				return
			}
		} else {
			h.logger.Error("Failed to read nanobot config", "instance", ic.Name, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to read nanobot config")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config":   configData,
		"instance": name,
	})
}

// HandlePut handles PUT /api/v1/instances/{name}/nanobot-config (NC-03, D-13).
// Writes the nanobot config.json file for the specified instance.
// D-13: Only writes the file, does not restart the instance.
// D-14: Response includes informational hint about restarting.
func (h *NanobotConfigHandler) HandlePut(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	cfg := h.getConfig()
	if cfg == nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Config not initialized")
		return
	}

	ic := findInstanceByNameForNanobotConfig(cfg, name)
	if ic == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
		return
	}

	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	configPath, err := nanobot.ParseConfigPath(ic.StartCommand, ic.Name)
	if err != nil {
		h.logger.Error("Failed to parse nanobot config path", "instance", ic.Name, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve nanobot config path")
		return
	}

	if err := h.manager.WriteConfig(configPath, reqBody); err != nil {
		h.logger.Error("Failed to write nanobot config", "instance", ic.Name, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to write nanobot config")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  fmt.Sprintf("Nanobot config updated for instance %q", name),
		"instance": name,
		"hint":     "Restart the instance via POST /api/v1/instances/{name}/stop + POST /api/v1/instances/{name}/start to apply changes",
	})
}
