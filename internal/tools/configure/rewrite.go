// Purpose: Rewrites noise_rule argument maps to normalize noise_action to the canonical action field.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import "encoding/json"

func parseRawArgsMap(args json.RawMessage) (map[string]any, error) {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, err
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	return raw, nil
}

func marshalRawArgsMap(raw map[string]any) json.RawMessage {
	// Error impossible: map contains primitive/json-compatible values from decoded input.
	rewritten, _ := json.Marshal(raw)
	return rewritten
}

func applyActionAlias(raw map[string]any, aliasField string, allowEmpty bool) {
	if sa, ok := raw[aliasField].(string); ok && (allowEmpty || sa != "") {
		raw["action"] = sa
	}
}

// RewriteNoiseRuleArgs rewrites noise_action to action in the raw argument map.
// If noise_action is empty or missing, it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteNoiseRuleArgs(args json.RawMessage) (json.RawMessage, error) {
	rawMap, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	rawMap["action"] = stringOrEmpty(rawMap["noise_action"])
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	if action, _ := rawMap["action"].(string); action == "add" {
		maybeFlattenSingleNoiseRule(rawMap)
	}
	return marshalRawArgsMap(rawMap), nil
}

func maybeFlattenSingleNoiseRule(rawMap map[string]any) {
	if rules, ok := rawMap["rules"].([]any); ok && len(rules) > 0 {
		return
	}
	rule, ok := buildFlatNoiseRule(rawMap)
	if !ok {
		return
	}
	rawMap["rules"] = []any{rule}
}

func buildFlatNoiseRule(rawMap map[string]any) (map[string]any, bool) {
	messageRegex := stringOrEmpty(rawMap["message_regex"])
	if messageRegex == "" {
		messageRegex = stringOrEmpty(rawMap["pattern"])
	}
	sourceRegex := stringOrEmpty(rawMap["source_regex"])
	urlRegex := stringOrEmpty(rawMap["url_regex"])
	method := stringOrEmpty(rawMap["method"])
	level := stringOrEmpty(rawMap["level"])

	statusMin, hasStatusMin := rawMap["status_min"]
	statusMax, hasStatusMax := rawMap["status_max"]

	if messageRegex == "" && sourceRegex == "" && urlRegex == "" && method == "" && level == "" && !hasStatusMin && !hasStatusMax {
		return nil, false
	}

	category := stringOrEmpty(rawMap["category"])
	if category == "" {
		category = "console"
	}

	matchSpec := map[string]any{}
	if messageRegex != "" {
		matchSpec["message_regex"] = messageRegex
	}
	if sourceRegex != "" {
		matchSpec["source_regex"] = sourceRegex
	}
	if urlRegex != "" {
		matchSpec["url_regex"] = urlRegex
	}
	if method != "" {
		matchSpec["method"] = method
	}
	if level != "" {
		matchSpec["level"] = level
	}
	if hasStatusMin {
		matchSpec["status_min"] = statusMin
	}
	if hasStatusMax {
		matchSpec["status_max"] = statusMax
	}

	rule := map[string]any{
		"category":   category,
		"match_spec": matchSpec,
	}
	if classification := stringOrEmpty(rawMap["classification"]); classification != "" {
		rule["classification"] = classification
	}
	return rule, true
}

func stringOrEmpty(v any) string {
	s, _ := v.(string)
	return s
}

// RewriteStreamingArgs rewrites streaming_action to action in the raw argument map.
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteStreamingArgs(args json.RawMessage) (json.RawMessage, error) {
	raw, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	applyActionAlias(raw, "streaming_action", true)
	return marshalRawArgsMap(raw), nil
}

// RewriteDiffSessionsArgs rewrites verif_session_action to action in the raw argument map.
// If the resulting action is empty or "diff_sessions", it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteDiffSessionsArgs(args json.RawMessage) (json.RawMessage, error) {
	raw, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	applyActionAlias(raw, "verif_session_action", false)

	// configure(action:"diff_sessions") is the tool entrypoint; default to list
	// unless a specific verif_session_action is provided.
	if action, _ := raw["action"].(string); action == "" || action == "diff_sessions" {
		raw["action"] = "list"
	}
	return marshalRawArgsMap(raw), nil
}
