package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/instance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Lifecycle test helpers ---

// lifecycleMockNotifier satisfies instance.Notifier for testing.
type lifecycleMockNotifier struct{}

func (m *lifecycleMockNotifier) IsEnabled() bool                   { return false }
func (m *lifecycleMockNotifier) Notify(title, message string) error { return nil }

// setupLifecycleTest creates an InstanceLifecycleHandler with a real InstanceManager
// and a configured test instance. Returns handler, instanceManager, and auth token.
func setupLifecycleTest(t *testing.T) (*InstanceLifecycleHandler, *instance.InstanceManager, string) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := &config.Config{
		Instances: []config.InstanceConfig{
			{
				Name:           "test-existing",
				Port:           18790,
				// Use cmd /c to run ping (stays alive ~30 seconds on Windows).
				// Include " --port 18790" in a REM comment to satisfy containsPortFlag()
				// and prevent auto-append of --port flag to the actual command.
				StartCommand:   `cmd /c "ping -n 30 127.0.0.1 & rem --port 18790"`,
				StartupTimeout: 5 * time.Second,
				AutoStart:      boolPtr(true),
			},
		},
	}

	im := instance.NewInstanceManager(cfg, logger, &lifecycleMockNotifier{})
	handler := NewInstanceLifecycleHandler(im, logger)
	token := "test-token-123456789012345678901"

	return handler, im, token
}

// --- Success path tests (addresses review MEDIUM-4) ---

func TestHandleStart_Success(t *testing.T) {
	handler, im, token := setupLifecycleTest(t)

	// Cleanup: stop any started instance
	t.Cleanup(func() {
		inst, _ := im.GetLifecycle("test-existing")
		if inst != nil && inst.IsRunning() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			inst.StopForUpdate(ctx)
		}
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/start", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Contains(t, response["message"], "started")
	assert.Equal(t, true, response["running"])
}

func TestHandleStop_Success(t *testing.T) {
	handler, im, token := setupLifecycleTest(t)

	// First, start the instance so we can stop it
	inst, err := im.GetLifecycle("test-existing")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, inst.StartAfterUpdate(ctx))

	// Ensure instance is running before proceeding
	require.True(t, inst.IsRunning(), "Instance should be running after start")

	// Cleanup: stop instance if still running after test
	t.Cleanup(func() {
		if inst.IsRunning() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			inst.StopForUpdate(cleanupCtx)
		}
	})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler.HandleStop, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/stop", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Contains(t, response["message"], "stopped")
	assert.Equal(t, false, response["running"])
}

// --- AlreadyRunning test (addresses review HIGH-3) ---

func TestHandleStart_AlreadyRunning(t *testing.T) {
	handler, im, token := setupLifecycleTest(t)

	// Inject running state: set PID to current test process (always exists)
	inst, err := im.GetLifecycle("test-existing")
	require.NoError(t, err)
	inst.SetPIDForTest(int32(os.Getpid()))

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/start", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "conflict", response["error"])
	assert.Contains(t, response["message"], "already running")
}

// --- Error path tests ---

func TestHandleStart_NotFound(t *testing.T) {
	handler, _, token := setupLifecycleTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))

	req := authenticatedRequest("POST", "/api/v1/instances/nonexistent/start", token, nil)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "not_found", response["error"])
}

func TestHandleStart_EmptyName(t *testing.T) {
	handler, _, token := setupLifecycleTest(t)

	// Call handler directly (not via ServeMux) because ServeMux redirects
	// double-slash paths instead of routing to the handler.
	req := authenticatedRequest("POST", "/api/v1/instances//start", token, nil)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()
	handler.HandleStart(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "bad_request", response["error"])
}

func TestHandleStop_NotFound(t *testing.T) {
	handler, _, token := setupLifecycleTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler.HandleStop, token))

	req := authenticatedRequest("POST", "/api/v1/instances/nonexistent/stop", token, nil)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "not_found", response["error"])
}

func TestHandleStop_EmptyName(t *testing.T) {
	handler, _, token := setupLifecycleTest(t)

	// Call handler directly (not via ServeMux) because ServeMux redirects
	// double-slash paths instead of routing to the handler.
	req := authenticatedRequest("POST", "/api/v1/instances//stop", token, nil)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()
	handler.HandleStop(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "bad_request", response["error"])
}

func TestHandleStop_AlreadyStopped(t *testing.T) {
	handler, _, token := setupLifecycleTest(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler.HandleStop, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/stop", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "conflict", response["error"])
	assert.Contains(t, response["message"], "not running")
}

// --- Auth tests (LC-03) ---

func TestLifecycleAuth_RequiredOnAllEndpoints(t *testing.T) {
	handler, _, _ := setupLifecycleTest(t)

	tests := []struct {
		name     string
		method   string
		path     string
		pathName string
	}{
		{"Start without auth", "POST", "/api/v1/instances/test-existing/start", "test-existing"},
		{"Stop without auth", "POST", "/api/v1/instances/test-existing/stop", "test-existing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, ""))
			mux.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler.HandleStop, ""))

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

func TestLifecycleAuth_WrongToken(t *testing.T) {
	handler, _, _ := setupLifecycleTest(t)

	// Auth middleware expects "correct-token", but we send "wrong-token"
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, "correct-token-123456789012345678"))

	req := httptest.NewRequest("POST", "/api/v1/instances/test-existing/start", nil)
	req.Header.Set("Authorization", "Bearer wrong-token-00000000000000000000000")
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Verify with valid token that we get a non-401 response
	handler2, _, token2 := setupLifecycleTest(t)
	mux2 := http.NewServeMux()
	mux2.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler2.HandleStop, token2))

	req2 := authenticatedRequest("POST", "/api/v1/instances/test-existing/stop", token2, nil)
	req2.SetPathValue("name", "test-existing")
	rec2 := httptest.NewRecorder()
	mux2.ServeHTTP(rec2, req2)

	// Should be 409 (not running), not 401 (unauthorized)
	assert.NotEqual(t, http.StatusUnauthorized, rec2.Code, "Valid token should not return 401")
}

// --- Concurrency tests (addresses review HIGH-1) ---

func TestHandleStart_UpdateInProgress(t *testing.T) {
	handler, im, token := setupLifecycleTest(t)

	// Lock the update to simulate an update in progress
	require.True(t, im.TryLockUpdate(), "Should acquire update lock")
	defer im.UnlockUpdate()

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/start", withAuth(handler.HandleStart, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/start", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "conflict", response["error"])
	assert.Contains(t, response["message"], "update is already in progress")
}

func TestHandleStop_UpdateInProgress(t *testing.T) {
	handler, im, token := setupLifecycleTest(t)

	// Lock the update to simulate an update in progress
	require.True(t, im.TryLockUpdate(), "Should acquire update lock")
	defer im.UnlockUpdate()

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/instances/{name}/stop", withAuth(handler.HandleStop, token))

	req := authenticatedRequest("POST", "/api/v1/instances/test-existing/stop", token, nil)
	req.SetPathValue("name", "test-existing")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var response map[string]interface{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "conflict", response["error"])
	assert.Contains(t, response["message"], "update is already in progress")
}
