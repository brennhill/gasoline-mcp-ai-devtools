// status_test.go — Tests for ExtensionStatus types and UpdateExtensionStatus method.
package capture

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================
// ExtensionStatus JSON Serialization Tests
// ============================================

func TestNewExtensionStatus_JSONFieldNames(t *testing.T) {
	t.Parallel()

	status := ExtensionStatus{
		Type:               "tracking_update",
		TrackingEnabled:    true,
		TrackedTabID:       42,
		TrackedTabURL:      "https://example.com/page",
		Message:            "Tab tracking enabled",
		ExtensionConnected: true,
		Timestamp:          "2024-01-15T10:30:00Z",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Verify snake_case field names
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	requiredFields := []string{
		"type",
		"tracking_enabled",
		"tracked_tab_id",
		"tracked_tab_url",
		"message",
		"extension_connected",
		"timestamp",
	}
	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("missing JSON field %q in serialized ExtensionStatus", field)
		}
	}
}

func TestNewExtensionStatus_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := ExtensionStatus{
		Type:               "tracking_update",
		TrackingEnabled:    true,
		TrackedTabID:       99,
		TrackedTabURL:      "https://app.example.com/dashboard",
		Message:            "Tracking started",
		ExtensionConnected: true,
		Timestamp:          "2024-01-15T10:30:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var restored ExtensionStatus
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if restored.Type != original.Type {
		t.Errorf("Type = %q, want %q", restored.Type, original.Type)
	}
	if restored.TrackingEnabled != original.TrackingEnabled {
		t.Errorf("TrackingEnabled = %v, want %v", restored.TrackingEnabled, original.TrackingEnabled)
	}
	if restored.TrackedTabID != original.TrackedTabID {
		t.Errorf("TrackedTabID = %d, want %d", restored.TrackedTabID, original.TrackedTabID)
	}
	if restored.TrackedTabURL != original.TrackedTabURL {
		t.Errorf("TrackedTabURL = %q, want %q", restored.TrackedTabURL, original.TrackedTabURL)
	}
	if restored.Message != original.Message {
		t.Errorf("Message = %q, want %q", restored.Message, original.Message)
	}
	if restored.ExtensionConnected != original.ExtensionConnected {
		t.Errorf("ExtensionConnected = %v, want %v", restored.ExtensionConnected, original.ExtensionConnected)
	}
	if restored.Timestamp != original.Timestamp {
		t.Errorf("Timestamp = %q, want %q", restored.Timestamp, original.Timestamp)
	}
}

func TestNewExtensionStatus_OmitEmptyMessage(t *testing.T) {
	t.Parallel()

	status := ExtensionStatus{
		Type:               "tracking_update",
		TrackingEnabled:    false,
		TrackedTabID:       0,
		TrackedTabURL:      "",
		Message:            "", // Should be omitted
		ExtensionConnected: false,
		Timestamp:          "2024-01-15T10:30:00Z",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	// Message has omitempty tag - should be absent
	if _, ok := m["message"]; ok {
		t.Error("empty Message should be omitted from JSON (omitempty)")
	}
}

// ============================================
// UpdateExtensionStatus Tests
// ============================================

func TestNewUpdateExtensionStatus_SetsTrackingState(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	status := ExtensionStatus{
		Type:               "tracking_update",
		TrackingEnabled:    true,
		TrackedTabID:       42,
		TrackedTabURL:      "https://example.com/page",
		ExtensionConnected: true,
		Timestamp:          time.Now().Format(time.RFC3339),
	}

	beforeUpdate := time.Now()
	c.UpdateExtensionStatus(status)
	afterUpdate := time.Now()

	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.ext.trackingEnabled {
		t.Error("trackingEnabled should be true after update")
	}
	if c.ext.trackedTabID != 42 {
		t.Errorf("trackedTabID = %d, want 42", c.ext.trackedTabID)
	}
	if c.ext.trackedTabURL != "https://example.com/page" {
		t.Errorf("trackedTabURL = %q, want https://example.com/page", c.ext.trackedTabURL)
	}
	if c.ext.trackingUpdated.Before(beforeUpdate) || c.ext.trackingUpdated.After(afterUpdate) {
		t.Error("trackingUpdated timestamp out of range")
	}
}

func TestNewUpdateExtensionStatus_DisablesTracking(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// First enable tracking
	c.UpdateExtensionStatus(ExtensionStatus{
		TrackingEnabled: true,
		TrackedTabID:    99,
		TrackedTabURL:   "https://example.com",
	})

	// Then disable it
	c.UpdateExtensionStatus(ExtensionStatus{
		TrackingEnabled: false,
		TrackedTabID:    0,
		TrackedTabURL:   "",
	})

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ext.trackingEnabled {
		t.Error("trackingEnabled should be false after disabling")
	}
	if c.ext.trackedTabID != 0 {
		t.Errorf("trackedTabID = %d, want 0", c.ext.trackedTabID)
	}
	if c.ext.trackedTabURL != "" {
		t.Errorf("trackedTabURL = %q, want empty", c.ext.trackedTabURL)
	}
}

func TestNewUpdateExtensionStatus_OverwritesPreviousState(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Set initial state
	c.UpdateExtensionStatus(ExtensionStatus{
		TrackingEnabled: true,
		TrackedTabID:    10,
		TrackedTabURL:   "https://first.com",
	})

	// Overwrite with new state
	c.UpdateExtensionStatus(ExtensionStatus{
		TrackingEnabled: true,
		TrackedTabID:    20,
		TrackedTabURL:   "https://second.com",
	})

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ext.trackedTabID != 20 {
		t.Errorf("trackedTabID = %d, want 20 (overwritten)", c.ext.trackedTabID)
	}
	if c.ext.trackedTabURL != "https://second.com" {
		t.Errorf("trackedTabURL = %q, want https://second.com", c.ext.trackedTabURL)
	}
}

func TestNewUpdateExtensionStatus_ZeroValues(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Update with zero-value status
	c.UpdateExtensionStatus(ExtensionStatus{})

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ext.trackingEnabled {
		t.Error("trackingEnabled should be false (zero value)")
	}
	if c.ext.trackedTabID != 0 {
		t.Errorf("trackedTabID = %d, want 0", c.ext.trackedTabID)
	}
	if c.ext.trackedTabURL != "" {
		t.Errorf("trackedTabURL = %q, want empty", c.ext.trackedTabURL)
	}
	if c.ext.trackingUpdated.IsZero() {
		t.Error("trackingUpdated should be set even with zero-value status")
	}
}

func TestNewUpdateExtensionStatus_UpdatesTimestampOnEachCall(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.UpdateExtensionStatus(ExtensionStatus{TrackingEnabled: true, TrackedTabID: 1})

	c.mu.RLock()
	firstUpdate := c.ext.trackingUpdated
	c.mu.RUnlock()

	// Small delay to ensure different timestamp
	time.Sleep(1 * time.Millisecond)

	c.UpdateExtensionStatus(ExtensionStatus{TrackingEnabled: true, TrackedTabID: 2})

	c.mu.RLock()
	secondUpdate := c.ext.trackingUpdated
	c.mu.RUnlock()

	if !secondUpdate.After(firstUpdate) {
		t.Errorf("second trackingUpdated (%v) should be after first (%v)", secondUpdate, firstUpdate)
	}
}
