// server_middleware.go — HTTP middleware and security helpers.
// Contains CORS middleware, origin/host validation.
package main

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// isAllowedOrigin checks if an Origin header value is from localhost or a browser extension.
// Returns true for empty origin (CLI/curl), localhost variants, and browser extension origins.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}

	// Browser extension origins - validate specific ID if configured
	if strings.HasPrefix(origin, "chrome-extension://") {
		expectedID := os.Getenv("GASOLINE_EXTENSION_ID")
		if expectedID != "" {
			return origin == "chrome-extension://"+expectedID
		}
		return true // Permissive mode when not configured
	}
	if strings.HasPrefix(origin, "moz-extension://") {
		expectedID := os.Getenv("GASOLINE_FIREFOX_EXTENSION_ID")
		if expectedID != "" {
			return origin == "moz-extension://"+expectedID
		}
		return true // Permissive mode when not configured
	}

	// Parse the origin URL to extract the hostname
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	hostname := u.Hostname()
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// isAllowedHost checks if the Host header is a localhost variant.
// Returns true for empty host (HTTP/1.0 clients), localhost, 127.0.0.1, and [::1]
// with any port. This prevents DNS rebinding attacks where attacker.com resolves
// to 127.0.0.1 — the browser sends Host: attacker.com which we reject.
func isAllowedHost(host string) bool {
	if host == "" {
		return true
	}

	// Strip port if present. net.SplitHostPort fails for hosts without port,
	// so we check both forms.
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	// Strip IPv6 brackets (e.g., "[::1]" → "::1") for bare IPv6 without port
	hostname = strings.TrimPrefix(hostname, "[")
	hostname = strings.TrimSuffix(hostname, "]")

	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}

// CORS middleware with Host and Origin validation for DNS rebinding protection
// (MCP spec §base/transports H-2/H-3).
//
// Security: Two layers of protection against DNS rebinding:
//  1. Host header validation — rejects requests where Host is not a localhost variant.
//  2. Origin validation — rejects requests from non-local, non-extension origins.
//  3. CORS origin echo — returns the specific allowed origin, never wildcard "*".
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Layer 1: Validate Host header (DNS rebinding protection)
		if !isAllowedHost(r.Host) {
			http.Error(w, "Invalid Host header", http.StatusForbidden)
			return
		}

		// Layer 2: Validate Origin header — if present and invalid, reject with 403
		origin := r.Header.Get("Origin")
		if origin != "" && !isAllowedOrigin(origin) {
			http.Error(w, `{"error":"forbidden: invalid origin"}`, http.StatusForbidden)
			return
		}

		// Layer 3: Echo back the specific allowed origin (never wildcard "*")
		// Only set ACAO when an Origin header is present and valid.
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Gasoline-Key, X-Gasoline-Client, X-Gasoline-Extension-Version")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// extensionOnly wraps a handler to require the X-Gasoline-Client header
// with a value starting with "gasoline-extension". Rejects with 403 if missing
// or invalid. This ensures only the Gasoline browser extension can call
// extension-facing endpoints.
func extensionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := r.Header.Get("X-Gasoline-Client")
		if !strings.HasPrefix(client, "gasoline-extension") {
			http.Error(w, `{"error":"forbidden: missing or invalid X-Gasoline-Client header"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// Note: jsonResponse is defined in handler.go
