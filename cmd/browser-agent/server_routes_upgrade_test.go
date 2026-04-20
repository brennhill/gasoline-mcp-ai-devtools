// server_routes_upgrade_test.go — Tests for /upgrade/nonce and /upgrade/install.
// Docs: docs/features/feature/self-update/index.md

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upgrade"
)

const testExtensionOrigin = "chrome-extension://abcdefghijklmnopabcdefghijklmnop"

// newUpgradeTestServer returns a Server wired with a fresh nonce and the
// previous spawn function restored on test cleanup.
func newUpgradeTestServer(t *testing.T, spawn func(string) error) *Server {
	t.Helper()
	s := &Server{
		upgradeNonce: upgrade.NewNonce(),
	}
	prev := upgradeSpawnFn
	upgradeSpawnFn = spawn
	t.Cleanup(func() { upgradeSpawnFn = prev })
	return s
}

// pinTestOrigin records testExtensionOrigin as the pinned origin so handler
// tests that skip the nonce-GET step and call install directly still pass the
// Verify gate.
func pinTestOrigin(s *Server) {
	s.upgradeNonce.Pin(testExtensionOrigin)
}

// newInstallRequest builds a POST /upgrade/install request with the caller's
// body and presets the expected Origin header so the nonce-origin check
// passes for handler-level tests that skip the GET path.
func newInstallRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	req.Header.Set("Origin", testExtensionOrigin)
	return req
}

func TestHandleUpgradeNonce_ReturnsCurrentNonce(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/nonce", nil)
	req.Header.Set("Origin", testExtensionOrigin)
	rr := httptest.NewRecorder()
	s.handleUpgradeNonce(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["nonce"] != s.upgradeNonce.Current() {
		t.Fatalf("nonce = %q, want %q", body["nonce"], s.upgradeNonce.Current())
	}
}

func TestHandleUpgradeNonce_RejectsNonGET(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodPost, "/upgrade/nonce", nil)
	req.Header.Set("Origin", testExtensionOrigin)
	rr := httptest.NewRecorder()
	s.handleUpgradeNonce(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleUpgradeNonce_RejectsMissingOrigin(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/nonce", nil)
	// deliberately no Origin header
	rr := httptest.NewRecorder()
	s.handleUpgradeNonce(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if pinned := s.upgradeNonce.PinnedOrigin(); pinned != "" {
		t.Fatalf("PinnedOrigin = %q, want empty — missing-Origin request must not pin", pinned)
	}
}

func TestHandleUpgradeNonce_PinsOriginOnFirstCall(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/nonce", nil)
	req.Header.Set("Origin", testExtensionOrigin)
	rr := httptest.NewRecorder()
	s.handleUpgradeNonce(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := s.upgradeNonce.PinnedOrigin(); got != testExtensionOrigin {
		t.Fatalf("PinnedOrigin = %q, want %q", got, testExtensionOrigin)
	}

	// A second GET from a different origin does NOT overwrite the first.
	other := "chrome-extension://zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
	req2 := httptest.NewRequest(http.MethodGet, "/upgrade/nonce", nil)
	req2.Header.Set("Origin", other)
	rr2 := httptest.NewRecorder()
	s.handleUpgradeNonce(rr2, req2)
	if got := s.upgradeNonce.PinnedOrigin(); got != testExtensionOrigin {
		t.Fatalf("PinnedOrigin after second GET = %q, want %q (first-origin must win)", got, testExtensionOrigin)
	}
}

func TestHandleUpgradeInstall_ValidNonceSpawns(t *testing.T) {
	var spawned string
	s := newUpgradeTestServer(t, func(url string) error {
		spawned = url
		return nil
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(body))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body=%s", rr.Code, rr.Body.String())
	}
	if spawned != upgradeInstallURL {
		t.Fatalf("spawned URL = %q, want %q", spawned, upgradeInstallURL)
	}
}

func TestHandleUpgradeInstall_RejectsWrongNonce(t *testing.T) {
	spawnCalled := false
	s := newUpgradeTestServer(t, func(string) error {
		spawnCalled = true
		return nil
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + strings.Repeat("0", 64) + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(body))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if spawnCalled {
		t.Fatal("spawn should not run for invalid nonce")
	}
}

func TestHandleUpgradeInstall_RejectsEmptyBody(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	pinTestOrigin(s)
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(""))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleUpgradeInstall_RejectsNonPOST(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/install", nil)
	req.Header.Set("Origin", testExtensionOrigin)
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleUpgradeInstall_RejectsMismatchedOrigin(t *testing.T) {
	spawnCalled := false
	s := newUpgradeTestServer(t, func(string) error {
		spawnCalled = true
		return nil
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	req.Header.Set("Origin", "chrome-extension://differentextensionid12345678901234")
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if spawnCalled {
		t.Fatal("spawn must not run for mismatched origin")
	}
}

func TestHandleUpgradeInstall_RejectsUnpinnedNonce(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	// Deliberately SKIP pinTestOrigin — nonce has never been fetched.
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(body))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (unpinned nonce must be rejected)", rr.Code)
	}
}

func TestHandleUpgradeInstall_RateLimited(t *testing.T) {
	var spawnCount int
	var mu sync.Mutex
	s := newUpgradeTestServer(t, func(string) error {
		mu.Lock()
		defer mu.Unlock()
		spawnCount++
		return nil
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`

	// First request succeeds.
	rr1 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr1, newInstallRequest(body))
	if rr1.Code != http.StatusAccepted {
		t.Fatalf("first request status = %d, want 202", rr1.Code)
	}

	// Second request within the window is rejected.
	rr2 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr2, newInstallRequest(body))
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rr2.Code)
	}
	if spawnCount != 1 {
		t.Fatalf("spawnCount = %d, want 1 (second request must not spawn)", spawnCount)
	}
}

func TestHandleUpgradeInstall_UnsupportedPlatform(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error {
		return upgrade.ErrUnsupportedPlatform
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(body))
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rr.Code)
	}
}

func TestHandleUpgradeInstall_SpawnErrorReturns500(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error {
		return errors.New("boom")
	})
	pinTestOrigin(s)
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(body))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestHandleUpgradeInstall_RejectsLargeBody(t *testing.T) {
	spawnCalled := false
	s := newUpgradeTestServer(t, func(string) error {
		spawnCalled = true
		return nil
	})
	pinTestOrigin(s)
	// Body >1KB: MaxBytesReader should cause Decode to fail and handler to 400.
	oversized := `{"nonce":"` + strings.Repeat("a", 2048) + `"}`
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, newInstallRequest(oversized))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if spawnCalled {
		t.Fatal("spawn must not run for oversized body")
	}
}

// After the rate-limit window elapses, a second request should succeed again.
func TestHandleUpgradeInstall_WindowElapses(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	pinTestOrigin(s)

	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	rr1 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr1, newInstallRequest(body))
	if rr1.Code != http.StatusAccepted {
		t.Fatalf("first request status = %d, want 202", rr1.Code)
	}

	// Simulate window elapsing by manually backdating the last-attempt stamp.
	s.upgradeMu.Lock()
	s.lastUpgradeAttempt = time.Now().Add(-2 * upgradeRateLimitWindow)
	s.upgradeMu.Unlock()

	rr2 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr2, newInstallRequest(body))
	if rr2.Code != http.StatusAccepted {
		t.Fatalf("second request after window status = %d, want 202", rr2.Code)
	}
}
