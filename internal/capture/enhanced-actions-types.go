// Purpose: Defines capture-side enhanced action payload structures and filtering contracts.
// Why: Standardizes action telemetry shape for replay, test generation, and observe query consumers.
// Docs: docs/features/feature/normalized-event-schema/index.md

package capture

// EnhancedAction represents a captured user action with multi-strategy selectors.
// Wire fields: see WireEnhancedAction in internal/types/wire_enhanced_action.go
type EnhancedAction struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	URL       string `json:"url,omitempty"`
	// any: Selectors map contains multiple selector strategies (css, xpath, text, testId, etc.)
	// with string values, but some strategies have nested objects (e.g., aria-label with role)
	Selectors     map[string]any `json:"selectors,omitempty"`
	Value         string         `json:"value,omitempty"`
	InputType     string         `json:"input_type,omitempty"`
	Key           string         `json:"key,omitempty"`
	FromURL       string         `json:"from_url,omitempty"`
	ToURL         string         `json:"to_url,omitempty"`
	SelectedValue string         `json:"selected_value,omitempty"`
	SelectedText  string         `json:"selected_text,omitempty"`
	ScrollY       int            `json:"scroll_y,omitempty"`
	TabId         int            `json:"tab_id,omitempty"`   // Chrome tab ID that produced this action
	TestIDs       []string       `json:"test_ids,omitempty"` // Test IDs this action belongs to (for test boundary correlation)
	Source        string         `json:"source,omitempty"`   // "human" for user actions, "ai" for AI-driven actions via interact tool
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
	TestID    string // If set, filter actions where TestID is in action's TestIDs array
}
