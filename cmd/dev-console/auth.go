package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// AuthMiddleware returns a middleware function that checks the X-Gasoline-Key header.
// If expectedKey is empty, no authentication is required (pass-through).
// If expectedKey is set, requests must include the correct key or receive 401.
// Key comparison uses crypto/subtle.ConstantTimeCompare to prevent timing attacks.
func AuthMiddleware(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no key is configured, auth is disabled
			if expectedKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check the X-Gasoline-Key header
			providedKey := r.Header.Get("X-Gasoline-Key")

			// Use constant-time comparison to prevent timing attacks
			expectedBytes := []byte(expectedKey)
			providedBytes := []byte(providedKey)

			if subtle.ConstantTimeCompare(expectedBytes, providedBytes) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
