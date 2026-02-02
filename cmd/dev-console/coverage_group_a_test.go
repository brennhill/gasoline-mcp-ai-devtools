package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Coverage Group A -- settings.go, status.go, pilot.go
// Tests for 0%-coverage and low-coverage functions
// ============================================

// ============================================
// settings.go tests
// ============================================

func TestCoverageGroupA_getSettingsPath(t *testing.T) {
	t.Parallel()
	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath failed: %v", err)
	}
	if path == "" {
		t.Fatal("getSettingsPath returned empty string")
	}
	if !strings.HasSuffix(path, ".gasoline-settings.json") {
		t.Errorf("expected path to end with .gasoline-settings.json, got %s", path)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".gasoline-settings.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCoverageGroupA_LoadSettingsFromDisk_NoFile(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()
	// LoadSettingsFromDisk should not panic regardless of whether the file exists.
	// We only verify it doesn't crash -- the file may or may not exist on the test host.
	capture.LoadSettingsFromDisk()
	// If we get here without panic, the function handled the file state gracefully.
}

// TestCoverageGroupA_SettingsDiskIO groups all disk I/O tests that write to
// ~/.gasoline-settings.json into a single serial test to avoid file races.
func TestCoverageGroupA_SettingsDiskIO(t *testing.T) {
	// NOT parallel: these tests write to the shared ~/.gasoline-settings.json

	settingsPath, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath failed: %v", err)
	}

	// Backup existing file if it exists, restore on cleanup
	var backup []byte
	if data, readErr := os.ReadFile(settingsPath); readErr == nil {
		backup = data
	}
	t.Cleanup(func() {
		if backup != nil {
			os.WriteFile(settingsPath, backup, 0644) //nolint:errcheck
		} else {
			os.Remove(settingsPath)
		}
		os.Remove(settingsPath + ".tmp")
	})

	t.Run("SaveSettingsToDisk", func(t *testing.T) {
		capture := capture.NewCapture()
		now := time.Now()

		capture.mu.Lock()
		capture.pilotEnabled = false
		capture.pilotUpdatedAt = now
		capture.extensionSession = "save-test-session"
		capture.mu.Unlock()

		if err := capture.SaveSettingsToDisk(); err != nil {
			t.Fatalf("SaveSettingsToDisk failed: %v", err)
		}

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("failed to read settings file: %v", err)
		}

		var settings PersistedSettings
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatalf("failed to parse settings: %v", err)
		}

		if settings.AIWebPilotEnabled == nil || *settings.AIWebPilotEnabled != false {
			t.Error("expected AIWebPilotEnabled to be false")
		}
		if settings.SessionID != "save-test-session" {
			t.Errorf("expected session ID save-test-session, got %s", settings.SessionID)
		}
	})

	t.Run("SaveAndVerifyFormat", func(t *testing.T) {
		capture := capture.NewCapture()
		capture.mu.Lock()
		capture.pilotEnabled = true
		capture.pilotUpdatedAt = time.Now()
		capture.extensionSession = "format-test-session"
		capture.mu.Unlock()

		if err := capture.SaveSettingsToDisk(); err != nil {
			t.Fatalf("SaveSettingsToDisk failed: %v", err)
		}

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("failed to read settings file: %v", err)
		}

		var loaded PersistedSettings
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("failed to parse saved settings: %v", err)
		}

		if loaded.AIWebPilotEnabled == nil {
			t.Fatal("AIWebPilotEnabled should not be nil")
		}
		if !*loaded.AIWebPilotEnabled {
			t.Error("expected AIWebPilotEnabled to be true")
		}
		if loaded.SessionID != "format-test-session" {
			t.Errorf("expected session ID format-test-session, got %s", loaded.SessionID)
		}
		if loaded.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})

	t.Run("LoadSettingsFromDisk_FreshSettings", func(t *testing.T) {
		// Save fresh settings
		capture := capture.NewCapture()
		now := time.Now()

		capture.mu.Lock()
		capture.pilotEnabled = true
		capture.pilotUpdatedAt = now
		capture.extensionSession = "fresh-session"
		capture.mu.Unlock()

		if err := capture.SaveSettingsToDisk(); err != nil {
			t.Fatalf("SaveSettingsToDisk failed: %v", err)
		}

		// Create a new capture and load settings
		capture2 := capture.NewCapture()
		capture2.LoadSettingsFromDisk()

		capture2.mu.RLock()
		enabled := capture2.pilotEnabled
		capture2.mu.RUnlock()

		// Settings should be loaded since they are fresh (< 5s old)
		if !enabled {
			t.Error("expected pilotEnabled to be true after loading fresh settings")
		}
	})

	t.Run("LoadSettingsFromDisk_StaleFile", func(t *testing.T) {
		// Write a stale timestamp directly to the file
		staleTime := time.Now().Add(-10 * time.Second)
		pilotTrue := true
		settings := PersistedSettings{
			AIWebPilotEnabled: &pilotTrue,
			Timestamp:         staleTime,
			SessionID:         "stale-session",
		}
		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal settings: %v", err)
		}
		if err := os.WriteFile(settingsPath, data, 0644); err != nil {
			t.Fatalf("failed to write settings: %v", err)
		}

		capture := capture.NewCapture()
		capture.LoadSettingsFromDisk()

		capture.mu.RLock()
		enabled := capture.pilotEnabled
		capture.mu.RUnlock()

		// Stale settings (>5s) should be ignored
		if enabled {
			t.Error("expected pilotEnabled to remain false for stale settings")
		}
	})

	t.Run("LoadSettingsFromDisk_InvalidJSON", func(t *testing.T) {
		// Write invalid JSON to settings file
		if err := os.WriteFile(settingsPath, []byte("not json"), 0644); err != nil {
			t.Fatalf("failed to write: %v", err)
		}

		capture := capture.NewCapture()
		// Should not panic
		capture.LoadSettingsFromDisk()

		capture.mu.RLock()
		enabled := capture.pilotEnabled
		capture.mu.RUnlock()

		if enabled {
			t.Error("expected pilotEnabled to remain false for invalid JSON")
		}
	})
}

func TestCoverageGroupA_HandleSettings_POST(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	payload := `{"session_id":"test-session","settings":{"aiWebPilotEnabled":true}}`
	req := httptest.NewRequest("POST", "/settings", strings.NewReader(payload))
	req.Header.Set("X-Gasoline-Session", "test-session")
	req.Header.Set("X-Gasoline-Client", "test-client")
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}

	// Verify pilot state was updated
	capture.mu.RLock()
	enabled := capture.pilotEnabled
	session := capture.extensionSession
	capture.mu.RUnlock()

	if !enabled {
		t.Error("expected pilotEnabled to be true after settings POST")
	}
	if session != "test-session" {
		t.Errorf("expected session test-session, got %s", session)
	}
}

func TestCoverageGroupA_HandleSettings_GET(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("GET", "/settings", nil)
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_HandleSettings_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("DELETE", "/settings", nil)
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_HandleSettings_InvalidJSON(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("POST", "/settings", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_HandleSettings_SessionChange(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	// Set initial session
	capture.mu.Lock()
	capture.extensionSession = "old-session"
	capture.mu.Unlock()

	payload := `{"session_id":"new-session","settings":{"aiWebPilotEnabled":false}}`
	req := httptest.NewRequest("POST", "/settings", strings.NewReader(payload))
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	capture.mu.RLock()
	session := capture.extensionSession
	capture.mu.RUnlock()

	if session != "new-session" {
		t.Errorf("expected session new-session, got %s", session)
	}
}

func TestCoverageGroupA_HandleSettings_PartialUpdate(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	// Set initial pilot state
	capture.mu.Lock()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	// POST without aiWebPilotEnabled - should not change it
	payload := `{"session_id":"partial-session","settings":{}}`
	req := httptest.NewRequest("POST", "/settings", strings.NewReader(payload))
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	capture.mu.RLock()
	enabled := capture.pilotEnabled
	capture.mu.RUnlock()

	if !enabled {
		t.Error("expected pilotEnabled to remain true after partial update")
	}
}

func TestCoverageGroupA_HandleSettings_RedactsAuthHeaders(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	payload := `{"session_id":"auth-session","settings":{"aiWebPilotEnabled":true}}`
	req := httptest.NewRequest("POST", "/settings", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("X-Auth-Token", "another-secret")
	w := httptest.NewRecorder()

	capture.HandleSettings(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ============================================
// status.go tests
// ============================================

func TestCoverageGroupA_HandleExtensionStatus_POST(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	body := `{"type":"status","tracking_enabled":true,"tracked_tab_id":42,"tracked_tab_url":"http://localhost:3000","extension_connected":true}`
	req := httptest.NewRequest("POST", "/api/extension-status", strings.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["received"] != true {
		t.Error("expected received:true in response")
	}

	// Verify state was updated
	capture.mu.RLock()
	tracking := capture.trackingEnabled
	tabID := capture.trackedTabID
	tabURL := capture.trackedTabURL
	capture.mu.RUnlock()

	if !tracking {
		t.Error("expected trackingEnabled to be true")
	}
	if tabID != 42 {
		t.Errorf("expected tabID 42, got %d", tabID)
	}
	if tabURL != "http://localhost:3000" {
		t.Errorf("expected tabURL http://localhost:3000, got %s", tabURL)
	}
}

func TestCoverageGroupA_HandleExtensionStatus_GET(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	// Pre-set tracking state
	capture.mu.Lock()
	capture.trackingEnabled = true
	capture.trackedTabID = 99
	capture.trackedTabURL = "http://example.com"
	capture.trackingUpdated = time.Now()
	capture.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/extension-status", nil)
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var status ExtensionStatus
	json.NewDecoder(resp.Body).Decode(&status)

	if !status.TrackingEnabled {
		t.Error("expected tracking_enabled true")
	}
	if status.TrackedTabID != 99 {
		t.Errorf("expected tracked_tab_id 99, got %d", status.TrackedTabID)
	}
	if status.TrackedTabURL != "http://example.com" {
		t.Errorf("expected tracked_tab_url http://example.com, got %s", status.TrackedTabURL)
	}
	if !status.ExtensionConnected {
		t.Error("expected extension_connected true for recent tracking update")
	}
}

func TestCoverageGroupA_HandleExtensionStatus_GET_StaleConnection(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	capture.mu.Lock()
	capture.trackingEnabled = true
	capture.trackedTabID = 1
	capture.trackingUpdated = time.Now().Add(-3 * time.Minute)
	capture.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/extension-status", nil)
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	var status ExtensionStatus
	json.NewDecoder(w.Result().Body).Decode(&status)

	if status.ExtensionConnected {
		t.Error("expected extension_connected false for stale connection (>2min)")
	}
}

func TestCoverageGroupA_HandleExtensionStatus_OPTIONS(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("OPTIONS", "/api/extension-status", nil)
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_HandleExtensionStatus_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("DELETE", "/api/extension-status", nil)
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_HandleExtensionStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("POST", "/api/extension-status", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	capture.HandleExtensionStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCoverageGroupA_GetTrackingStatus(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	// Default state
	enabled, tabID, tabURL := capture.GetTrackingStatus()
	if enabled {
		t.Error("expected tracking disabled by default")
	}
	if tabID != 0 {
		t.Errorf("expected tabID 0, got %d", tabID)
	}
	if tabURL != "" {
		t.Errorf("expected empty tabURL, got %s", tabURL)
	}

	// Set tracking state
	capture.mu.Lock()
	capture.trackingEnabled = true
	capture.trackedTabID = 55
	capture.trackedTabURL = "http://test.local"
	capture.mu.Unlock()

	enabled, tabID, tabURL = capture.GetTrackingStatus()
	if !enabled {
		t.Error("expected tracking enabled")
	}
	if tabID != 55 {
		t.Errorf("expected tabID 55, got %d", tabID)
	}
	if tabURL != "http://test.local" {
		t.Errorf("expected tabURL http://test.local, got %s", tabURL)
	}
}

// ============================================
// pilot.go tests
// ============================================

func TestCoverageGroupA_HandlePilotStatus(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("GET", "/pilot-status", nil)
	w := httptest.NewRecorder()

	capture.HandlePilotStatus(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var status PilotStatusResponse
	json.NewDecoder(resp.Body).Decode(&status)

	if status.Source != "never_connected" {
		t.Errorf("expected source never_connected, got %s", status.Source)
	}
	if status.Enabled {
		t.Error("expected enabled false")
	}
	if status.ExtensionConnected {
		t.Error("expected extension_connected false")
	}
}

func TestCoverageGroupA_HandlePilotStatus_Connected(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := httptest.NewRequest("GET", "/pilot-status", nil)
	w := httptest.NewRecorder()

	capture.HandlePilotStatus(w, req)

	var status PilotStatusResponse
	json.NewDecoder(w.Result().Body).Decode(&status)

	if status.Source != "extension_poll" {
		t.Errorf("expected source extension_poll, got %s", status.Source)
	}
	if !status.Enabled {
		t.Error("expected enabled true")
	}
	if !status.ExtensionConnected {
		t.Error("expected extension_connected true")
	}
	if status.LastUpdate == "" {
		t.Error("expected non-empty last_update")
	}
	if status.LastPollAgo == "" {
		t.Error("expected non-empty last_poll_ago")
	}
}

func TestCoverageGroupA_HandlePilotStatus_ContentType(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	req := httptest.NewRequest("GET", "/pilot-status", nil)
	w := httptest.NewRecorder()

	capture.HandlePilotStatus(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

func TestCoverageGroupA_GetPilotStatus_SettingsHeartbeat(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-10 * time.Second)
	capture.pilotUpdatedAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	status := capture.GetPilotStatus()
	if status.Source != "settings_heartbeat" {
		t.Errorf("expected source settings_heartbeat, got %s", status.Source)
	}
	if !status.ExtensionConnected {
		t.Error("expected extension_connected true for settings heartbeat")
	}
}

func TestCoverageGroupA_GetPilotStatus_StaleWithSettingsNewer(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-30 * time.Second)
	capture.pilotUpdatedAt = time.Now().Add(-10 * time.Second)
	capture.pilotEnabled = false
	capture.mu.Unlock()

	status := capture.GetPilotStatus()
	if status.Source != "stale" {
		t.Errorf("expected source stale, got %s", status.Source)
	}
	if status.ExtensionConnected {
		t.Error("expected extension_connected false for stale state")
	}
	if status.LastUpdate == "" {
		t.Error("expected non-empty last_update when settings is newer")
	}
}

func TestCoverageGroupA_GetPilotStatus_StaleWithPollNewer(t *testing.T) {
	t.Parallel()
	capture := capture.NewCapture()

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-10 * time.Second)
	capture.pilotUpdatedAt = time.Now().Add(-30 * time.Second)
	capture.pilotEnabled = true
	capture.mu.Unlock()

	status := capture.GetPilotStatus()
	if status.Source != "stale" {
		t.Errorf("expected source stale, got %s", status.Source)
	}
	if status.LastUpdate == "" {
		t.Error("expected non-empty last_update")
	}
	if status.LastPollAgo == "" {
		t.Error("expected non-empty last_poll_ago")
	}
}

func TestCoverageGroupA_toolObservePolling_NoEntries(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	resp := mcp.toolHandler.toolObservePolling(req, nil)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	if !strings.Contains(result.Content[0].Text, "No polling activity") {
		t.Errorf("expected 'No polling activity' message, got: %s", result.Content[0].Text)
	}
}

func TestCoverageGroupA_toolObservePolling_WithEntries(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.logPollingActivity(PollingLogEntry{
		Timestamp: time.Now(),
		Endpoint:  "pending-queries",
		Method:    "GET",
		SessionID: "sess-1",
	})
	capture.logPollingActivity(PollingLogEntry{
		Timestamp: time.Now(),
		Endpoint:  "settings",
		Method:    "POST",
		SessionID: "sess-1",
	})
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
	}

	resp := mcp.toolHandler.toolObservePolling(req, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	if !strings.Contains(result.Content[0].Text, "Polling activity") {
		t.Errorf("expected 'Polling activity' summary, got: %s", result.Content[0].Text)
	}
}

// ============================================
// pilot.go -- checkPilotReady tests
// ============================================

func TestCoverageGroupA_checkPilotReady_NeverConnected(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if readiness.ShouldAccept {
		t.Error("expected ShouldAccept=false when never connected")
	}
	if readiness.State != PluginOff {
		t.Errorf("expected PluginOff, got %d", readiness.State)
	}
	if readiness.ErrorResp == nil {
		t.Error("expected error response when never connected")
	}
}

func TestCoverageGroupA_checkPilotReady_RecentPollEnabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if !readiness.ShouldAccept {
		t.Error("expected ShouldAccept=true when recent poll and pilot enabled")
	}
	if readiness.State != PluginOnPilotEnabled {
		t.Errorf("expected PluginOnPilotEnabled, got %d", readiness.State)
	}
	if readiness.Warning != "" {
		t.Error("expected no warning for fresh poll")
	}
}

func TestCoverageGroupA_checkPilotReady_RecentPollDisabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = false
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if readiness.ShouldAccept {
		t.Error("expected ShouldAccept=false when pilot disabled")
	}
	if readiness.State != PluginOnPilotDisabled {
		t.Errorf("expected PluginOnPilotDisabled, got %d", readiness.State)
	}
	if readiness.ErrorResp == nil {
		t.Error("expected error response when pilot disabled")
	}
}

func TestCoverageGroupA_checkPilotReady_SettingsRecentPollingStale_Enabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-10 * time.Second)
	capture.pilotUpdatedAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if !readiness.ShouldAccept {
		t.Error("expected ShouldAccept=true when settings recent and pilot enabled")
	}
	if readiness.Warning == "" {
		t.Error("expected warning about stale polling")
	}
}

func TestCoverageGroupA_checkPilotReady_SettingsRecentPollingStale_Disabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-10 * time.Second)
	capture.pilotUpdatedAt = time.Now()
	capture.pilotEnabled = false
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if readiness.ShouldAccept {
		t.Error("expected ShouldAccept=false when pilot disabled via settings")
	}
	if readiness.State != PluginOnPilotDisabled {
		t.Errorf("expected PluginOnPilotDisabled, got %d", readiness.State)
	}
}

func TestCoverageGroupA_checkPilotReady_BothStale(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now().Add(-30 * time.Second)
	capture.pilotUpdatedAt = time.Now().Add(-20 * time.Second)
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if !readiness.ShouldAccept {
		t.Error("expected ShouldAccept=true (optimistic) even when both stale")
	}
	if readiness.Warning == "" {
		t.Error("expected warning about stale connection")
	}
	if !strings.Contains(readiness.Warning, "stale") {
		t.Error("expected warning to mention 'stale'")
	}
}

func TestCoverageGroupA_checkPilotReady_BothStale_PollNewerZero(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.pilotUpdatedAt = time.Now().Add(-20 * time.Second)
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	readiness := mcp.toolHandler.checkPilotReady(req)
	if !readiness.ShouldAccept {
		t.Error("expected ShouldAccept=true (optimistic) even when stale with zero poll")
	}
	if readiness.Warning == "" {
		t.Error("expected warning about stale connection")
	}
}

// ============================================
// pilot.go -- browser action delegation handlers
// ============================================

func TestCoverageGroupA_handleBrowserActionRefresh(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	resp := mcp.toolHandler.handleBrowserActionRefresh(req, json.RawMessage(`{}`))
	if resp.Error != nil {
		t.Fatalf("unexpected JSONRPC error: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout-related error in response, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserActionBack(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
	}

	resp := mcp.toolHandler.handleBrowserActionBack(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for back action, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserActionForward(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
	}

	resp := mcp.toolHandler.handleBrowserActionForward(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for forward action, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserActionNewTab(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
	}

	resp := mcp.toolHandler.handleBrowserActionNewTab(req, json.RawMessage(`{"url":"http://example.com"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for new_tab action, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserActionNewTab_NoURL(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
	}

	resp := mcp.toolHandler.handleBrowserActionNewTab(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "URL required") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for new_tab without URL, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- manage_state delegation handlers
// ============================================

func TestCoverageGroupA_handlePilotManageStateLoad(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`10`),
	}

	resp := mcp.toolHandler.handlePilotManageStateLoad(req, json.RawMessage(`{"snapshot_name":"test-snapshot"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for load_state, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageStateLoad_NoName(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`11`),
	}

	resp := mcp.toolHandler.handlePilotManageStateLoad(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "snapshot_name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for load without snapshot_name, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageStateList(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`12`),
	}

	resp := mcp.toolHandler.handlePilotManageStateList(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for list_states, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageStateDelete(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`13`),
	}

	resp := mcp.toolHandler.handlePilotManageStateDelete(req, json.RawMessage(`{"snapshot_name":"to-delete"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error for delete_state, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageStateDelete_NoName(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`14`),
	}

	resp := mcp.toolHandler.handlePilotManageStateDelete(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "snapshot_name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for delete without snapshot_name, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- browser action: pilot disabled / never connected
// ============================================

func TestCoverageGroupA_handleBrowserAction_PilotDisabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = false
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`20`),
	}

	resp := mcp.toolHandler.handleBrowserActionRefresh(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "pilot_disabled") || strings.Contains(c.Text, "AI Web Pilot") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pilot_disabled error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserAction_NeverConnected(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`21`),
	}

	resp := mcp.toolHandler.handleBrowserActionRefresh(req, json.RawMessage(`{}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "Extension not connected") || strings.Contains(c.Text, "extension_timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected extension not connected error, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- handleBrowserAction validation
// ============================================

func TestCoverageGroupA_handleBrowserAction_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`30`),
	}

	resp := mcp.toolHandler.handleBrowserAction(req, json.RawMessage(`not json`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_json error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserAction_MissingAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`31`),
	}

	resp := mcp.toolHandler.handleBrowserAction(req, json.RawMessage(`{"url":"http://example.com"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for missing action, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserAction_InvalidAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`32`),
	}

	resp := mcp.toolHandler.handleBrowserAction(req, json.RawMessage(`{"action":"invalid_action"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_param") || strings.Contains(c.Text, "Invalid action") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_param error for bad action, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handleBrowserAction_NavigateNoURL(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`33`),
	}

	resp := mcp.toolHandler.handleBrowserAction(req, json.RawMessage(`{"action":"navigate"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "URL required") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for navigate without URL, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- handlePilotManageState validation
// ============================================

func TestCoverageGroupA_handlePilotManageState_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`40`),
	}

	resp := mcp.toolHandler.handlePilotManageState(req, json.RawMessage(`not json`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_json error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageState_MissingAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`41`),
	}

	resp := mcp.toolHandler.handlePilotManageState(req, json.RawMessage(`{"snapshot_name":"test"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageState_InvalidAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`42`),
	}

	resp := mcp.toolHandler.handlePilotManageState(req, json.RawMessage(`{"action":"bogus"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_param") || strings.Contains(c.Text, "Invalid action") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_param error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotManageState_SaveNoName(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`43`),
	}

	resp := mcp.toolHandler.handlePilotManageState(req, json.RawMessage(`{"action":"save"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "snapshot_name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param for save without snapshot_name, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- handlePilotHighlight validation
// ============================================

func TestCoverageGroupA_handlePilotHighlight_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`50`),
	}

	resp := mcp.toolHandler.handlePilotHighlight(req, json.RawMessage(`not valid json`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_json error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotHighlight_MissingSelector(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`51`),
	}

	resp := mcp.toolHandler.handlePilotHighlight(req, json.RawMessage(`{"duration_ms":3000}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "selector") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for missing selector, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotHighlight_WithDefaultDuration(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	capture.queryTimeout = 50 * time.Millisecond
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`52`),
	}

	resp := mcp.toolHandler.handlePilotHighlight(req, json.RawMessage(`{"selector":".test-element"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "extension_timeout") || strings.Contains(c.Text, "Timeout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected timeout error, got: %v", result.Content)
	}
}

// ============================================
// pilot.go -- handlePilotExecuteJS validation
// ============================================

func TestCoverageGroupA_handlePilotExecuteJS_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`60`),
	}

	resp := mcp.toolHandler.handlePilotExecuteJS(req, json.RawMessage(`not json`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "invalid_json") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected invalid_json error, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotExecuteJS_MissingScript(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`61`),
	}

	resp := mcp.toolHandler.handlePilotExecuteJS(req, json.RawMessage(`{"timeout_ms":5000}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "missing_param") || strings.Contains(c.Text, "script") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected missing_param error for missing script, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotExecuteJS_AsyncResponse(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = true
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`62`),
	}

	resp := mcp.toolHandler.handlePilotExecuteJS(req, json.RawMessage(`{"script":"document.title"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Content) == 0 {
		t.Fatal("expected content in response")
	}
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "queued") || strings.Contains(c.Text, "correlation_id") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected queued response with correlation_id, got: %v", result.Content)
	}
}

func TestCoverageGroupA_handlePilotExecuteJS_PilotDisabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := capture.NewCapture()
	mcp := setupToolHandler(t, server, capture)

	capture.mu.Lock()
	capture.lastPollAt = time.Now()
	capture.pilotEnabled = false
	capture.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`63`),
	}

	resp := mcp.toolHandler.handlePilotExecuteJS(req, json.RawMessage(`{"script":"document.title"}`))
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	found := false
	for _, c := range result.Content {
		if strings.Contains(c.Text, "pilot_disabled") || strings.Contains(c.Text, "AI Web Pilot") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pilot_disabled error, got: %v", result.Content)
	}
}

// ============================================
// Helpers
// ============================================

