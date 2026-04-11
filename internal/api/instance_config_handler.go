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
	"github.com/HQGroup/nanobot-auto-updater/internal/lifecycle"
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
type InstanceConfigHandler struct {
	getConfig func() *config.Config // injected for testability; production uses config.GetCurrentConfig
	logger    *slog.Logger
}

// NewInstanceConfigHandler creates a new InstanceConfigHandler.
// getConfig is called on each request to read the current config (supports hot reload).
func NewInstanceConfigHandler(getConfig func() *config.Config, logger *slog.Logger) *InstanceConfigHandler {
	return &InstanceConfigHandler{
		getConfig: getConfig,
		logger:    logger.With("source", "api-instance-config"),
	}
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

	err := config.UpdateConfig(func(cfg *config.Config) error {
		index, _ := findInstanceByName(cfg, name)
		if index == -1 {
			return &notFoundError{name: name}
		}
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

	// Stop all nanobot processes after config update.
	// Known limitation: StopAllNanobots stops ALL nanobot.exe processes system-wide,
	// not just the deleted instance. Hot-reload will restart remaining instances within ~500ms.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	lifecycle.StopAllNanobots(ctx, 5*time.Second, h.logger)
	h.logger.Warn("StopAllNanobots stops all nanobot processes; remaining instances will restart via hot-reload within 500ms")

	h.logger.Info("Instance config deleted", "name", name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Instance %q deleted", name),
	})
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

	err = config.UpdateConfig(func(cfg *config.Config) error {
		sourceIndex, sourceIC := findInstanceByName(cfg, sourceName)
		if sourceIndex == -1 {
			return &notFoundError{name: sourceName}
		}

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(clonedInstance))
}

// writeValidationError is a method variant that delegates to the package-level function.
func (h *InstanceConfigHandler) writeValidationError(w http.ResponseWriter, message string, details []validationErrorDetail) {
	writeValidationError(w, message, details)
}
