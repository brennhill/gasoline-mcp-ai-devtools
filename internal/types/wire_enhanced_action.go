// Purpose: Defines canonical wire schema for enhanced user-action payload transport.
// Why: Keeps extension-to-daemon action serialization stable and versionable.
// Docs: docs/features/feature/normalized-event-schema/index.md

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
