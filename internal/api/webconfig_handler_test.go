package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebConfig_LocalhostToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	token := "test-bearer-token-1234567890abcdef"
	handler := localhostOnly(NewWebConfigHandler(token, logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response WebConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, token, response.AuthToken)
}

func TestWebConfig_RemoteForbidden(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := localhostOnly(NewWebConfigHandler("some-token", logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "forbidden", response["error"])
}

func TestWebConfig_IPv6Localhost(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	token := "test-token-abcdef1234567890"
	handler := localhostOnly(NewWebConfigHandler(token, logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "[::1]:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response WebConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, token, response.AuthToken)
}

func TestWebConfig_InvalidRemoteAddr(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := localhostOnly(NewWebConfigHandler("some-token", logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "invalid"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWebConfig_NoAuthRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	token := "test-token-no-auth-required"
	handler := localhostOnly(NewWebConfigHandler(token, logger))

	// Request WITHOUT Authorization header
	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 200 (not 401) -- no auth required, only localhost check
	assert.Equal(t, http.StatusOK, rec.Code)

	var response WebConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, token, response.AuthToken)
}

func TestWebConfig_EmptyToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := localhostOnly(NewWebConfigHandler("", logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response WebConfigResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	assert.Equal(t, "", response.AuthToken)
}

func TestWebConfig_JSONContentType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := localhostOnly(NewWebConfigHandler("token", logger))

	req := httptest.NewRequest("GET", "/api/v1/web-config", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}
