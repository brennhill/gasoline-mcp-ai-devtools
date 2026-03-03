// tool_dispatch_helpers.go — Shared alias-resolution and mode-list helpers for tool dispatch.

package main

import (
	"encoding/json"
)

// modeAlias defines a deprecated parameter that can substitute for the canonical 'what' param.
//
// ConflictFn gates the conflict check: when set, a conflict is only raised if ConflictFn returns true.
// This supports tools where a param like "action" doubles as both a mode selector and a sub-action
// field — conflicts are only flagged when the alias value is a known top-level mode.
//
// FallbackFn gates the fallback: when set, the alias value is only used as a mode selector when
// FallbackFn returns true. When nil, any non-empty alias value is accepted as a fallback.
type modeAlias struct {
	JSONField  string           // JSON field name in args (e.g. "action", "mode", "format")
	ConflictFn func(string) bool // Optional: only raise conflict when this returns true
	FallbackFn func(string) bool // Optional: only use as fallback mode when this returns true
}

// modeResolution bundles context needed for mode resolution error messages.
type modeResolution struct {
	ToolName   string            // For error messages (e.g. "observe", "analyze")
	ValidModes string            // Sorted comma-separated list for hints
	Aliases    map[string]string // Mode aliases (e.g. "network" -> "network_waterfall")
}

// resolveToolMode extracts and resolves the 'what' parameter from args, checking alias params
// for fallback values. Returns the resolved mode, which alias param was used (empty if canonical),
// and an error response if resolution fails.
//
// Resolution order:
//  1. Parse 'what' and all alias params from args.
//  2. Detect conflicts: if 'what' is set and an alias has a different value, return conflict error.
//  3. Fall back to aliases in order if 'what' is empty.
//  4. Return missing-param error if no mode found.
//  5. Apply mode aliases (e.g. "network" -> "network_waterfall").
func resolveToolMode(
	req JSONRPCRequest,
	args json.RawMessage,
	aliasDefs []modeAlias,
	res modeResolution,
) (what string, usedAliasParam string, errResp *JSONRPCResponse) {

	// Parse all potential mode fields into a map.
	fields := make(map[string]string, len(aliasDefs)+1)
	if len(args) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
			return "", "", &resp
		}
		for _, key := range append([]string{"what"}, aliasFieldNames(aliasDefs)...) {
			if v, ok := raw[key]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					fields[key] = s
				}
			}
		}
	}

	what = fields["what"]

	// Check for conflicts: what is set and an alias has a different value.
	for _, ad := range aliasDefs {
		aliasVal := fields[ad.JSONField]
		if aliasVal == "" || aliasVal == what || what == "" {
			continue
		}
		if ad.ConflictFn != nil && !ad.ConflictFn(aliasVal) {
			continue
		}
		resp := whatAliasConflictResponse(req, ad.JSONField, what, aliasVal, res.ValidModes)
		return "", "", &resp
	}

	// Fall back to alias params in order.
	if what == "" {
		for _, ad := range aliasDefs {
			aliasVal := fields[ad.JSONField]
			if aliasVal == "" {
				continue
			}
			if ad.FallbackFn != nil && !ad.FallbackFn(aliasVal) {
				continue
			}
			what = aliasVal
			usedAliasParam = ad.JSONField
			break
		}
	}

	// Missing mode.
	if what == "" {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"Required parameter 'what' is missing",
			"Add the 'what' parameter and call again",
			withParam("what"),
			withHint("Valid values: "+res.ValidModes),
		)}
		return "", usedAliasParam, &resp
	}

	// Apply mode aliases (e.g. "network" -> "network_waterfall").
	if res.Aliases != nil {
		if canonical, ok := res.Aliases[what]; ok {
			what = canonical
		}
	}

	return what, usedAliasParam, nil
}

// aliasFieldNames extracts JSON field names from alias definitions.
func aliasFieldNames(defs []modeAlias) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.JSONField
	}
	return names
}
