package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// validateBearerToken extracts and validates the Bearer token from the request.
// It returns an error message if validation fails, or nil on success.
//
// API-02: Bearer token in Authorization header is validated
// API-05: Use constant time comparison (subtle.ConstantTimeCompare)
func validateBearerToken(r *http.Request, expectedToken string) error {
	authHeader := r.Header.Get("Authorization")

	// Check if Authorization header exists
	if authHeader == "" {
		return &authError{
			code:    "unauthorized",
			message: "Missing Authorization header",
		}
	}

	// Extract token using strings.TrimPrefix
	// API-05: Use strings.TrimPrefix to extract token from "Bearer <token>"
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// If TrimPrefix returns same string, it means "Bearer " prefix was not found
	if token == authHeader {
		return &authError{
			code:    "unauthorized",
			message: "Invalid Authorization header format",
		}
	}

	// Check if token is empty
	if token == "" {
		return &authError{
			code:    "unauthorized",
			message: "Empty Bearer token",
		}
	}

	// Use subtle.ConstantTimeCompare for constant time comparison to prevent timing attacks
	// API-05: Constant time comparison prevents timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return &authError{
			code:    "unauthorized",
			message: "Invalid Bearer token",
		}
	}

	return nil
}

// AuthMiddleware returns a middleware that validates Bearer token authentication.
// It implements RFC 6750 Bearer Token Usage standard.
//
// API-02: Bearer token validation
// API-05: Security requirements (constant time comparison, JSON error format)
//
// getToken is called on every request to get the current expected token.
// This enables hot-reloading the bearer token without restarting the API server.
func AuthMiddleware(getToken func() string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate Bearer token — read current token dynamically
			err := validateBearerToken(r, getToken())
			if err != nil {
				// Extract error details
				var errorCode, errorMsg string
				if authErr, ok := err.(*authError); ok {
					errorCode = authErr.code
					errorMsg = authErr.message
				} else {
					errorCode = "unauthorized"
					errorMsg = err.Error()
				}

				// Log authentication failure
				logger.Warn("Authentication failed",
					"error", errorCode,
					"message", errorMsg,
					"path", r.URL.Path,
					"method", r.Method,
				)

				// API-05: Return 401 with JSON error
				writeJSONError(w, http.StatusUnauthorized, errorCode, errorMsg)
				return
			}

			// Token is valid, proceed to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a JSON error response following RFC 7807 Problem Details format.
//
// API-05: Return JSON error format
// Format: {"error": "error_code", "message": "Human readable message"}
//
// Usage:
//
//	writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Missing Authorization header")
func writeJSONError(w http.ResponseWriter, status int, errorCode, message string) {
	// Set Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")

	// Set HTTP status code
	w.WriteHeader(status)

	// Encode JSON response
	// RFC 7807 Problem Details format: {"error": "error_code", "message": "Human readable message"}
	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	// Encode and write JSON response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If JSON encoding fails, write a plain text error as fallback
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

// authError represents an authentication error with code and message
type authError struct {
	code    string
	message string
}

// Error implements the error interface
func (e *authError) Error() string {
	return e.code
}

// Message returns the detailed error message
func (e *authError) Message() string {
	return e.message
}
