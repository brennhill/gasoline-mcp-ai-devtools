// types.go — Recording types for flow recording and playback
// Captures user interactions as structured actions that can be replayed
// as Playwright tests or bug reproduction scripts.
package recording

// ============================================
// Recording Types (Flow Recording & Playback)
// ============================================

// RecordingAction represents a user action captured during recording
type RecordingAction struct {
	Type          string `json:"type"`               // "click", "type", "navigate", "screenshot"
	TimestampMs   int64  `json:"timestamp_ms"`       // Milliseconds since epoch
	URL           string `json:"url,omitempty"`      // Page URL at time of action
	Selector      string `json:"selector,omitempty"` // CSS selector for the element (data-testid preferred)
	DataTestID    string `json:"data_testid,omitempty"` // data-testid attribute value if present
	Text          string `json:"text,omitempty"`    // Text typed or "[redacted]" if sensitive_data_enabled=false
	X             int    `json:"x,omitempty"`        // X coordinate
	Y             int    `json:"y,omitempty"`        // Y coordinate
	ScreenshotPath string `json:"screenshot_path,omitempty"` // Path to screenshot file (relative to recording dir)
}

// Recording represents a captured user flow
type Recording struct {
	ID                   string             `json:"id"`                   // Format: "name-YYYYMMDDTHHMMSSZ"
	Name                 string             `json:"name"`                 // User-provided or auto-generated from page title
	CreatedAt            string             `json:"created_at"`           // ISO8601 timestamp
	StartURL             string             `json:"start_url"`            // Initial page URL
	Viewport             ViewportInfo       `json:"viewport,omitempty"`   // Viewport size at recording time
	Duration             int64              `json:"duration_ms"`          // Total duration in milliseconds
	ActionCount          int                `json:"action_count"`         // Number of actions captured
	Actions              []RecordingAction  `json:"actions"`              // Ordered list of actions
	SensitiveDataEnabled bool               `json:"sensitive_data_enabled"` // Whether to capture full text (default false)
	TestID               string             `json:"test_id,omitempty"`    // Test boundary ID if recording was part of a test
}

// ViewportInfo captures the browser viewport dimensions
type ViewportInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// RecordingMetadata is persisted to ~/.gasoline/recordings/{id}/metadata.json
type RecordingMetadata struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	CreatedAt            string            `json:"created_at"`
	StartURL             string            `json:"start_url"`
	Viewport             ViewportInfo      `json:"viewport"`
	Duration             int64             `json:"duration_ms"`
	ActionCount          int               `json:"action_count"`
	Actions              []RecordingAction `json:"actions"`
	SensitiveDataEnabled bool              `json:"sensitive_data_enabled"`
	TestID               string            `json:"test_id,omitempty"`
}
