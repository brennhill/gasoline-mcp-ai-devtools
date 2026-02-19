// rewrite.go — Pure argument rewriting functions for configure sub-handlers.
// These functions normalize composite tool parameters before dispatch.
package configure

import "encoding/json"

// RewriteNoiseRuleArgs rewrites noise_action to action in the raw argument map.
// If noise_action is empty or missing, it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteNoiseRuleArgs(args json.RawMessage) (json.RawMessage, error) {
	var compositeArgs struct {
		NoiseAction string `json:"noise_action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return nil, err
		}
	}

	var rawMap map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawMap); err != nil {
			return nil, err
		}
	}
	if rawMap == nil {
		rawMap = make(map[string]any)
	}
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	// Error impossible: rawMap contains only primitive types and strings from input
	rewritten, _ := json.Marshal(rawMap)
	return rewritten, nil
}

// RewriteStreamingArgs rewrites streaming_action to action in the raw argument map.
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteStreamingArgs(args json.RawMessage) (json.RawMessage, error) {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, err
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	if sa, ok := raw["streaming_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return rewritten, nil
}

// RewriteDiffSessionsArgs rewrites verif_session_action to action in the raw argument map.
// If the resulting action is empty or "diff_sessions", it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteDiffSessionsArgs(args json.RawMessage) (json.RawMessage, error) {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, err
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	if sa, ok := raw["verif_session_action"].(string); ok && sa != "" {
		raw["action"] = sa
	}

	// configure(action:"diff_sessions") is the tool entrypoint; default to list
	// unless a specific verif_session_action is provided.
	if action, _ := raw["action"].(string); action == "" || action == "diff_sessions" {
		raw["action"] = "list"
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return rewritten, nil
}
