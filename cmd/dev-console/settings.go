// @fileoverview Settings management with disk persistence
// Implements the plugin-server communications protocol documented in
// docs/plugin-server-communications.md

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SettingsPayload is the POST /settings request body
type SettingsPayload struct {
	SessionID string                 `json:"session_id"`
	Settings  map[string]interface{} `json:"settings"`
}

// PersistedSettings is the disk format for ~/.gasoline-settings.json
type PersistedSettings struct {
	AIWebPilotEnabled *bool     `json:"ai_web_pilot_enabled,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	SessionID         string    `json:"session_id"`
}

// getSettingsPath returns the path to the settings cache file
func getSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gasoline-settings.json"), nil
}

// LoadSettingsFromDisk loads cached settings from ~/.gasoline-settings.json
func (c *Capture) LoadSettingsFromDisk() {
	path, err := getSettingsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Could not determine settings path: %v\n", err)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[gasoline] Could not read settings file: %v\n", err)
		}
		return
	}

	var settings PersistedSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Could not parse settings file: %v\n", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Only apply settings if they're recent (to avoid using stale data from dead extension)
	// 5-second threshold matches the connection staleness check in checkPilotReady()
	age := time.Since(settings.Timestamp)
	if age > 5*time.Second {
		fmt.Fprintf(os.Stderr, "[gasoline] Ignoring stale cached settings (age: %v)\n", age)
		return
	}

	if settings.AIWebPilotEnabled != nil {
		c.pilotEnabled = *settings.AIWebPilotEnabled
		c.pilotUpdatedAt = settings.Timestamp
		fmt.Fprintf(os.Stderr, "[gasoline] Loaded cached settings: pilot=%v (age: %v)\n", c.pilotEnabled, age)
	}
}

// SaveSettingsToDisk persists current settings to ~/.gasoline-settings.json
func (c *Capture) SaveSettingsToDisk() error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	c.mu.RLock()
	settings := PersistedSettings{
		AIWebPilotEnabled: &c.pilotEnabled,
		Timestamp:         c.pilotUpdatedAt,
		SessionID:         c.extensionSession,
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// HandleSettings handles POST /settings from the extension
func (c *Capture) HandleSettings(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	sessionID := r.Header.Get("X-Gasoline-Session")
	clientID := r.Header.Get("X-Gasoline-Client")

	// Collect all headers for debug logging (redact auth)
	headers := make(map[string]string)
	for name, values := range r.Header {
		if strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") {
			headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	if r.Method != "POST" {
		// Allow GET for backward compatibility (returns empty for now)
		if r.Method == "GET" {
			jsonResponse(w, http.StatusOK, map[string]interface{}{})
			return
		}
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*10)) // 10KB limit
	if err != nil {
		duration := time.Since(startTime)
		debugEntry := HTTPDebugEntry{
			Timestamp:      startTime,
			Endpoint:       "/settings",
			Method:         "POST",
			SessionID:      sessionID,
			ClientID:       clientID,
			Headers:        headers,
			ResponseStatus: http.StatusBadRequest,
			DurationMs:     duration.Milliseconds(),
			Error:          fmt.Sprintf("Could not read body: %v", err),
		}
		c.mu.Lock()
		c.logHTTPDebugEntry(debugEntry)
		c.mu.Unlock()
		printHTTPDebug(debugEntry)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Could not read body"})
		return
	}

	requestPreview := string(body)
	if len(requestPreview) > 1000 {
		requestPreview = requestPreview[:1000] + "..."
	}

	var payload SettingsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		duration := time.Since(startTime)
		debugEntry := HTTPDebugEntry{
			Timestamp:      startTime,
			Endpoint:       "/settings",
			Method:         "POST",
			SessionID:      sessionID,
			ClientID:       clientID,
			Headers:        headers,
			RequestBody:    requestPreview,
			ResponseStatus: http.StatusBadRequest,
			DurationMs:     duration.Milliseconds(),
			Error:          fmt.Sprintf("Invalid JSON: %v", err),
		}
		c.mu.Lock()
		c.logHTTPDebugEntry(debugEntry)
		c.mu.Unlock()
		printHTTPDebug(debugEntry)
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	now := time.Now()
	c.mu.Lock()

	// Merge settings: only update fields that are present in the payload
	// This allows the extension to send partial updates (only initialized values)
	var pilotEnabledPtr *bool
	if aiPilot, ok := payload.Settings["aiWebPilotEnabled"].(bool); ok {
		c.pilotEnabled = aiPilot
		c.pilotUpdatedAt = now
		pilotEnabledPtr = &aiPilot
	}

	// Track session if provided
	if payload.SessionID != "" && payload.SessionID != c.extensionSession {
		if c.extensionSession != "" {
			fmt.Fprintf(os.Stderr, "[gasoline] Settings: session changed %s -> %s\n", c.extensionSession, payload.SessionID)
		}
		c.extensionSession = payload.SessionID
		c.sessionChangedAt = now
	}

	// Log settings POST to circular buffer
	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "settings",
		Method:       "POST",
		SessionID:    payload.SessionID,
		PilotEnabled: pilotEnabledPtr,
	})

	responseData := map[string]interface{}{
		"status":    "ok",
		"timestamp": now.Format(time.RFC3339),
	}
	responseJSON, _ := json.Marshal(responseData)
	responsePreview := string(responseJSON)

	// Log HTTP debug entry
	duration := time.Since(startTime)
	debugEntry := HTTPDebugEntry{
		Timestamp:      startTime,
		Endpoint:       "/settings",
		Method:         "POST",
		SessionID:      payload.SessionID,
		ClientID:       clientID,
		Headers:        headers,
		RequestBody:    requestPreview,
		ResponseStatus: http.StatusOK,
		ResponseBody:   responsePreview,
		DurationMs:     duration.Milliseconds(),
	}
	c.logHTTPDebugEntry(debugEntry)

	c.mu.Unlock()

	// Print debug log after unlocking to avoid deadlock
	printHTTPDebug(debugEntry)

	// Persist to disk (async, don't block response)
	go func() {
		if err := c.SaveSettingsToDisk(); err != nil {
			fmt.Fprintf(os.Stderr, "[gasoline] Failed to save settings to disk: %v\n", err)
		}
	}()

	jsonResponse(w, http.StatusOK, responseData)
}
