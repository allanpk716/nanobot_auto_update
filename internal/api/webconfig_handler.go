package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
)

// WebConfigResponse is the JSON response for the web-config endpoint.
type WebConfigResponse struct {
	AuthToken string `json:"auth_token"`
}

// NewWebConfigHandler creates a handler that returns web UI configuration.
// The returned handler should be wrapped with localhostOnly for security.
func NewWebConfigHandler(bearerToken string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := WebConfigResponse{
			AuthToken: bearerToken,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error("Failed to encode web-config response", "error", err)
		}
	}
}

// localhostOnly wraps an http.HandlerFunc to reject non-localhost requests.
// Uses net.SplitHostPort to extract host from RemoteAddr (format "host:port").
// Supports both IPv4 (127.0.0.1) and IPv6 (::1) loopback addresses.
func localhostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeJSONError(w, http.StatusForbidden, "forbidden", "Access denied")
			return
		}
		if host != "127.0.0.1" && host != "::1" {
			writeJSONError(w, http.StatusForbidden, "forbidden", "Access denied: localhost only")
			return
		}
		next.ServeHTTP(w, r)
	}
}
