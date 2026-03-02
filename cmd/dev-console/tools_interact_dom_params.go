// Purpose: Defines shared DOM primitive request parameter structures and arg helpers.
// Why: Keeps transport parsing/rewriting semantics centralized across DOM primitive handlers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import "encoding/json"

type domPrimitiveParams struct {
	Selector      string   `json:"selector"`
	ScopeSelector string   `json:"scope_selector,omitempty"`
	ElementID     string   `json:"element_id,omitempty"`
	Index         *int     `json:"index,omitempty"`
	Nth           *int     `json:"nth,omitempty"`
	IndexGen      string   `json:"index_generation,omitempty"`
	Text          string   `json:"text,omitempty"`
	Value         string   `json:"value,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	Clear         bool     `json:"clear,omitempty"`
	Checked       *bool    `json:"checked,omitempty"`
	Name          string   `json:"name,omitempty"`
	TimeoutMs     int      `json:"timeout_ms,omitempty"`
	TabID         int      `json:"tab_id,omitempty"`
	Analyze       bool     `json:"analyze,omitempty"`
	X             *float64 `json:"x,omitempty"`
	Y             *float64 `json:"y,omitempty"`
	URLContains   string   `json:"url_contains,omitempty"`
	Absent        bool     `json:"absent,omitempty"`
}

type hardwareClickParams struct {
	X     *float64 `json:"x"`
	Y     *float64 `json:"y"`
	TabID int      `json:"tab_id,omitempty"`
}

var domSelectorOptionalActions = map[string]struct{}{
	"open_composer":          {},
	"submit_active_composer": {},
	"confirm_top_dialog":     {},
	"dismiss_top_overlay":    {},
	"auto_dismiss_overlays":  {},
	"wait_for_stable":        {},
	"key_press":              {},
	"wait_for":               {},
}

func parseDOMPrimitiveParams(args json.RawMessage) (domPrimitiveParams, error) {
	var params domPrimitiveParams
	if err := json.Unmarshal(args, &params); err != nil {
		return domPrimitiveParams{}, err
	}
	return params, nil
}

func parseHardwareClickParams(args json.RawMessage) (hardwareClickParams, error) {
	var params hardwareClickParams
	if err := json.Unmarshal(args, &params); err != nil {
		return hardwareClickParams{}, err
	}
	return params, nil
}

func updateArgsSelector(args json.RawMessage, selector string) json.RawMessage {
	var rawArgs map[string]json.RawMessage
	if json.Unmarshal(args, &rawArgs) != nil {
		return args
	}
	selectorJSON, err := json.Marshal(selector)
	if err != nil {
		return args
	}
	rawArgs["selector"] = selectorJSON
	updated, err := json.Marshal(rawArgs)
	if err != nil {
		return args
	}
	return updated
}
