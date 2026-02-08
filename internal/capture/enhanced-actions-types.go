// enhanced-actions-types.go — Enhanced actions types.
// EnhancedAction represents captured user actions with multi-strategy selectors.
package capture

// EnhancedAction represents a captured user action with multi-strategy selectors
type EnhancedAction struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	URL       string `json:"url,omitempty"`
	// any: Selectors map contains multiple selector strategies (css, xpath, text, testId, etc.)
	// with string values, but some strategies have nested objects (e.g., aria-label with role)
	Selectors     map[string]any `json:"selectors,omitempty"`
	Value         string         `json:"value,omitempty"`
	InputType     string         `json:"inputType,omitempty"`
	Key           string         `json:"key,omitempty"`
	FromURL       string         `json:"fromUrl,omitempty"`
	ToURL         string         `json:"toUrl,omitempty"`
	SelectedValue string         `json:"selectedValue,omitempty"`
	SelectedText  string         `json:"selectedText,omitempty"`
	ScrollY       int            `json:"scrollY,omitempty"`
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
