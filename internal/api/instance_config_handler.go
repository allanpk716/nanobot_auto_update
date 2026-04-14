package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/nanobot"
)

// instanceConfigRequest is the JSON body for create/update/copy requests.
// startup_timeout is in seconds (uint32) per D-06.
type instanceConfigRequest struct {
	Name           string `json:"name"`
	Port           uint32 `json:"port"`
	StartCommand   string `json:"start_command"`
	StartupTimeout uint32 `json:"startup_timeout"` // seconds, converted to time.Duration internally
	AutoStart      *bool  `json:"auto_start"`
}

// instanceConfigResponse is the JSON response for a single instance config.
// startup_timeout is in seconds (uint32) per D-06.
type instanceConfigResponse struct {
	Name           string `json:"name"`
	Port           uint32 `json:"port"`
	StartCommand   string `json:"start_command"`
	StartupTimeout uint32 `json:"startup_timeout"`
	AutoStart      *bool  `json:"auto_start"`
}

// validationErrorDetail represents a single field validation error.
type validationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// validationErrorResponse is the 422 error response (D-14).
type validationErrorResponse struct {
	Error   string                  `json:"error"`
	Message string                  `json:"message"`
	Errors  []validationErrorDetail `json:"errors"`
}

// validationError is a custom error type wrapping validation details
// so that handlers can distinguish validation failures from other errors.
type validationError struct {
	details []validationErrorDetail
}

func (e *validationError) Error() string {
	return "validation failed"
}

// notFoundError is a custom error type indicating the requested instance was not found.
type notFoundError struct {
	name string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("instance %q not found", e.name)
}

// InstanceConfigHandler handles CRUD operations for instance configurations.
//
// Optional lifecycle callbacks (Phase 52):
// These callbacks are injected via setter methods to extend instance lifecycle
// events with nanobot config management. They are nil by default and safe to ignore
// for tests that don't need nanobot config behavior.
//
//   - onCreateInstance: Called after a new instance is persisted to config.yaml.
//     Creates the nanobot config directory and default config.json.
//     Failure is non-blocking (logged as warning).
//
//   - onCopyInstance: Called after a copied instance is persisted to config.yaml.
//     Clones the source nanobot config.json to the new directory with updated port/workspace.
//     Failure is non-blocking (logged as warning).
//
//   - onDeleteInstance: Called after an instance is removed from config.yaml.
//     Removes the nanobot config directory for the deleted instance.
//     Failure is non-blocking (logged as warning).
//
// Optional onStopInstance (for targeted instance stop on delete):
//   - onStopInstance: Called when an instance is deleted to stop only that instance
//     by PID, instead of killing all nanobot processes system-wide.
type InstanceConfigHandler struct {
	getConfig        func() *config.Config // injected for testability; production uses config.GetCurrentConfig
	logger           *slog.Logger
	onCreateInstance func(name string, port uint32, startCommand string) error                                                                                                                                  // Phase 52: nanobot config creation
	onCopyInstance   func(sourceName string, sourceStartCommand string, targetName string, targetPort uint32, targetStartCommand string) error                                                                  // Phase 52: nanobot config clone
	onDeleteInstance func(name string, startCommand string) error                                                                                                                                               // Phase 52: nanobot config directory cleanup
	onStopInstance   func(ctx context.Context, name string) error                                                                                                                                                // targeted instance stop by PID
}

// NewInstanceConfigHandler creates a new InstanceConfigHandler.
// getConfig is called on each request to read the current config (supports hot reload).
func NewInstanceConfigHandler(getConfig func() *config.Config, logger *slog.Logger) *InstanceConfigHandler {
	return &InstanceConfigHandler{
		getConfig: getConfig,
		logger:    logger.With("source", "api-instance-config"),
	}
}

// SetOnCreateInstance sets the callback invoked after creating a new instance.
// The callback receives the instance name, port, and startCommand.
func (h *InstanceConfigHandler) SetOnCreateInstance(fn func(name string, port uint32, startCommand string) error) {
	h.onCreateInstance = fn
}

// SetOnCopyInstance sets the callback invoked after copying an instance.
// The callback receives the source instance info and target instance info.
func (h *InstanceConfigHandler) SetOnCopyInstance(fn func(sourceName string, sourceStartCommand string, targetName string, targetPort uint32, targetStartCommand string) error) {
	h.onCopyInstance = fn
}

// SetOnDeleteInstance sets the callback invoked after deleting an instance.
// The callback receives the instance name and startCommand for path resolution.
func (h *InstanceConfigHandler) SetOnDeleteInstance(fn func(name string, startCommand string) error) {
	h.onDeleteInstance = fn
}

// SetOnStopInstance sets the callback invoked to stop a specific instance by PID
// when it is deleted. This replaces the old StopAllNanobots approach.
// The callback receives a context and the instance name.
func (h *InstanceConfigHandler) SetOnStopInstance(fn func(ctx context.Context, name string) error) {
	h.onStopInstance = fn
}

// toResponse converts an internal InstanceConfig to a JSON response.
// StartupTimeout is converted from time.Duration to seconds.
func toResponse(ic config.InstanceConfig) instanceConfigResponse {
	return instanceConfigResponse{
		Name:           ic.Name,
		Port:           ic.Port,
		StartCommand:   ic.StartCommand,
		StartupTimeout: uint32(ic.StartupTimeout.Seconds()),
		AutoStart:      ic.AutoStart,
	}
}

// toInstanceConfig converts a JSON request to an internal InstanceConfig.
// StartupTimeout is converted from seconds to time.Duration.
func toInstanceConfig(req instanceConfigRequest) config.InstanceConfig {
	ic := config.InstanceConfig{
		Name:         req.Name,
		Port:         req.Port,
		StartCommand: req.StartCommand,
		AutoStart:    req.AutoStart,
	}
	if req.StartupTimeout > 0 {
		ic.StartupTimeout = time.Duration(req.StartupTimeout) * time.Second
	}
	return ic
}

// writeValidationError writes a 422 response with field-level validation errors (D-14).
func writeValidationError(w http.ResponseWriter, message string, details []validationErrorDetail) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(validationErrorResponse{
		Error:   "validation_error",
		Message: message,
		Errors:  details,
	})
}

// findInstanceByName finds an instance by name, returning its index and pointer.
// Returns (-1, nil) if not found.
func findInstanceByName(cfg *config.Config, name string) (int, *config.InstanceConfig) {
	for i := range cfg.Instances {
		if cfg.Instances[i].Name == name {
			return i, &cfg.Instances[i]
		}
	}
	return -1, nil
}

// validateInstanceConfig collects ALL validation errors (field + uniqueness) in one pass.
// excludeIndex is the index of the instance being updated (to exclude it from uniqueness checks).
func validateInstanceConfig(ic *config.InstanceConfig, instances []config.InstanceConfig, excludeIndex int) []validationErrorDetail {
	var details []validationErrorDetail

	// Field validation via InstanceConfig.Validate()
	if err := ic.Validate(); err != nil {
		details = append(details, validationErrorDetail{
			Field:   "instance",
			Message: err.Error(),
		})
	}

	// Unique name check
	for i, inst := range instances {
		if i == excludeIndex {
			continue
		}
		if inst.Name == ic.Name {
			details = append(details, validationErrorDetail{
				Field:   "name",
				Message: fmt.Sprintf("Instance name %q already exists", ic.Name),
			})
			break
		}
	}

	// Unique port check
	for i, inst := range instances {
		if i == excludeIndex {
			continue
		}
		if inst.Port == ic.Port {
			details = append(details, validationErrorDetail{
				Field:   "port",
				Message: fmt.Sprintf("Port %d is already used by instance %q", ic.Port, inst.Name),
			})
			break
		}
	}

	return details
}

// HandleList handles GET /api/v1/instance-configs
func (h *InstanceConfigHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	cfg := h.getConfig()
	if cfg == nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Config not initialized")
		return
	}

	instances := make([]instanceConfigResponse, len(cfg.Instances))
	for i, ic := range cfg.Instances {
		instances[i] = toResponse(ic)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"instances": instances,
	})
}

// HandleGet handles GET /api/v1/instance-configs/{name}
func (h *InstanceConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	cfg := h.getConfig()
	if cfg == nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Config not initialized")
		return
	}

	_, ic := findInstanceByName(cfg, name)
	if ic == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", fmt.Sprintf("Instance %q not found", name))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toResponse(*ic))
}

// HandleCreate handles POST /api/v1/instance-configs
func (h *InstanceConfigHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req instanceConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	ic := toInstanceConfig(req)

	err := config.UpdateConfig(func(cfg *config.Config) error {
		details := validateInstanceConfig(&ic, cfg.Instances, -1)
		if len(details) > 0 {
			return &validationError{details: details}
		}
		cfg.Instances = append(cfg.Instances, ic)
		return nil
	})
	if err != nil {
		var valErr *validationError
		if errors.As(err, &valErr) {
			h.writeValidationError(w, "Validation failed", valErr.details)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.logger.Info("Instance config created", "name", ic.Name)

	// Phase 52: Create nanobot config directory with default config (NC-01)
	if h.onCreateInstance != nil {
		if err := h.onCreateInstance(ic.Name, ic.Port, ic.StartCommand); err != nil {
			h.logger.Warn("Failed to create nanobot config for new instance",
				"name", ic.Name, "error", err)
			// Non-blocking: instance is created, nanobot config can be fixed via PUT endpoint
			// or auto-created on next GET (lazy-creation fallback in HandleGet)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(ic))
}

// HandleUpdate handles PUT /api/v1/instance-configs/{name}
func (h *InstanceConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	pathName := r.PathValue("name")

	var req instanceConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	ic := toInstanceConfig(req)

	// Name immutability: if name is provided and differs from path, reject
	if req.Name != "" && req.Name != pathName {
		h.writeValidationError(w, "Validation failed", []validationErrorDetail{
			{Field: "name", Message: "Instance name cannot be changed"},
		})
		return
	}
	// If name is empty in body, use the path name
	ic.Name = pathName

	err := config.UpdateConfig(func(cfg *config.Config) error {
		existingIndex, _ := findInstanceByName(cfg, pathName)
		if existingIndex == -1 {
			return &notFoundError{name: pathName}
		}

		details := validateInstanceConfig(&ic, cfg.Instances, existingIndex)
		if len(details) > 0 {
			return &validationError{details: details}
		}

		cfg.Instances[existingIndex] = ic
		return nil
	})
	if err != nil {
		var nfErr *notFoundError
		if errors.As(err, &nfErr) {
			writeJSONError(w, http.StatusNotFound, "not_found", nfErr.Error())
			return
		}
		var valErr *validationError
		if errors.As(err, &valErr) {
			h.writeValidationError(w, "Validation failed", valErr.details)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.logger.Info("Instance config updated", "name", ic.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toResponse(ic))
}

// HandleDelete handles DELETE /api/v1/instance-configs/{name}
func (h *InstanceConfigHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Capture instance info before UpdateConfig removes it
	var deletedStartCommand string

	err := config.UpdateConfig(func(cfg *config.Config) error {
		index, ic := findInstanceByName(cfg, name)
		if index == -1 {
			return &notFoundError{name: name}
		}
		deletedStartCommand = ic.StartCommand
		cfg.Instances = append(cfg.Instances[:index], cfg.Instances[index+1:]...)
		return nil
	})
	if err != nil {
		var nfErr *notFoundError
		if errors.As(err, &nfErr) {
			writeJSONError(w, http.StatusNotFound, "not_found", nfErr.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Stop only the deleted instance (not all instances).
	// Uses onStopInstance callback to target the specific PID.
	if h.onStopInstance != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := h.onStopInstance(ctx, name); err != nil {
			h.logger.Warn("Failed to stop deleted instance, it may exit on its own",
				"name", name, "error", err)
		} else {
			h.logger.Info("Stopped deleted instance", "name", name)
		}
	}

	// Phase 52: Clean up nanobot config directory for deleted instance.
	// Skip cleanup if other instances share the same config path (default gateway).
	if h.onDeleteInstance != nil {
		skipCleanup := h.shouldSkipConfigCleanup(name, deletedStartCommand)
		if skipCleanup {
			h.logger.Info("Skipping nanobot config cleanup: other instances share the same config path",
				"name", name, "start_command", deletedStartCommand)
		} else if err := h.onDeleteInstance(name, deletedStartCommand); err != nil {
			h.logger.Warn("Failed to clean up nanobot config for deleted instance",
				"name", name, "error", err)
			// Non-blocking: instance is deleted from config.yaml, orphaned dir can be cleaned manually
		}
	}

	h.logger.Info("Instance config deleted", "name", name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Instance %q deleted", name),
	})
}

// shouldSkipConfigCleanup returns true if other instances in the config share
// the same nanobot config path as the deleted instance. This prevents deleting
// a shared config file (e.g., ~/.nanobot/config.json) that other default gateway
// instances depend on.
func (h *InstanceConfigHandler) shouldSkipConfigCleanup(deletedName, deletedStartCommand string) bool {
	cfg := h.getConfig()
	if cfg == nil {
		return false
	}

	// Resolve the deleted instance's config path
	deletedPath, err := nanobot.ParseConfigPath(deletedStartCommand, deletedName)
	if err != nil {
		return false
	}

	// Check if any remaining instance resolves to the same path
	for _, ic := range cfg.Instances {
		otherPath, err := nanobot.ParseConfigPath(ic.StartCommand, ic.Name)
		if err != nil {
			continue
		}
		if otherPath == deletedPath {
			return true // Another instance uses the same config file
		}
	}
	return false
}

// HandleCopy handles POST /api/v1/instance-configs/{name}/copy
func (h *InstanceConfigHandler) HandleCopy(w http.ResponseWriter, r *http.Request) {
	sourceName := r.PathValue("name")

	// Handle empty body gracefully (MEDIUM review concern)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Failed to read request body")
		return
	}

	var req instanceConfigRequest
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
			return
		}
	}

	var clonedInstance config.InstanceConfig
	var sourceStartCommand string

	err = config.UpdateConfig(func(cfg *config.Config) error {
		sourceIndex, sourceIC := findInstanceByName(cfg, sourceName)
		if sourceIndex == -1 {
			return &notFoundError{name: sourceName}
		}

		// Capture sourceStartCommand before any modifications
		sourceStartCommand = sourceIC.StartCommand

		// Clone the source instance config
		clonedInstance = *sourceIC

		// Name (D-11): use provided name or auto-generate
		if req.Name != "" {
			clonedInstance.Name = req.Name
		} else {
			clonedInstance.Name = sourceName + "-copy"
		}

		// Port (D-12): use provided port or auto-increment
		if req.Port != 0 {
			clonedInstance.Port = req.Port
		} else {
			// Auto-increment from source port + 1
			candidatePort := sourceIC.Port + 1
			maxAttempts := 100
			found := false
			for i := 0; i < maxAttempts; i++ {
				port := candidatePort + uint32(i)
				if port > 65535 {
					break
				}
				inUse := false
				for _, inst := range cfg.Instances {
					if inst.Port == port {
						inUse = true
						break
					}
				}
				if !inUse {
					clonedInstance.Port = port
					found = true
					break
				}
			}
			if !found {
				return &validationError{details: []validationErrorDetail{
					{Field: "port", Message: "Could not find an available port after 100 attempts"},
				}}
			}
		}

		// Override other fields if provided
		if req.StartCommand != "" {
			clonedInstance.StartCommand = req.StartCommand
		}
		if req.StartupTimeout > 0 {
			clonedInstance.StartupTimeout = time.Duration(req.StartupTimeout) * time.Second
		}
		if req.AutoStart != nil {
			clonedInstance.AutoStart = req.AutoStart
		}

		// Deep copy AutoStart pointer for the cloned instance
		if clonedInstance.AutoStart != nil {
			val := *clonedInstance.AutoStart
			clonedInstance.AutoStart = &val
		}

		// Prevent config path collision: if the copy resolves to the same config file
		// as the source, auto-generate a unique --config path for the copy.
		// This prevents CloneConfig from silently overwriting the source's config
		// (which would corrupt the source instance's port, workspace, and skills).
		sourceCfgPath, _ := nanobot.ParseConfigPath(sourceStartCommand, sourceName)
		targetCfgPath, _ := nanobot.ParseConfigPath(clonedInstance.StartCommand, clonedInstance.Name)
		if sourceCfgPath == targetCfgPath {
			uniqueConfigPath := fmt.Sprintf("~/.nanobot-%s/config.json", clonedInstance.Name)
			newStartCmd := nanobot.UpdateStartCommandConfig(clonedInstance.StartCommand, uniqueConfigPath)
			clonedInstance.StartCommand = newStartCmd
			h.logger.Info("Auto-generated unique config path for copied instance",
				"instance", clonedInstance.Name, "config_path", uniqueConfigPath)
		}

		details := validateInstanceConfig(&clonedInstance, cfg.Instances, -1)
		if len(details) > 0 {
			return &validationError{details: details}
		}

		cfg.Instances = append(cfg.Instances, clonedInstance)
		return nil
	})
	if err != nil {
		var nfErr *notFoundError
		if errors.As(err, &nfErr) {
			writeJSONError(w, http.StatusNotFound, "not_found", nfErr.Error())
			return
		}
		var valErr *validationError
		if errors.As(err, &valErr) {
			h.writeValidationError(w, "Validation failed", valErr.details)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	h.logger.Info("Instance config copied", "source", sourceName, "new_name", clonedInstance.Name, "new_port", clonedInstance.Port)

	// Phase 52: Clone nanobot config to new instance directory (NC-04)
	// Note: Only gateway.port and agents.defaults.workspace are updated in the cloned config.
	// nanobot config.json has no top-level "name" field.
	if h.onCopyInstance != nil {
		if err := h.onCopyInstance(sourceName, sourceStartCommand, clonedInstance.Name, clonedInstance.Port, clonedInstance.StartCommand); err != nil {
			h.logger.Warn("Failed to clone nanobot config for copied instance",
				"source", sourceName, "target", clonedInstance.Name, "error", err)
			// Non-blocking: instance is copied, nanobot config can be fixed via PUT endpoint
			// or auto-created on next GET (lazy-creation fallback in HandleGet)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(clonedInstance))
}

// writeValidationError is a method variant that delegates to the package-level function.
func (h *InstanceConfigHandler) writeValidationError(w http.ResponseWriter, message string, details []validationErrorDetail) {
	writeValidationError(w, message, details)
}
