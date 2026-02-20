// computed_styles.go — Computed styles argument parsing for the analyze tool.
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
	var params ComputedStylesArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
	}
	if params.Selector == "" {
		return nil, errors.New("required parameter 'selector' is missing")
	}
	return &params, nil
}
