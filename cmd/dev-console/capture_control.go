// capture_control.go — AI-controlled capture settings with rate limiting and audit logging.
// Session-scoped overrides let AI agents adjust capture verbosity (log levels, WebSocket
// mode, network bodies) without modifying user config. All changes are rate-limited to
// 1 per second to prevent oscillation attacks. Every change is written to an append-only
// JSONL audit log (~/.gasoline/audit.jsonl) with rotation at 10MB (max 3 rotations).
// Design: Overrides are in-memory only — server restart resets to user defaults.
// The /settings endpoint exposes current overrides so the extension can poll and apply them.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// validSettings defines the allowed values for each capture setting.
var validSettings = map[string][]string{
	"log_level":          {"error", "warn", "all"},
	"ws_mode":            {"off", "lifecycle", "messages"},
	"network_bodies":     {"true", "false"},
	"screenshot_on_error": {"true", "false"},
	"action_replay":      {"true", "false"},
}

// defaultSettings defines the default value for each capture setting.
var defaultSettings = map[string]string{
	"log_level":          "error",
	"ws_mode":            "lifecycle",
	"network_bodies":     "true",
	"screenshot_on_error": "false",
	"action_replay":      "true",
}

// CaptureOverrides stores AI-set capture setting overrides. Session-scoped (in-memory only).
type CaptureOverrides struct {
	mu           sync.RWMutex
	overrides    map[string]string
	lastChangeAt time.Time
	pendingAlert *Alert
}

// NewCaptureOverrides creates a new empty override store.
func NewCaptureOverrides() *CaptureOverrides {
	return &CaptureOverrides{
		overrides: make(map[string]string),
	}
}

// validateCaptureSettingValue checks if a setting name and value are valid.
func validateCaptureSettingValue(setting, value string) error {
	validValues, ok := validSettings[setting]
	if !ok {
		names := make([]string, 0, len(validSettings))
		for k := range validSettings {
			names = append(names, k)
		}
		return fmt.Errorf("Unknown capture setting: %s. Valid: %s", setting, strings.Join(names, ", "))
	}
	for _, v := range validValues {
		if v == value {
			return nil
		}
	}
	return fmt.Errorf("Invalid value '%s' for %s. Valid: %s", value, setting, strings.Join(validValues, ", "))
}

// Set applies a single override. Returns an error if the setting or value is invalid,
// or if rate-limited (more than 1 change per second).
func (co *CaptureOverrides) Set(setting, value string) error {
	co.mu.Lock()
	defer co.mu.Unlock()

	if err := validateCaptureSettingValue(setting, value); err != nil {
		return err
	}

	// Rate limit: 1 change per second
	if !co.lastChangeAt.IsZero() && time.Since(co.lastChangeAt) < time.Second {
		return fmt.Errorf("Rate limited: capture settings can be changed at most once per second")
	}

	// Record previous value for alert
	prev := defaultSettings[setting]
	if existing, exists := co.overrides[setting]; exists {
		prev = existing
	}

	co.overrides[setting] = value
	co.lastChangeAt = time.Now()

	// Emit alert
	co.pendingAlert = &Alert{
		Severity:  "info",
		Category:  "capture_override",
		Title:     fmt.Sprintf("AI changed %s: %s → %s", setting, prev, value),
		Detail:    fmt.Sprintf("Capture setting '%s' changed from '%s' to '%s' by AI", setting, prev, value),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "capture_control",
	}

	return nil
}

// SetMultiple applies multiple overrides in one call (counts as one rate-limit event).
func (co *CaptureOverrides) SetMultiple(settings map[string]string) map[string]error {
	co.mu.Lock()
	defer co.mu.Unlock()

	// Rate limit check
	if !co.lastChangeAt.IsZero() && time.Since(co.lastChangeAt) < time.Second {
		errs := make(map[string]error)
		for k := range settings {
			errs[k] = fmt.Errorf("Rate limited: capture settings can be changed at most once per second")
		}
		return errs
	}

	errs := make(map[string]error)
	changed := []string{}

	for setting, value := range settings {
		if err := validateCaptureSettingValue(setting, value); err != nil {
			errs[setting] = err
			continue
		}
		co.overrides[setting] = value
		changed = append(changed, setting+"="+value)
		errs[setting] = nil
	}

	if len(changed) > 0 {
		co.lastChangeAt = time.Now()
		co.pendingAlert = &Alert{
			Severity:  "info",
			Category:  "capture_override",
			Title:     fmt.Sprintf("AI changed capture settings: %s", strings.Join(changed, ", ")),
			Detail:    fmt.Sprintf("Capture settings changed: %s", strings.Join(changed, ", ")),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "capture_control",
		}
	}

	return errs
}

// Reset clears all overrides.
func (co *CaptureOverrides) Reset() {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.overrides = make(map[string]string)
	co.pendingAlert = &Alert{
		Severity:  "info",
		Category:  "capture_override",
		Title:     "AI reset all capture settings to defaults",
		Detail:    "All capture overrides cleared. Extension will revert to user settings on next poll.",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Source:    "capture_control",
	}
}

// GetAll returns a copy of all current overrides.
func (co *CaptureOverrides) GetAll() map[string]string {
	co.mu.RLock()
	defer co.mu.RUnlock()
	result := make(map[string]string, len(co.overrides))
	for k, v := range co.overrides {
		result[k] = v
	}
	return result
}

// GetDefault returns the default value for a setting.
func (co *CaptureOverrides) GetDefault(setting string) string {
	return defaultSettings[setting]
}

// GetSettingsResponse returns the overrides map for the /settings endpoint.
func (co *CaptureOverrides) GetSettingsResponse() map[string]string {
	return co.GetAll()
}

// GetPageInfo returns override details for observe(what: "page") response.
func (co *CaptureOverrides) GetPageInfo() map[string]interface{} {
	co.mu.RLock()
	defer co.mu.RUnlock()
	if len(co.overrides) == 0 {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(co.overrides))
	for setting, value := range co.overrides {
		result[setting] = map[string]string{
			"value":   value,
			"default": defaultSettings[setting],
		}
	}
	return result
}

// DrainAlert returns and clears the pending alert (if any).
func (co *CaptureOverrides) DrainAlert() *Alert {
	co.mu.Lock()
	defer co.mu.Unlock()
	alert := co.pendingAlert
	co.pendingAlert = nil
	return alert
}

// --- Audit Logger ---

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	Timestamp string `json:"ts"`
	Event     string `json:"event"`
	Setting   string `json:"setting,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Source    string `json:"source"`
	Agent     string `json:"agent,omitempty"`
}

// AuditLogger writes structured audit events to a JSONL file with rotation.
type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	maxSize int64
}

// NewAuditLogger creates an audit logger at the given path.
// Creates parent directories if they don't exist.
func NewAuditLogger(path string) (*AuditLogger, error) {
	dir := filepath.Dir(path)
	// #nosec G301 -- 0755 for log directory is appropriate
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // #nosec G302 G304 -- audit logs are world-readable; path from caller
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &AuditLogger{
		file:    f,
		path:    path,
		maxSize: 10 * 1024 * 1024, // 10MB default
	}, nil
}

// Write appends an audit event to the log file.
func (al *AuditLogger) Write(event AuditEvent) {
	if al == nil {
		return
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	line := append(data, '\n')
	_, _ = al.file.Write(line) // #nosec G104 -- best-effort audit logging

	// Check if rotation is needed
	if info, err := al.file.Stat(); err == nil && info.Size() >= al.maxSize {
		al.rotate()
	}
}

// Close flushes and closes the audit log file.
func (al *AuditLogger) Close() {
	if al == nil {
		return
	}
	al.mu.Lock()
	defer al.mu.Unlock()
	if al.file != nil {
		_ = al.file.Close() // #nosec G104 -- best-effort close
	}
}

// rotate renames current file and shifts existing rotations.
func (al *AuditLogger) rotate() {
	_ = al.file.Close() // #nosec G104 -- best-effort close before rotation

	// Shift .2 → .3, .1 → .2
	for i := 2; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", al.path, i)
		new := fmt.Sprintf("%s.%d", al.path, i+1)
		_ = os.Rename(old, new) // #nosec G104 -- files may not exist, that's OK
	}

	// Current → .1
	_ = os.Rename(al.path, al.path+".1") // #nosec G104 -- best-effort rotation

	// Open new file
	f, err := os.OpenFile(al.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // #nosec G302 G304 -- audit logs are world-readable; path from internal field
	if err != nil {
		return
	}
	al.file = f
}

// --- Settings Endpoint Response ---

// SettingsResponse is returned by GET /settings.
type SettingsResponse struct {
	Connected       bool              `json:"connected"`
	CaptureOverrides map[string]string `json:"capture_overrides"`
}

// buildSettingsResponse creates the response for the /settings endpoint.
func buildSettingsResponse(co *CaptureOverrides) SettingsResponse {
	overrides := co.GetAll()
	if overrides == nil {
		overrides = make(map[string]string)
	}
	return SettingsResponse{
		Connected:       true,
		CaptureOverrides: overrides,
	}
}

// --- Configure Handler ---

// handleCaptureSettings processes a configure(action: "capture") call with settings.
// Returns (result message, error message). One will be empty.
func handleCaptureSettings(co *CaptureOverrides, settings map[string]string, logger *AuditLogger, agent string) (string, string) {
	if len(settings) == 0 {
		return "", "No settings provided. Valid: log_level, ws_mode, network_bodies, screenshot_on_error, action_replay."
	}

	if len(settings) == 1 {
		// Single setting — use Set (includes rate limiting)
		for k, v := range settings {
			prev := co.GetDefault(k)
			existing := co.GetAll()
			if ex, ok := existing[k]; ok {
				prev = ex
			}
			if err := co.Set(k, v); err != nil {
				return "", err.Error()
			}
			// Audit log
			if logger != nil {
				logger.Write(AuditEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Event:     "capture_override",
					Setting:   k,
					From:      prev,
					To:        v,
					Source:    "ai",
					Agent:     agent,
				})
			}
			return fmt.Sprintf("Capture setting updated: %s = %s (was: %s)", k, v, prev), ""
		}
	}

	// Multiple settings — use SetMultiple
	existing := co.GetAll()
	errs := co.SetMultiple(settings)
	var errMsgs []string
	var successMsgs []string
	for k, err := range errs {
		if err != nil {
			errMsgs = append(errMsgs, err.Error())
		} else {
			prev := defaultSettings[k]
			if ex, ok := existing[k]; ok {
				prev = ex
			}
			successMsgs = append(successMsgs, fmt.Sprintf("%s = %s (was: %s)", k, settings[k], prev))
			// Audit log each
			if logger != nil {
				logger.Write(AuditEvent{
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Event:     "capture_override",
					Setting:   k,
					From:      prev,
					To:        settings[k],
					Source:    "ai",
					Agent:     agent,
				})
			}
		}
	}

	if len(errMsgs) > 0 {
		return "", strings.Join(errMsgs, "; ")
	}
	return fmt.Sprintf("Capture settings updated: %s", strings.Join(successMsgs, ", ")), ""
}

// handleCaptureReset processes a configure(action: "capture") call with settings="reset".
func handleCaptureReset(co *CaptureOverrides, logger *AuditLogger, agent string) (string, string) {
	co.Reset()
	if logger != nil {
		logger.Write(AuditEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Event:     "capture_reset",
			Reason:    "explicit",
			Source:    "ai",
			Agent:     agent,
		})
	}
	return "All capture overrides reset to defaults.", ""
}

// --- MCP Tool Handler ---

// toolConfigureCapture handles configure(action: "capture", settings: {...}) MCP calls.
func (h *ToolHandler) toolConfigureCapture(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Settings json.RawMessage `json:"settings"`
	}
	_ = json.Unmarshal(args, &params)

	if params.Settings == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(
			"Required parameter 'settings' is missing. Provide a map of settings or \"reset\".")}
	}

	// Check if settings is the string "reset"
	var resetStr string
	if json.Unmarshal(params.Settings, &resetStr) == nil && resetStr == "reset" {
		result, errMsg := handleCaptureReset(h.captureOverrides, h.auditLogger, "unknown")
		if errMsg != "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(errMsg)}
		}
		// Drain alert and add to buffer
		if alert := h.captureOverrides.DrainAlert(); alert != nil {
			h.addAlert(*alert)
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(result)}
	}

	// Parse as settings map
	var settings map[string]string
	if err := json.Unmarshal(params.Settings, &settings); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(
			"Invalid 'settings' parameter. Provide a map of setting names to values, or \"reset\".")}
	}

	result, errMsg := handleCaptureSettings(h.captureOverrides, settings, h.auditLogger, "unknown")
	if errMsg != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpErrorResponse(errMsg)}
	}

	// Drain alert and add to buffer
	if alert := h.captureOverrides.DrainAlert(); alert != nil {
		h.addAlert(*alert)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(result)}
}
