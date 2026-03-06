// Purpose: Parses and validates form discovery query arguments for the analyze tool.
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
	return parseAnalyzeArgs[FormsArgs](args)
}

// FormValidationArgs holds parsed arguments for form validation queries.
type FormValidationArgs struct {
	Selector string `json:"selector,omitempty"`
	TabID    int    `json:"tab_id,omitempty"`
}

// ParseFormValidationArgs validates and parses form validation arguments.
func ParseFormValidationArgs(args json.RawMessage) (*FormValidationArgs, error) {
	return parseAnalyzeArgs[FormValidationArgs](args)
}

// DataTableArgs holds parsed arguments for table extraction queries.
type DataTableArgs struct {
	Selector string `json:"selector,omitempty"`
	MaxRows  int    `json:"max_rows,omitempty"`
	MaxCols  int    `json:"max_cols,omitempty"`
	TabID    int    `json:"tab_id,omitempty"`
}

// ParseDataTableArgs validates and parses table extraction arguments.
func ParseDataTableArgs(args json.RawMessage) (*DataTableArgs, error) {
	return parseAnalyzeArgs[DataTableArgs](args)
}
