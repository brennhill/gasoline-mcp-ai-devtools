// Purpose: Parses and validates computed styles query arguments for the analyze tool.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import (
	"encoding/json"
	"errors"
)

// ComputedStylesArgs holds parsed arguments for computed styles queries.
type ComputedStylesArgs struct {
	Selector   string   `json:"selector"`
	Properties []string `json:"properties,omitempty"`
	Frame      string   `json:"frame,omitempty"`
	TabID      int      `json:"tab_id,omitempty"`
}

// ParseComputedStylesArgs validates and parses computed styles arguments.
func ParseComputedStylesArgs(args json.RawMessage) (*ComputedStylesArgs, error) {
	params, err := parseAnalyzeArgs[ComputedStylesArgs](args)
	if err != nil {
		return nil, err
	}
	if params.Selector == "" {
		return nil, errors.New("required parameter 'selector' is missing")
	}
	return params, nil
}
