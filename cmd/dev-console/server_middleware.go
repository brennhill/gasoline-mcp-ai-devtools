// server_middleware.go — HTTP middleware and security helpers.
// Contains CORS middleware, origin/host validation.
package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dev-console/dev-console/internal/state"
)

type extensionTrustConfig struct {
	ChromeID  string `json:"chrome_id,omitempty"`
	FirefoxID string `json:"firefox_id,omitempty"`
}

var (
	extensionTrustMu     sync.Mutex
	extensionTrustLoaded bool
	extensionTrust       extensionTrustConfig
)

const trustedExtensionFileName = "trusted-extension-ids.json"

func extensionTrustPath() (string, error) {
	return state.InRoot("settings", trustedExtensionFileName)
}

func loadExtensionTrustLocked() {
	if extensionTrustLoaded {
		return
	}
	extensionTrustLoaded = true

	path, err := extensionTrustPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &extensionTrust)
}

func saveExtensionTrustLocked() {
	path, err := extensionTrustPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}

	data, err := json.MarshalIndent(extensionTrust, "", "  ")
	if err != nil {
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

func isValidExtensionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// resetExtensionTrustCacheForTests resets the in-memory trust cache.
// Used by tests that assert first-seen extension pairing behavior.
func resetExtensionTrustCacheForTests() {
	extensionTrustMu.Lock()
	defer extensionTrustMu.Unlock()
	extensionTrustLoaded = false
	extensionTrust = extensionTrustConfig{}
}

// isExtensionOrigin checks if origin matches a browser extension scheme and,
// when an expected ID env var is set, verifies the full origin matches.
//
// Security model:
// - If GASOLINE_EXTENSION_ID / GASOLINE_FIREFOX_EXTENSION_ID is set, only that ID is allowed.
// - Otherwise, first-seen extension ID is paired (TOFU) and persisted in state.
// - After pairing, only the stored ID is allowed for that browser family.
func isExtensionOrigin(origin string) (matched bool, allowed bool) {
	type extCheck struct {
		prefix   string
		envVar   string
		isChrome bool
	}
	checks := []extCheck{
		{"chrome-extension://", "GASOLINE_EXTENSION_ID", true},
		{"moz-extension://", "GASOLINE_FIREFOX_EXTENSION_ID", false},
	}
	for _, c := range checks {
		if strings.HasPrefix(origin, c.prefix) {
			id := strings.TrimPrefix(origin, c.prefix)
			if !isValidExtensionID(id) {
				return true, false
			}

			expectedID := strings.TrimSpace(os.Getenv(c.envVar))
			if expectedID != "" {
				return true, id == expectedID
			}

			allowed := checkOrPairExtensionID(id, c.isChrome)
			return true, allowed
		}
	}
	return false, false
}

// checkOrPairExtensionID checks if the extension ID is trusted, or pairs it on first-seen (TOFU).
// Uses defer for panic safety.
func checkOrPairExtensionID(id string, isChrome bool) bool {
	extensionTrustMu.Lock()
	defer extensionTrustMu.Unlock()
	loadExtensionTrustLocked()

	var trustedID string
	if isChrome {
		trustedID = extensionTrust.ChromeID
	} else {
		trustedID = extensionTrust.FirefoxID
	}

	if trustedID == "" {
		if isChrome {
			extensionTrust.ChromeID = id
		} else {
			extensionTrust.FirefoxID = id
		}
		saveExtensionTrustLocked()
		return true
	}

	return id == trustedID
}

// isAllowedOrigin checks if an Origin header value is from localhost or a browser extension.
// Returns true for empty origin (CLI/curl), localhost variants, and browser extension origins.
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	if matched, allowed := isExtensionOrigin(origin); matched {
		return allowed
	}
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
// with exactly "gasoline-extension" or "gasoline-extension/{version}".
// Rejects with 403 if missing or invalid. This ensures only the Gasoline
// browser extension can call extension-facing endpoints.
func extensionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := r.Header.Get("X-Gasoline-Client")
		// Require exact match or versioned format (gasoline-extension/6.0.3)
		if client != "gasoline-extension" && !strings.HasPrefix(client, "gasoline-extension/") {
			http.Error(w, `{"error":"forbidden: missing or invalid X-Gasoline-Client header"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// Note: jsonResponse is defined in handler.go
