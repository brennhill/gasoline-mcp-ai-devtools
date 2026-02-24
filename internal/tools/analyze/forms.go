// Purpose: Provides analyze tool implementation helpers shared by command handlers.
// Why: Centralizes analyze logic to keep handler behavior consistent across command paths.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import (
	"encoding/json"
)

// FormsArgs holds parsed arguments for form discovery queries.
type FormsArgs struct {
	Selector string `json:"selector,omitempty"`
	TabID    int    `json:"tab_id,omitempty"`
}

// ParseFormsArgs validates and parses form discovery arguments.
func ParseFormsArgs(args json.RawMessage) (*FormsArgs, error) {
	var params FormsArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	return &params, nil
}

// FormValidationArgs holds parsed arguments for form validation queries.
type FormValidationArgs struct {
	Selector string `json:"selector,omitempty"`
	TabID    int    `json:"tab_id,omitempty"`
}

// ParseFormValidationArgs validates and parses form validation arguments.
func ParseFormValidationArgs(args json.RawMessage) (*FormValidationArgs, error) {
	var params FormValidationArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	return &params, nil
}
