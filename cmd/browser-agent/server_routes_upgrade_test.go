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

func TestHandleUpgradeNonce_ReturnsCurrentNonce(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/nonce", nil)
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
	rr := httptest.NewRecorder()
	s.handleUpgradeNonce(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleUpgradeInstall_ValidNonceSpawns(t *testing.T) {
	var spawned string
	s := newUpgradeTestServer(t, func(url string) error {
		spawned = url
		return nil
	})
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
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
	body := `{"nonce":"` + strings.Repeat("0", 64) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if spawnCalled {
		t.Fatal("spawn should not run for invalid nonce")
	}
}

func TestHandleUpgradeInstall_RejectsEmptyBody(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(""))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleUpgradeInstall_RejectsNonPOST(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/upgrade/install", nil)
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
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
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`

	// First request succeeds.
	req1 := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr1 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr1, req1)
	if rr1.Code != http.StatusAccepted {
		t.Fatalf("first request status = %d, want 202", rr1.Code)
	}

	// Second request within the window is rejected.
	req2 := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr2 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr2, req2)
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
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rr.Code)
	}
}

func TestHandleUpgradeInstall_SpawnErrorReturns500(t *testing.T) {
	s := newUpgradeTestServer(t, func(string) error {
		return errors.New("boom")
	})
	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
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
	// Body >1KB: MaxBytesReader should cause Decode to fail and handler to 400.
	oversized := `{"nonce":"` + strings.Repeat("a", 2048) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(oversized))
	rr := httptest.NewRecorder()
	s.handleUpgradeInstall(rr, req)
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

	body := `{"nonce":"` + s.upgradeNonce.Current() + `"}`
	req1 := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr1 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr1, req1)
	if rr1.Code != http.StatusAccepted {
		t.Fatalf("first request status = %d, want 202", rr1.Code)
	}

	// Simulate window elapsing by manually backdating the last-attempt stamp.
	s.upgradeMu.Lock()
	s.lastUpgradeAttempt = time.Now().Add(-2 * upgradeRateLimitWindow)
	s.upgradeMu.Unlock()

	req2 := httptest.NewRequest(http.MethodPost, "/upgrade/install", strings.NewReader(body))
	rr2 := httptest.NewRecorder()
	s.handleUpgradeInstall(rr2, req2)
	if rr2.Code != http.StatusAccepted {
		t.Fatalf("second request after window status = %d, want 202", rr2.Code)
	}
}
