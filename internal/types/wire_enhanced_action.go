// wire_enhanced_action.go — Wire type for enhanced actions over HTTP.
// WireEnhancedAction defines the JSON fields sent by the extension.
// Changes here MUST be mirrored in src/types/wire-enhanced-action.ts.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
package types

// WireEnhancedAction is the canonical wire format for enhanced actions.
// Extension sends these fields; the Go daemon may add server-only enrichment.
type WireEnhancedAction struct {
	Type          string         `json:"type"`
	Timestamp     int64          `json:"timestamp"`
	URL           string         `json:"url,omitempty"`
	Selectors     map[string]any `json:"selectors,omitempty"` // any: multiple selector strategies with varying value types
	Value         string         `json:"value,omitempty"`
	InputType     string         `json:"input_type,omitempty"`
	Key           string         `json:"key,omitempty"`
	FromURL       string         `json:"from_url,omitempty"`
	ToURL         string         `json:"to_url,omitempty"`
	SelectedValue string         `json:"selected_value,omitempty"`
	SelectedText  string         `json:"selected_text,omitempty"`
	ScrollY       int            `json:"scroll_y,omitempty"`
	TabId         int            `json:"tab_id,omitempty"`
}
