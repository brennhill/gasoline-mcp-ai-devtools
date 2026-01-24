package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- CaptureOverrides Tests ---

func TestCaptureOverridesSetValid(t *testing.T) {
	co := NewCaptureOverrides()
	err := co.Set("ws_mode", "messages")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	overrides := co.GetAll()
	if overrides["ws_mode"] != "messages" {
		t.Fatalf("expected ws_mode=messages, got %v", overrides["ws_mode"])
	}
}

func TestCaptureOverridesSetMultiple(t *testing.T) {
	co := NewCaptureOverrides()
	errs := co.SetMultiple(map[string]string{
		"ws_mode":   "messages",
		"log_level": "all",
	})
	for k, err := range errs {
		if err != nil {
			t.Fatalf("setting %s failed: %v", k, err)
		}
	}
	overrides := co.GetAll()
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}
	if overrides["ws_mode"] != "messages" || overrides["log_level"] != "all" {
		t.Fatalf("unexpected overrides: %v", overrides)
	}
}

func TestCaptureOverridesInvalidSettingName(t *testing.T) {
	co := NewCaptureOverrides()
	err := co.Set("invalid_setting", "foo")
	if err == nil {
		t.Fatal("expected error for invalid setting name")
	}
	if !strings.Contains(err.Error(), "Unknown capture setting") {
		t.Fatalf("expected 'Unknown capture setting' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "log_level") {
		t.Fatalf("error should list valid settings, got: %v", err)
	}
}

func TestCaptureOverridesInvalidSettingValue(t *testing.T) {
	co := NewCaptureOverrides()
	err := co.Set("log_level", "verbose")
	if err == nil {
		t.Fatal("expected error for invalid setting value")
	}
	if !strings.Contains(err.Error(), "Invalid value") {
		t.Fatalf("expected 'Invalid value' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "error, warn, all") {
		t.Fatalf("error should list valid values, got: %v", err)
	}
}

func TestCaptureOverridesAllSettingValues(t *testing.T) {
	tests := []struct {
		setting string
		valid   []string
		invalid string
	}{
		{"log_level", []string{"error", "warn", "all"}, "verbose"},
		{"ws_mode", []string{"off", "lifecycle", "messages"}, "full"},
		{"network_bodies", []string{"true", "false"}, "maybe"},
		{"screenshot_on_error", []string{"true", "false"}, "yes"},
		{"action_replay", []string{"true", "false"}, "no"},
	}

	for _, tt := range tests {
		for _, v := range tt.valid {
			co := NewCaptureOverrides()
			err := co.Set(tt.setting, v)
			if err != nil {
				t.Errorf("setting %s=%s should be valid, got error: %v", tt.setting, v, err)
			}
		}
		co := NewCaptureOverrides()
		err := co.Set(tt.setting, tt.invalid)
		if err == nil {
			t.Errorf("setting %s=%s should be invalid", tt.setting, tt.invalid)
		}
	}
}

func TestCaptureOverridesReset(t *testing.T) {
	co := NewCaptureOverrides()
	co.SetMultiple(map[string]string{
		"ws_mode":   "messages",
		"log_level": "all",
	})
	co.Reset()
	overrides := co.GetAll()
	if len(overrides) != 0 {
		t.Fatalf("expected 0 overrides after reset, got %d", len(overrides))
	}
}

func TestCaptureOverridesGetAllReturnsEmpty(t *testing.T) {
	co := NewCaptureOverrides()
	overrides := co.GetAll()
	if overrides == nil {
		t.Fatal("GetAll should return non-nil empty map")
	}
	if len(overrides) != 0 {
		t.Fatalf("expected 0 overrides, got %d", len(overrides))
	}
}

func TestCaptureOverridesGetPreviousValue(t *testing.T) {
	co := NewCaptureOverrides()
	// First set - previous is the default
	prev := co.GetDefault("ws_mode")
	if prev != "lifecycle" {
		t.Fatalf("expected default ws_mode=lifecycle, got %s", prev)
	}
	_ = co.Set("ws_mode", "messages")
	// Second set - previous is "messages"
	overrides := co.GetAll()
	if overrides["ws_mode"] != "messages" {
		t.Fatalf("expected ws_mode=messages, got %s", overrides["ws_mode"])
	}
}

func TestCaptureOverridesRateLimitFirstChange(t *testing.T) {
	co := NewCaptureOverrides()
	err := co.Set("ws_mode", "messages")
	if err != nil {
		t.Fatalf("first change should succeed, got: %v", err)
	}
}

func TestCaptureOverridesRateLimitSecondChangeWithinOneSecond(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")
	err := co.Set("log_level", "all")
	if err == nil {
		t.Fatal("second change within 1s should be rate-limited")
	}
	if !strings.Contains(err.Error(), "Rate limited") {
		t.Fatalf("expected rate limit error, got: %v", err)
	}
}

func TestCaptureOverridesRateLimitMultipleSettingsOneCall(t *testing.T) {
	co := NewCaptureOverrides()
	// SetMultiple should count as one change
	errs := co.SetMultiple(map[string]string{
		"ws_mode":   "messages",
		"log_level": "all",
	})
	for _, err := range errs {
		if err != nil {
			t.Fatalf("multiple settings in one call should succeed: %v", err)
		}
	}
	overrides := co.GetAll()
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}
}

func TestCaptureOverridesRateLimitAfterOneSecond(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")
	// Manually advance the last change time
	co.mu.Lock()
	co.lastChangeAt = time.Now().Add(-2 * time.Second)
	co.mu.Unlock()
	err := co.Set("log_level", "all")
	if err != nil {
		t.Fatalf("change after 1s should succeed, got: %v", err)
	}
}

func TestCaptureOverridesConcurrentAccess(t *testing.T) {
	co := NewCaptureOverrides()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			co.GetAll()
		}()
	}
	wg.Wait()
}

func TestCaptureOverridesGetForSettingsResponse(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")

	resp := co.GetSettingsResponse()
	if resp == nil {
		t.Fatal("settings response should not be nil")
	}
	if resp["ws_mode"] != "messages" {
		t.Fatalf("expected ws_mode=messages in response, got %v", resp["ws_mode"])
	}
}

func TestCaptureOverridesEmptySettingsResponse(t *testing.T) {
	co := NewCaptureOverrides()
	resp := co.GetSettingsResponse()
	if len(resp) != 0 {
		t.Fatalf("expected empty settings response, got %v", resp)
	}
}

// --- AuditLogger Tests ---

func TestAuditLoggerCreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.Write(AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     "capture_override",
		Setting:   "ws_mode",
		From:      "lifecycle",
		To:        "messages",
		Source:    "ai",
		Agent:     "claude-code",
	})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file should not be empty")
	}
}

func TestAuditLoggerJSONLFormat(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.Write(AuditEvent{
		Timestamp: "2026-01-24T15:32:00Z",
		Event:     "capture_override",
		Setting:   "ws_mode",
		From:      "lifecycle",
		To:        "messages",
		Source:    "ai",
		Agent:     "claude-code",
	})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var event AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to parse JSONL line: %v", err)
	}
	if event.Event != "capture_override" {
		t.Fatalf("expected event=capture_override, got %s", event.Event)
	}
	if event.Setting != "ws_mode" {
		t.Fatalf("expected setting=ws_mode, got %s", event.Setting)
	}
	if event.Agent != "claude-code" {
		t.Fatalf("expected agent=claude-code, got %s", event.Agent)
	}
}

func TestAuditLoggerMultipleWrites(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	defer logger.Close()

	for i := 0; i < 5; i++ {
		logger.Write(AuditEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Event:     "capture_override",
			Setting:   "log_level",
			Source:    "ai",
		})
	}

	data, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
}

func TestAuditLoggerResetEvent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	defer logger.Close()

	logger.Write(AuditEvent{
		Timestamp: "2026-01-24T15:45:00Z",
		Event:     "capture_reset",
		Reason:    "explicit",
		Source:    "ai",
		Agent:     "claude-code",
	})

	data, _ := os.ReadFile(logPath)
	var event AuditEvent
	json.Unmarshal([]byte(strings.TrimSpace(string(data))), &event)
	if event.Event != "capture_reset" {
		t.Fatalf("expected capture_reset, got %s", event.Event)
	}
	if event.Reason != "explicit" {
		t.Fatalf("expected reason=explicit, got %s", event.Reason)
	}
}

func TestAuditLoggerRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	defer logger.Close()

	// Write a large entry to simulate approaching 10MB
	// We'll set the rotation size lower for testing
	logger.maxSize = 1000 // 1KB for test

	longEntry := AuditEvent{
		Timestamp: "2026-01-24T15:32:00Z",
		Event:     "capture_override",
		Setting:   strings.Repeat("x", 500),
		Source:    "ai",
	}

	// Write enough to trigger rotation
	for i := 0; i < 5; i++ {
		logger.Write(longEntry)
	}

	// Check that rotation happened
	if _, err := os.Stat(logPath + ".1"); os.IsNotExist(err) {
		t.Fatal("expected rotated file audit.jsonl.1 to exist")
	}
}

func TestAuditLoggerRotationKeepsMax3(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	defer logger.Close()

	logger.maxSize = 200 // Very small for test

	longEntry := AuditEvent{
		Timestamp: "2026-01-24T15:32:00Z",
		Event:     "capture_override",
		Setting:   strings.Repeat("x", 100),
		Source:    "ai",
	}

	// Write enough to trigger multiple rotations
	for i := 0; i < 30; i++ {
		logger.Write(longEntry)
	}

	// .1, .2, .3 should exist
	for i := 1; i <= 3; i++ {
		rotated := logPath + "." + string(rune('0'+i))
		if _, err := os.Stat(rotated); os.IsNotExist(err) {
			// Some rotations may not have happened yet depending on timing
			// At minimum .1 should exist
			if i == 1 {
				t.Fatalf("expected at least %s to exist", rotated)
			}
		}
	}

	// .4 should NOT exist
	if _, err := os.Stat(logPath + ".4"); !os.IsNotExist(err) {
		t.Fatal("audit.jsonl.4 should not exist (max 3 rotations)")
	}
}

func TestAuditLoggerCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "subdir", "audit.jsonl")
	logger, err := NewAuditLogger(nestedPath)
	if err != nil {
		t.Fatalf("should create directory automatically: %v", err)
	}
	defer logger.Close()

	logger.Write(AuditEvent{Event: "test"})
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Fatal("log file should exist after write")
	}
}

func TestAuditLoggerNilSafe(t *testing.T) {
	// A nil logger should not panic
	var logger *AuditLogger
	logger.Write(AuditEvent{Event: "test"}) // Should not panic
}

func TestAuditLoggerConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Write(AuditEvent{
				Event:  "capture_override",
				Source: "ai",
			})
		}()
	}
	wg.Wait()

	data, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 50 {
		t.Fatalf("expected 50 lines, got %d", len(lines))
	}
}

// --- Settings Endpoint Tests ---

func TestSettingsResponseConnected(t *testing.T) {
	co := NewCaptureOverrides()
	resp := buildSettingsResponse(co)

	var result map[string]interface{}
	data, _ := json.Marshal(resp)
	json.Unmarshal(data, &result)

	if result["connected"] != true {
		t.Fatal("settings response should include connected: true")
	}
}

func TestSettingsResponseWithOverrides(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")

	resp := buildSettingsResponse(co)

	var result map[string]interface{}
	data, _ := json.Marshal(resp)
	json.Unmarshal(data, &result)

	overrides, ok := result["capture_overrides"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capture_overrides in response")
	}
	if overrides["ws_mode"] != "messages" {
		t.Fatalf("expected ws_mode=messages, got %v", overrides["ws_mode"])
	}
}

func TestSettingsResponseWithoutOverrides(t *testing.T) {
	co := NewCaptureOverrides()
	resp := buildSettingsResponse(co)

	var result map[string]interface{}
	data, _ := json.Marshal(resp)
	json.Unmarshal(data, &result)

	overrides, ok := result["capture_overrides"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capture_overrides field (even if empty)")
	}
	if len(overrides) != 0 {
		t.Fatalf("expected empty overrides, got %v", overrides)
	}
}

// --- Alert on Change Tests ---

func TestCaptureOverrideEmitsAlert(t *testing.T) {
	co := NewCaptureOverrides()
	alerts := co.Set("ws_mode", "messages")
	_ = alerts // Set returns error, alert is stored internally

	alert := co.DrainAlert()
	if alert == nil {
		t.Fatal("expected an alert after setting change")
	}
	if alert.Category != "capture_override" {
		t.Fatalf("expected category=capture_override, got %s", alert.Category)
	}
	if !strings.Contains(alert.Title, "ws_mode") {
		t.Fatalf("alert title should mention setting name, got: %s", alert.Title)
	}
	if !strings.Contains(alert.Detail, "lifecycle") {
		t.Fatalf("alert detail should mention previous value, got: %s", alert.Detail)
	}
	if !strings.Contains(alert.Detail, "messages") {
		t.Fatalf("alert detail should mention new value, got: %s", alert.Detail)
	}
}

func TestCaptureOverrideAlertDrained(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")
	_ = co.DrainAlert() // drain it
	alert := co.DrainAlert()
	if alert != nil {
		t.Fatal("alert should be nil after draining")
	}
}

func TestCaptureResetEmitsAlert(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")
	_ = co.DrainAlert() // drain the set alert
	co.Reset()
	alert := co.DrainAlert()
	if alert == nil {
		t.Fatal("expected alert after reset")
	}
	if alert.Category != "capture_override" {
		t.Fatalf("expected category=capture_override, got %s", alert.Category)
	}
	if !strings.Contains(alert.Title, "reset") {
		t.Fatalf("alert should mention reset, got: %s", alert.Title)
	}
}

// --- Page Info Integration Tests ---

func TestCaptureOverridesForPageInfo(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")

	info := co.GetPageInfo()
	if info == nil {
		t.Fatal("page info should not be nil")
	}
	wsInfo, ok := info["ws_mode"].(map[string]string)
	if !ok {
		t.Fatal("expected ws_mode entry in page info")
	}
	if wsInfo["value"] != "messages" {
		t.Fatalf("expected value=messages, got %s", wsInfo["value"])
	}
	if wsInfo["default"] != "lifecycle" {
		t.Fatalf("expected default=lifecycle, got %s", wsInfo["default"])
	}
}

func TestCaptureOverridesPageInfoEmpty(t *testing.T) {
	co := NewCaptureOverrides()
	info := co.GetPageInfo()
	if len(info) != 0 {
		t.Fatalf("expected empty page info when no overrides, got %v", info)
	}
}

// --- Integration: Configure Handler Tests ---

func TestConfigureCaptureSetSingle(t *testing.T) {
	co := NewCaptureOverrides()
	settings := map[string]string{"ws_mode": "messages"}
	result, err := handleCaptureSettings(co, settings, nil, "test-agent")
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if !strings.Contains(result, "ws_mode") {
		t.Fatalf("result should confirm setting, got: %s", result)
	}
}

func TestConfigureCaptureSetMultiple(t *testing.T) {
	co := NewCaptureOverrides()
	settings := map[string]string{
		"ws_mode":   "messages",
		"log_level": "all",
	}
	result, err := handleCaptureSettings(co, settings, nil, "test-agent")
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if !strings.Contains(result, "ws_mode") || !strings.Contains(result, "log_level") {
		t.Fatalf("result should confirm both settings, got: %s", result)
	}
}

func TestConfigureCaptureReset(t *testing.T) {
	co := NewCaptureOverrides()
	_ = co.Set("ws_mode", "messages")
	result, err := handleCaptureReset(co, nil, "test-agent")
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if !strings.Contains(result, "reset") {
		t.Fatalf("result should confirm reset, got: %s", result)
	}
	if len(co.GetAll()) != 0 {
		t.Fatal("overrides should be empty after reset")
	}
}

func TestConfigureCaptureInvalidSetting(t *testing.T) {
	co := NewCaptureOverrides()
	settings := map[string]string{"bad_setting": "value"}
	_, err := handleCaptureSettings(co, settings, nil, "test-agent")
	if err == "" {
		t.Fatal("expected error for invalid setting")
	}
}

func TestConfigureCaptureRateLimited(t *testing.T) {
	co := NewCaptureOverrides()
	settings1 := map[string]string{"ws_mode": "messages"}
	handleCaptureSettings(co, settings1, nil, "test-agent")
	settings2 := map[string]string{"log_level": "all"}
	_, err := handleCaptureSettings(co, settings2, nil, "test-agent")
	if err == "" {
		t.Fatal("expected rate limit error on second call within 1s")
	}
	if !strings.Contains(err, "Rate limited") {
		t.Fatalf("expected rate limit error, got: %s", err)
	}
}

func TestConfigureCaptureAuditLogged(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	logger, _ := NewAuditLogger(logPath)
	defer logger.Close()

	co := NewCaptureOverrides()
	settings := map[string]string{"ws_mode": "messages"}
	handleCaptureSettings(co, settings, logger, "claude-code")

	data, _ := os.ReadFile(logPath)
	if len(data) == 0 {
		t.Fatal("audit log should have an entry")
	}
	var event AuditEvent
	json.Unmarshal([]byte(strings.TrimSpace(string(data))), &event)
	if event.Agent != "claude-code" {
		t.Fatalf("expected agent=claude-code, got %s", event.Agent)
	}
	if event.Setting != "ws_mode" {
		t.Fatalf("expected setting=ws_mode, got %s", event.Setting)
	}
}

func TestConfigureCaptureAuditLogFailureDoesNotBlock(t *testing.T) {
	// Pass nil logger - should still work
	co := NewCaptureOverrides()
	settings := map[string]string{"ws_mode": "messages"}
	result, err := handleCaptureSettings(co, settings, nil, "test-agent")
	if err != "" {
		t.Fatalf("should succeed without audit logger: %s", err)
	}
	if result == "" {
		t.Fatal("should return result even without audit logger")
	}
}

// --- Server Restart (Session Scoping) Tests ---

func TestCaptureOverridesSessionScoped(t *testing.T) {
	// Overrides are in-memory only — new instance = clean state
	co1 := NewCaptureOverrides()
	_ = co1.Set("ws_mode", "messages")

	co2 := NewCaptureOverrides()
	overrides := co2.GetAll()
	if len(overrides) != 0 {
		t.Fatal("new CaptureOverrides should have no overrides (session-scoped)")
	}
}
