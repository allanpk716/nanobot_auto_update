package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Shared test helpers ---

func boolPtr(b bool) *bool {
	return &b
}

// setupInstanceConfigTest creates a handler with an injected config reader (no file system, no viper, no hot-reload).
// Use this for read-only tests (List, Get, Auth).
func setupInstanceConfigTest(t *testing.T) (*InstanceConfigHandler, string, *config.Config) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{
				Name:           "test-existing",
				Port:           18790,
				StartCommand:   "nanobot gateway",
				StartupTimeout: 30 * time.Second,
				AutoStart:      boolPtr(true),
			},
		},
	}

	handler := NewInstanceConfigHandler(func() *config.Config {
		return cfg
	}, logger)

	token := "test-token-123456789012345678901"

	return handler, token, cfg
}

// setupIntegrationTest creates a handler backed by a real config file (for mutation tests).
// Each test gets its own temp directory, config file, and hot-reload state.
func setupIntegrationTest(t *testing.T) (*InstanceConfigHandler, string) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "test-existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
    auto_start: true
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Start watch to initialize globalHotReload (needed for UpdateConfig -> GetCurrentConfig)
	config.WatchConfig(cfg, logger, &config.HotReloadCallbacks{})

	handler := NewInstanceConfigHandler(config.GetCurrentConfig, logger)
	token := "test-token-123456789012345678901"

	t.Cleanup(func() {
		config.StopWatch()
	})

	return handler, token
}

// authenticatedRequest creates an httptest.Request with the Bearer token set.
func authenticatedRequest(method, path, token string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// withAuth wraps a handler with auth middleware for testing.
func withAuth(handler http.HandlerFunc, token string) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return AuthMiddleware(func() string { return token }, logger)(handler)
}

// --- Read-only handler tests (injected config, no file system) ---

func TestHandleList_ReturnsAllInstances(t *testing.T) {
	handler, token, _ := setupInstanceConfigTest(t)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instance-configs", withAuth(handler.HandleList, token))

	req := authenticatedRequest("GET", "/api/v1/instance-configs", token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string][]instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	instances := response["instances"]
	require.Len(t, instances, 1)
	assert.Equal(t, "test-existing", instances[0].Name)
	assert.Equal(t, uint32(18790), instances[0].Port)
	assert.Equal(t, "nanobot gateway", instances[0].StartCommand)
	assert.Equal(t, uint32(30), instances[0].StartupTimeout)
	assert.NotNil(t, instances[0].AutoStart)
	assert.True(t, *instances[0].AutoStart)
}

func TestHandleGet_ExistingInstance(t *testing.T) {
	handler, token, _ := setupInstanceConfigTest(t)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instance-configs/{name}", withAuth(handler.HandleGet, token))

	req := authenticatedRequest("GET", "/api/v1/instance-configs/test-existing", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	assert.Equal(t, "test-existing", response.Name)
	assert.Equal(t, uint32(18790), response.Port)
	assert.Equal(t, "nanobot gateway", response.StartCommand)
	assert.Equal(t, uint32(30), response.StartupTimeout)
}

func TestHandleGet_NonExistentInstance(t *testing.T) {
	handler, token, _ := setupInstanceConfigTest(t)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instance-configs/{name}", withAuth(handler.HandleGet, token))

	req := authenticatedRequest("GET", "/api/v1/instance-configs/unknown", token, nil)
	req.SetPathValue("name", "unknown")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "not_found", response["error"])
}

// --- Auth tests ---

func TestAuth_RequiredOnAllEndpoints(t *testing.T) {
	handler, _, _ := setupInstanceConfigTest(t)

	tests := []struct {
		name       string
		method     string
		path       string
		pathName   string // value for {name} path parameter, if applicable
	}{
		{"List without auth", "GET", "/api/v1/instance-configs", ""},
		{"Create without auth", "POST", "/api/v1/instance-configs", ""},
		{"Get without auth", "GET", "/api/v1/instance-configs/test-existing", "test-existing"},
		{"Update without auth", "PUT", "/api/v1/instance-configs/test-existing", "test-existing"},
		{"Delete without auth", "DELETE", "/api/v1/instance-configs/test-existing", "test-existing"},
		{"Copy without auth", "POST", "/api/v1/instance-configs/test-existing/copy", "test-existing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("GET /api/v1/instance-configs", withAuth(handler.HandleList, ""))
			mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, ""))
			mux.Handle("GET /api/v1/instance-configs/{name}", withAuth(handler.HandleGet, ""))
			mux.Handle("PUT /api/v1/instance-configs/{name}", withAuth(handler.HandleUpdate, ""))
			mux.Handle("DELETE /api/v1/instance-configs/{name}", withAuth(handler.HandleDelete, ""))
			mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, ""))

			req := httptest.NewRequest(tt.method, tt.path, nil)
			// No Authorization header
			if tt.pathName != "" {
				req.SetPathValue("name", tt.pathName)
			}

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code, "Expected 401 for %s %s", tt.method, tt.path)

			var response map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
			assert.Equal(t, "unauthorized", response["error"])
		})
	}
}

func TestAuth_WrongToken(t *testing.T) {
	handler, _, _ := setupInstanceConfigTest(t)

	// Auth middleware expects "correct-token", but we send "wrong-token"
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/instance-configs", withAuth(handler.HandleList, "correct-token-123456789012345678"))

	req := httptest.NewRequest("GET", "/api/v1/instance-configs", nil)
	req.Header.Set("Authorization", "Bearer wrong-token-00000000000000000000000")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Verify with valid token that we get a non-401 response
	handler2, token2, _ := setupInstanceConfigTest(t)
	mux2 := http.NewServeMux()
	mux2.Handle("GET /api/v1/instance-configs", withAuth(handler2.HandleList, token2))

	req2 := authenticatedRequest("GET", "/api/v1/instance-configs", token2, nil)
	rec2 := httptest.NewRecorder()
	mux2.ServeHTTP(rec2, req2)

	assert.NotEqual(t, http.StatusUnauthorized, rec2.Code, "Valid token should not return 401")
}

// --- Mutation handler tests (integration with real config file) ---

func TestHandleCreate_ValidConfig(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	body := `{"name":"new-instance","port":18791,"start_command":"nanobot worker","startup_timeout":45}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "new-instance", response.Name)
	assert.Equal(t, uint32(18791), response.Port)
	assert.Equal(t, "nanobot worker", response.StartCommand)
	assert.Equal(t, uint32(45), response.StartupTimeout)
}

func TestHandleCreate_DuplicateName(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	// "test-existing" is already in the config
	body := `{"name":"test-existing","port":18791,"start_command":"nanobot worker","startup_timeout":30}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response validationErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "validation_error", response.Error)

	// Should contain a name-related error
	found := false
	for _, e := range response.Errors {
		if e.Field == "name" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected validation error on 'name' field")
}

func TestHandleCreate_DuplicatePort(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	// Port 18790 is already used by test-existing
	body := `{"name":"another-instance","port":18790,"start_command":"nanobot worker","startup_timeout":30}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response validationErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "validation_error", response.Error)

	// Should contain a port-related error
	found := false
	for _, e := range response.Errors {
		if e.Field == "port" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected validation error on 'port' field")
}

func TestHandleCreate_MissingRequiredFields(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	// Missing name, port, start_command
	body := `{}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response validationErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "validation_error", response.Error)
	assert.NotEmpty(t, response.Errors, "Expected multiple validation errors")
}

func TestHandleCreate_InvalidJSON(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader("{invalid json"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "bad_request", response["error"])
}

func TestHandleUpdate_ValidUpdate(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/instance-configs/{name}", withAuth(handler.HandleUpdate, token))

	body := `{"name":"test-existing","port":18791,"start_command":"nanobot gateway --new","startup_timeout":45}`
	req := authenticatedRequest("PUT", "/api/v1/instance-configs/test-existing", token, strings.NewReader(body))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, uint32(18791), response.Port)
	assert.Equal(t, "nanobot gateway --new", response.StartCommand)
	assert.Equal(t, uint32(45), response.StartupTimeout)
}

func TestHandleUpdate_NonExistent(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/instance-configs/{name}", withAuth(handler.HandleUpdate, token))

	body := `{"name":"nonexistent","port":18791,"start_command":"nanobot","startup_timeout":30}`
	req := authenticatedRequest("PUT", "/api/v1/instance-configs/nonexistent", token, strings.NewReader(body))
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleUpdate_PortConflict(t *testing.T) {
	// Use a config with two instances already present, so port conflict can be tested
	// without depending on hot-reload timing between Create and Update.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	initialYAML := `api:
  port: 9999
  bearer_token: "test-token-123456789012345678901"
instances:
  - name: "test-existing"
    port: 18790
    start_command: "nanobot gateway"
    startup_timeout: 30s
    auto_start: true
  - name: "second-instance"
    port: 18791
    start_command: "nanobot worker"
    startup_timeout: 30s
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialYAML), 0644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config.WatchConfig(cfg, logger, &config.HotReloadCallbacks{})
	t.Cleanup(func() { config.StopWatch() })

	handler := NewInstanceConfigHandler(config.GetCurrentConfig, logger)
	token := "test-token-123456789012345678901"

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/instance-configs/{name}", withAuth(handler.HandleUpdate, token))

	// Try to update test-existing to use port 18791 (conflict with second-instance)
	updateBody := `{"name":"test-existing","port":18791,"start_command":"nanobot gateway","startup_timeout":30}`
	req := authenticatedRequest("PUT", "/api/v1/instance-configs/test-existing", token, strings.NewReader(updateBody))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response validationErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "validation_error", response.Error)
}

func TestHandleUpdate_NameChange(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("PUT /api/v1/instance-configs/{name}", withAuth(handler.HandleUpdate, token))

	// Try to change name from test-existing to new-name
	body := `{"name":"new-name","port":18790,"start_command":"nanobot gateway","startup_timeout":30}`
	req := authenticatedRequest("PUT", "/api/v1/instance-configs/test-existing", token, strings.NewReader(body))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var response validationErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "validation_error", response.Error)

	// Should contain name-related error
	found := false
	for _, e := range response.Errors {
		if e.Field == "name" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected validation error on 'name' field for name change attempt")
}

func TestHandleDelete_ExistingInstance(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/instance-configs/{name}", withAuth(handler.HandleDelete, token))

	req := authenticatedRequest("DELETE", "/api/v1/instance-configs/test-existing", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Contains(t, response["message"], "test-existing")
	assert.Contains(t, response["message"], "deleted")
}

func TestHandleDelete_NonExistent(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/instance-configs/{name}", withAuth(handler.HandleDelete, token))

	req := authenticatedRequest("DELETE", "/api/v1/instance-configs/unknown", token, nil)
	req.SetPathValue("name", "unknown")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleCopy_DefaultNamePort(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, token))

	req := authenticatedRequest("POST", "/api/v1/instance-configs/test-existing/copy", token, strings.NewReader("{}"))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))

	// Default: name = source + "-copy", port = source + 1
	assert.Equal(t, "test-existing-copy", response.Name)
	assert.Equal(t, uint32(18791), response.Port)
	// Should copy start_command and startup_timeout from source
	assert.Equal(t, "nanobot gateway", response.StartCommand)
	assert.Equal(t, uint32(30), response.StartupTimeout)
}

func TestHandleCopy_CustomNameAndPort(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, token))

	body := `{"name":"my-custom-copy","port":18800}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs/test-existing/copy", token, strings.NewReader(body))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "my-custom-copy", response.Name)
	assert.Equal(t, uint32(18800), response.Port)
}

func TestHandleCopy_NonExistentSource(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, token))

	req := authenticatedRequest("POST", "/api/v1/instance-configs/unknown/copy", token, strings.NewReader("{}"))
	req.SetPathValue("name", "unknown")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleCopy_EmptyBody(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, token))

	// Empty body -- should use defaults
	req := authenticatedRequest("POST", "/api/v1/instance-configs/test-existing/copy", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "test-existing-copy", response.Name)
	assert.Equal(t, uint32(18791), response.Port)
}

// --- Callback tests (Phase 52: nanobot config lifecycle integration) ---

func TestHandleCreate_InvokesOnCreateCallback(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	var callbackCalled bool
	var callbackName string
	var callbackPort uint32
	var callbackStartCommand string

	handler.SetOnCreateInstance(func(name string, port uint32, startCommand string) error {
		callbackCalled = true
		callbackName = name
		callbackPort = port
		callbackStartCommand = startCommand
		return nil
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	body := `{"name":"new-instance","port":18791,"start_command":"nanobot gateway","startup_timeout":30}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, callbackCalled, "onCreateInstance callback should have been called")
	assert.Equal(t, "new-instance", callbackName)
	assert.Equal(t, uint32(18791), callbackPort)
	assert.Equal(t, "nanobot gateway", callbackStartCommand)
}

func TestHandleCreate_CallbackFailureNonBlocking(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	handler.SetOnCreateInstance(func(name string, port uint32, startCommand string) error {
		return fmt.Errorf("simulated nanobot config creation failure")
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs", withAuth(handler.HandleCreate, token))

	body := `{"name":"new-instance","port":18791,"start_command":"nanobot gateway","startup_timeout":30}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs", token, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Instance should still be created (201) even though callback failed
	assert.Equal(t, http.StatusCreated, rec.Code)

	var response instanceConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "new-instance", response.Name)
}

func TestHandleCopy_InvokesOnCopyCallback(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	var callbackCalled bool
	var callbackSourceName string
	var callbackTargetName string
	var callbackTargetPort uint32

	handler.SetOnCopyInstance(func(sourceName string, sourceStartCommand string, targetName string, targetPort uint32, targetStartCommand string) error {
		callbackCalled = true
		callbackSourceName = sourceName
		callbackTargetName = targetName
		callbackTargetPort = targetPort
		return nil
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instance-configs/{name}/copy", withAuth(handler.HandleCopy, token))

	body := `{"name":"my-copy","port":18800}`
	req := authenticatedRequest("POST", "/api/v1/instance-configs/test-existing/copy", token, strings.NewReader(body))
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.True(t, callbackCalled, "onCopyInstance callback should have been called")
	assert.Equal(t, "test-existing", callbackSourceName)
	assert.Equal(t, "my-copy", callbackTargetName)
	assert.Equal(t, uint32(18800), callbackTargetPort)
}

func TestHandleDelete_InvokesOnDeleteCallback(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	var callbackCalled bool
	var callbackName string
	var callbackStartCommand string

	handler.SetOnDeleteInstance(func(name string, startCommand string) error {
		callbackCalled = true
		callbackName = name
		callbackStartCommand = startCommand
		return nil
	})

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/instance-configs/{name}", withAuth(handler.HandleDelete, token))

	req := authenticatedRequest("DELETE", "/api/v1/instance-configs/test-existing", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, callbackCalled, "onDeleteInstance callback should have been called")
	assert.Equal(t, "test-existing", callbackName)
	assert.Equal(t, "nanobot gateway", callbackStartCommand)
}

func TestHandleDelete_CallbackFailureNonBlocking(t *testing.T) {
	handler, token := setupIntegrationTest(t)

	handler.SetOnDeleteInstance(func(name string, startCommand string) error {
		return fmt.Errorf("simulated nanobot config cleanup failure")
	})

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/instance-configs/{name}", withAuth(handler.HandleDelete, token))

	req := authenticatedRequest("DELETE", "/api/v1/instance-configs/test-existing", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Instance should still be deleted (200) even though callback failed
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Contains(t, response["message"], "deleted")
}
