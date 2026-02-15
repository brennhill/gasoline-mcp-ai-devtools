package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func resetVersionCheckState(t *testing.T) {
	t.Helper()
	versionCheckMu.Lock()
	availableVersion = ""
	lastVersionCheck = time.Time{}
	versionFetchActive = false
	versionCheckMu.Unlock()
}

func TestCheckGitHubVersionSuccessAndCache(t *testing.T) {
	resetVersionCheckState(t)
	oldURL := getGitHubAPIURL()
	defer setGitHubAPIURL(oldURL)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer srv.Close()
	setGitHubAPIURL(srv.URL)

	checkGitHubVersion()
	versionCheckMu.Lock()
	gotVersion := availableVersion
	gotLast := lastVersionCheck
	versionCheckMu.Unlock()
	if gotVersion != "9.9.9" {
		t.Fatalf("availableVersion = %q, want %q", gotVersion, "9.9.9")
	}
	if gotLast.IsZero() {
		t.Fatal("lastVersionCheck should be set after successful fetch")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hits = %d, want 1", got)
	}

	// Cache TTL path: second call should not issue a new HTTP request.
	checkGitHubVersion()
	if got := hits.Load(); got != 1 {
		t.Fatalf("cache miss: server hits = %d, want still 1", got)
	}
}

func TestCheckGitHubVersionErrorPaths(t *testing.T) {
	resetVersionCheckState(t)
	oldURL := getGitHubAPIURL()
	defer setGitHubAPIURL(oldURL)

	t.Run("non-200 status ignored", func(t *testing.T) {
		resetVersionCheckState(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
		}))
		defer srv.Close()
		setGitHubAPIURL(srv.URL)
		checkGitHubVersion()

		versionCheckMu.Lock()
		defer versionCheckMu.Unlock()
		if availableVersion != "" {
			t.Fatalf("availableVersion = %q, want empty", availableVersion)
		}
		if !lastVersionCheck.IsZero() {
			t.Fatal("lastVersionCheck should remain zero on non-200 response")
		}
	})

	t.Run("invalid JSON ignored", func(t *testing.T) {
		resetVersionCheckState(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{invalid`))
		}))
		defer srv.Close()
		setGitHubAPIURL(srv.URL)
		checkGitHubVersion()

		versionCheckMu.Lock()
		defer versionCheckMu.Unlock()
		if availableVersion != "" {
			t.Fatalf("availableVersion = %q, want empty", availableVersion)
		}
		if !lastVersionCheck.IsZero() {
			t.Fatal("lastVersionCheck should remain zero on parse failure")
		}
	})

	t.Run("empty tag ignored", func(t *testing.T) {
		resetVersionCheckState(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name":""}`))
		}))
		defer srv.Close()
		setGitHubAPIURL(srv.URL)
		checkGitHubVersion()

		versionCheckMu.Lock()
		defer versionCheckMu.Unlock()
		if availableVersion != "" {
			t.Fatalf("availableVersion = %q, want empty", availableVersion)
		}
		if !lastVersionCheck.IsZero() {
			t.Fatal("lastVersionCheck should remain zero for empty tag")
		}
	})
}

func TestStartVersionCheckLoopInitialFetch(t *testing.T) {
	resetVersionCheckState(t)
	oldURL := getGitHubAPIURL()
	defer setGitHubAPIURL(oldURL)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v7.7.7"}`))
	}))
	defer srv.Close()
	setGitHubAPIURL(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	startVersionCheckLoop(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if got := hits.Load(); got == 0 {
		t.Fatal("startVersionCheckLoop did not perform initial check")
	}
	versionCheckMu.Lock()
	gotVersion := availableVersion
	versionCheckMu.Unlock()
	if gotVersion != "7.7.7" {
		t.Fatalf("availableVersion = %q, want %q", gotVersion, "7.7.7")
	}
}

func TestGetEnvOrDefaultSmoke(t *testing.T) {
	key := fmt.Sprintf("GASOLINE_TEST_ENV_%d", time.Now().UnixNano())
	if got := getEnvOrDefault(key, "fallback"); got != "fallback" {
		t.Fatalf("getEnvOrDefault(%q) = %q, want fallback", key, got)
	}
}
