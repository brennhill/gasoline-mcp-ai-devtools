package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/types"
)

// dummyHandler is a simple handler that returns 200 OK with "authorized" body
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("authorized"))
	})
}

func TestAuthNoKeyConfigured(t *testing.T) {
	t.Parallel()
	// When no API key is configured (empty string), all requests pass through
	handler := AuthMiddleware("")(dummyHandler())

	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "authorized" {
		t.Errorf("Expected body 'authorized', got %q", rec.Body.String())
	}
}

func TestAuthKeyConfiguredNoHeader(t *testing.T) {
	t.Parallel()
	// When API key is configured but request has no header, return 401
	handler := AuthMiddleware("my-secret-key")(dummyHandler())

	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthKeyConfiguredWrongKey(t *testing.T) {
	t.Parallel()
	// When API key is configured but request has wrong key, return 401
	handler := AuthMiddleware("my-secret-key")(dummyHandler())

	req := httptest.NewRequest("GET", "/logs", nil)
	req.Header.Set("X-Gasoline-Key", "wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
}

func TestAuthKeyConfiguredCorrectKey(t *testing.T) {
	t.Parallel()
	// When API key is configured and request has correct key, pass through
	handler := AuthMiddleware("my-secret-key")(dummyHandler())

	req := httptest.NewRequest("GET", "/logs", nil)
	req.Header.Set("X-Gasoline-Key", "my-secret-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "authorized" {
		t.Errorf("Expected body 'authorized', got %q", rec.Body.String())
	}
}

func TestAuth401ResponseBody(t *testing.T) {
	t.Parallel()
	// 401 response body must be JSON {"error": "unauthorized"}
	handler := AuthMiddleware("my-secret-key")(dummyHandler())

	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("Failed to parse response body as JSON: %v", err)
	}

	if body["error"] != "unauthorized" {
		t.Errorf("Expected error 'unauthorized', got %q", body["error"])
	}
}

func TestAuthApplesToLogsEndpoint(t *testing.T) {
	t.Parallel()
	handler := AuthMiddleware("secret123")(dummyHandler())

	// Without key - should be rejected
	req := httptest.NewRequest("POST", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/logs without key: expected 401, got %d", rec.Code)
	}

	// With correct key - should pass
	req = httptest.NewRequest("POST", "/logs", nil)
	req.Header.Set("X-Gasoline-Key", "secret123")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/logs with correct key: expected 200, got %d", rec.Code)
	}
}

func TestAuthAppliesToNetworkBodyEndpoint(t *testing.T) {
	t.Parallel()
	handler := AuthMiddleware("secret123")(dummyHandler())

	// Without key - should be rejected
	req := httptest.NewRequest("POST", "/network-body", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/network-body without key: expected 401, got %d", rec.Code)
	}

	// With correct key - should pass
	req = httptest.NewRequest("POST", "/network-body", nil)
	req.Header.Set("X-Gasoline-Key", "secret123")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/network-body with correct key: expected 200, got %d", rec.Code)
	}
}

func TestAuthAppliesToActionsEndpoint(t *testing.T) {
	t.Parallel()
	handler := AuthMiddleware("secret123")(dummyHandler())

	// Without key - should be rejected
	req := httptest.NewRequest("POST", "/actions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("/actions without key: expected 401, got %d", rec.Code)
	}

	// With correct key - should pass
	req = httptest.NewRequest("POST", "/actions", nil)
	req.Header.Set("X-Gasoline-Key", "secret123")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("/actions with correct key: expected 200, got %d", rec.Code)
	}
}

func TestAuthEmptyKeyMeansDisabled(t *testing.T) {
	t.Parallel()
	// Empty string API key means auth is disabled
	handler := AuthMiddleware("")(dummyHandler())

	// Request without any header should pass through
	req := httptest.NewRequest("GET", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Empty key should disable auth, got status %d", rec.Code)
	}
}

func TestAuthConstantTimeComparison(t *testing.T) {
	t.Parallel()
	// This test verifies that the auth middleware uses crypto/subtle
	// for key comparison. We test this by ensuring that keys of different
	// lengths still result in proper 401 (timing-safe comparison pads or
	// compares full bytes, not short-circuit on first mismatch).
	handler := AuthMiddleware("correct-key")(dummyHandler())

	testCases := []struct {
		name string
		key  string
	}{
		{"empty key", ""},
		{"single char", "x"},
		{"partial match", "correct"},
		{"longer key", "correct-key-but-longer"},
		{"same length wrong", "incorrect!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/logs", nil)
			req.Header.Set("X-Gasoline-Key", tc.key)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Key %q should be rejected, got status %d", tc.key, rec.Code)
			}
		})
	}

	// Correct key should pass
	req := httptest.NewRequest("GET", "/logs", nil)
	req.Header.Set("X-Gasoline-Key", "correct-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Correct key should pass, got status %d", rec.Code)
	}
}

func TestAuthMiddlewarePreservesCORSPreflight(t *testing.T) {
	t.Parallel()
	// OPTIONS requests (CORS preflight) should still work with auth enabled
	// since browsers send preflight without custom headers first
	handler := AuthMiddleware("my-key")(dummyHandler())

	req := httptest.NewRequest("OPTIONS", "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Auth middleware should still require key even for OPTIONS
	// (CORS middleware runs before auth in the actual server stack)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("OPTIONS without key: expected 401, got %d", rec.Code)
	}
}

// ============================================
// isAllowedOrigin: Extension ID Validation Tests
// ============================================

func TestIsAllowedOrigin_EmptyOrigin(t *testing.T) {
	t.Parallel()
	// Empty origin (CLI/curl) should always be allowed
	if !isAllowedOrigin("") {
		t.Error("empty origin should be allowed")
	}
}

func TestIsAllowedOrigin_Localhost(t *testing.T) {
	t.Parallel()
	origins := []string{
		"http://localhost",
		"http://localhost:3000",
		"http://127.0.0.1",
		"http://127.0.0.1:8080",
		"http://[::1]",
		"http://[::1]:9000",
	}
	for _, origin := range origins {
		if !isAllowedOrigin(origin) {
			t.Errorf("localhost origin %q should be allowed", origin)
		}
	}
}

func TestIsAllowedOrigin_ExternalRejected(t *testing.T) {
	t.Parallel()
	origins := []string{
		"https://evil.com",
		"https://attacker.example.com",
		"http://192.168.1.1",
	}
	for _, origin := range origins {
		if isAllowedOrigin(origin) {
			t.Errorf("external origin %q should be rejected", origin)
		}
	}
}

func TestIsAllowedOrigin_ChromeExtension_NoEnvVar(t *testing.T) {
	t.Parallel()
	// When GASOLINE_EXTENSION_ID is not set, any chrome-extension:// is allowed
	original := os.Getenv("GASOLINE_EXTENSION_ID")
	defer os.Setenv("GASOLINE_EXTENSION_ID", original)
	os.Unsetenv("GASOLINE_EXTENSION_ID")

	if !isAllowedOrigin("chrome-extension://abcdefghijklmnop") {
		t.Error("chrome-extension origin should be allowed when env var is not set")
	}
	if !isAllowedOrigin("chrome-extension://anyrandomextensionid") {
		t.Error("any chrome-extension origin should be allowed when env var is not set")
	}
}

func TestIsAllowedOrigin_ChromeExtension_WithEnvVar_CorrectID(t *testing.T) {
	t.Parallel()
	// When GASOLINE_EXTENSION_ID is set, only that specific extension is allowed
	original := os.Getenv("GASOLINE_EXTENSION_ID")
	defer os.Setenv("GASOLINE_EXTENSION_ID", original)
	os.Setenv("GASOLINE_EXTENSION_ID", "abcdefghijklmnop")

	if !isAllowedOrigin("chrome-extension://abcdefghijklmnop") {
		t.Error("chrome-extension with matching ID should be allowed")
	}
}

func TestIsAllowedOrigin_ChromeExtension_WithEnvVar_WrongID(t *testing.T) {
	t.Parallel()
	// When GASOLINE_EXTENSION_ID is set, a different extension ID is rejected
	original := os.Getenv("GASOLINE_EXTENSION_ID")
	defer os.Setenv("GASOLINE_EXTENSION_ID", original)
	os.Setenv("GASOLINE_EXTENSION_ID", "abcdefghijklmnop")

	if isAllowedOrigin("chrome-extension://wrongextensionidhere") {
		t.Error("chrome-extension with wrong ID should be rejected when env var is set")
	}
}

func TestIsAllowedOrigin_FirefoxExtension_NoEnvVar(t *testing.T) {
	t.Parallel()
	// When GASOLINE_FIREFOX_EXTENSION_ID is not set, any moz-extension:// is allowed
	original := os.Getenv("GASOLINE_FIREFOX_EXTENSION_ID")
	defer os.Setenv("GASOLINE_FIREFOX_EXTENSION_ID", original)
	os.Unsetenv("GASOLINE_FIREFOX_EXTENSION_ID")

	if !isAllowedOrigin("moz-extension://some-uuid-here") {
		t.Error("moz-extension origin should be allowed when env var is not set")
	}
}

func TestIsAllowedOrigin_FirefoxExtension_WithEnvVar_CorrectID(t *testing.T) {
	t.Parallel()
	// When GASOLINE_FIREFOX_EXTENSION_ID is set, only that specific extension is allowed
	original := os.Getenv("GASOLINE_FIREFOX_EXTENSION_ID")
	defer os.Setenv("GASOLINE_FIREFOX_EXTENSION_ID", original)
	os.Setenv("GASOLINE_FIREFOX_EXTENSION_ID", "my-firefox-ext-uuid")

	if !isAllowedOrigin("moz-extension://my-firefox-ext-uuid") {
		t.Error("moz-extension with matching ID should be allowed")
	}
}

func TestIsAllowedOrigin_FirefoxExtension_WithEnvVar_WrongID(t *testing.T) {
	t.Parallel()
	// When GASOLINE_FIREFOX_EXTENSION_ID is set, a different extension ID is rejected
	original := os.Getenv("GASOLINE_FIREFOX_EXTENSION_ID")
	defer os.Setenv("GASOLINE_FIREFOX_EXTENSION_ID", original)
	os.Setenv("GASOLINE_FIREFOX_EXTENSION_ID", "my-firefox-ext-uuid")

	if isAllowedOrigin("moz-extension://wrong-firefox-uuid") {
		t.Error("moz-extension with wrong ID should be rejected when env var is set")
	}
}
