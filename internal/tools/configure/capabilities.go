// capabilities.go — Pure functions for building machine-readable tool capability maps.
package configure

import (
	"regexp"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/mcp"
)

type modeParamSpec struct {
	Required []string
	Optional []string
}

var (
	defaultParenPattern = regexp.MustCompile(`(?i)\(default[:\s]*([^)]+)\)`)
	defaultsToPattern   = regexp.MustCompile(`(?i)defaults?\s+to\s+([a-zA-Z0-9_./:-]+)`)
)

var configureModeSpecs = map[string]modeParamSpec{
	"store": {
		Optional: []string{"store_action", "namespace", "key", "data", "value"},
	},
	"load": {},
	"noise_rule": {
		Optional: []string{
			"noise_action", "rules", "rule_id", "pattern", "category", "classification",
			"message_regex", "source_regex", "url_regex", "method", "status_min", "status_max", "level", "reason",
		},
	},
	"clear": {
		Optional: []string{"buffer"},
	},
	"health":   {},
	"tutorial": {},
	"examples": {},
	"help": {
		Optional: []string{"tool", "mode"},
	},
	"cheatsheet": {
		Optional: []string{"tool", "mode"},
	},
	"streaming": {
		Optional: []string{"streaming_action", "events", "throttle_seconds", "severity_min"},
	},
	"test_boundary_start": {
		Required: []string{"test_id"},
		Optional: []string{"label"},
	},
	"test_boundary_end": {
		Required: []string{"test_id"},
	},
	"recording_start": {
		Optional: []string{"name", "tab_id", "sensitive_data_enabled"},
	},
	"recording_stop": {
		Optional: []string{"recording_id"},
	},
	"playback": {
		Optional: []string{"recording_id"},
	},
	"log_diff": {
		Optional: []string{"original_id", "replay_id"},
	},
	"telemetry": {
		Optional: []string{"telemetry_mode"},
	},
	"describe_capabilities": {},
	"diff_sessions": {
		Optional: []string{"verif_session_action", "name", "compare_a", "compare_b", "url"},
	},
	"audit_log": {
		Optional: []string{"operation", "audit_session_id", "tool_name", "since", "limit"},
	},
	"restart": {},
	"save_sequence": {
		Required: []string{"name"},
		Optional: []string{"description", "steps", "tags"},
	},
	"get_sequence": {
		Required: []string{"name"},
	},
	"list_sequences": {},
	"delete_sequence": {
		Required: []string{"name"},
	},
	"replay_sequence": {
		Required: []string{"name"},
		Optional: []string{"name", "override_steps", "step_timeout_ms", "continue_on_error", "stop_after_step"},
	},
	"doctor": {},
}

var observeModeSpecs = map[string]modeParamSpec{
	"command_result": {
		Required: []string{"correlation_id"},
	},
	"recording_actions": {
		Required: []string{"recording_id"},
	},
	"playback_results": {
		Required: []string{"recording_id"},
	},
	"log_diff_report": {
		Required: []string{"original_id", "replay_id"},
	},
}

var analyzeModeSpecs = map[string]modeParamSpec{
	"dom": {
		Required: []string{"selector"},
	},
	"annotation_detail": {
		Required: []string{"correlation_id"},
	},
	"draw_session": {
		Required: []string{"file"},
	},
	"computed_styles": {
		Required: []string{"selector"},
	},
	"link_validation": {
		Required: []string{"urls"},
	},
	"visual_baseline": {
		Required: []string{"name"},
	},
	"visual_diff": {
		Required: []string{"baseline"},
	},
}

var interactModeSpecs = map[string]modeParamSpec{
	"highlight": {
		Required: []string{"selector"},
	},
	"execute_js": {
		Required: []string{"script"},
	},
	"navigate": {
		Required: []string{"url"},
	},
	"new_tab": {
		Required: []string{"url"},
	},
	"subtitle": {
		Required: []string{"text"},
	},
	"save_state": {
		Required: []string{"snapshot_name"},
	},
	"load_state": {
		Required: []string{"snapshot_name"},
	},
	"delete_state": {
		Required: []string{"snapshot_name"},
	},
	"set_storage": {
		Required: []string{"key", "value"},
	},
	"delete_storage": {
		Required: []string{"key"},
	},
	"set_cookie": {
		Required: []string{"name", "value"},
	},
	"delete_cookie": {
		Required: []string{"name"},
	},
	"upload": {
		Required: []string{"file_path"},
	},
	"navigate_and_wait_for": {
		Required: []string{"url", "wait_for"},
	},
	"fill_form_and_submit": {
		Required: []string{"fields"},
	},
	"fill_form": {
		Required: []string{"fields"},
	},
}

var toolModeSpecs = map[string]map[string]modeParamSpec{
	"observe":  observeModeSpecs,
	"analyze":  analyzeModeSpecs,
	"interact": interactModeSpecs,
}

// BuildCapabilitiesMap transforms tool schemas into machine-readable capability metadata.
// It preserves legacy fields (dispatch_param, modes, params, description) and adds:
// - param_details: per-parameter type/enum/default metadata
// - mode_params: per-mode required/optional params and metadata
func BuildCapabilitiesMap(tools []mcp.MCPTool) map[string]any {
	toolsMap := make(map[string]any, len(tools))
	for _, tool := range tools {
		props, _ := tool.InputSchema["properties"].(map[string]any)
		required := toStringSlice(tool.InputSchema["required"])

		dispatchParam := ""
		if len(required) > 0 {
			dispatchParam = required[0]
		}

		modes := extractModes(dispatchParam, props)

		paramNames := make([]string, 0, len(props))
		for name := range props {
			if name != dispatchParam {
				paramNames = append(paramNames, name)
			}
		}
		sort.Strings(paramNames)

		paramDetails := buildParamDetails(props)
		modeParams := buildModeParams(tool.Name, modes, dispatchParam, paramNames, paramDetails)

		toolsMap[tool.Name] = map[string]any{
			"dispatch_param": dispatchParam,
			"modes":          modes,
			"params":         paramNames,
			"param_details":  paramDetails,
			"mode_params":    modeParams,
			"description":    tool.Description,
		}
	}
	return toolsMap
}

func extractModes(dispatchParam string, props map[string]any) []string {
	if dispatchParam == "" {
		return nil
	}
	prop, ok := props[dispatchParam].(map[string]any)
	if !ok {
		return nil
	}
	return toStringSlice(prop["enum"])
}

func buildParamDetails(props map[string]any) map[string]any {
	details := make(map[string]any, len(props))
	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]any)
		if !ok {
			continue
		}
		meta := map[string]any{}

		if typ, ok := prop["type"].(string); ok && typ != "" {
			meta["type"] = typ
		}

		if enumVals := toStringSlice(prop["enum"]); len(enumVals) > 0 {
			meta["enum"] = enumVals
		}

		if desc, ok := prop["description"].(string); ok && desc != "" {
			meta["description"] = desc
			if _, hasDefault := meta["default"]; !hasDefault {
				if parsedDefault, ok := extractDefaultFromDescription(desc); ok {
					meta["default"] = parsedDefault
				}
			}
		}

		if explicitDefault, ok := prop["default"]; ok {
			meta["default"] = explicitDefault
		}

		if items, ok := prop["items"].(map[string]any); ok {
			if itemType, ok := items["type"].(string); ok && itemType != "" {
				meta["item_type"] = itemType
			}
		}

		if len(meta) > 0 {
			details[name] = meta
		}
	}
	return details
}

func extractDefaultFromDescription(description string) (string, bool) {
	if description == "" {
		return "", false
	}
	if match := defaultParenPattern.FindStringSubmatch(description); len(match) == 2 {
		return cleanDefaultText(match[1]), true
	}
	if match := defaultsToPattern.FindStringSubmatch(description); len(match) == 2 {
		return cleanDefaultText(match[1]), true
	}
	return "", false
}

func cleanDefaultText(v string) string {
	trimmed := strings.TrimSpace(v)
	trimmed = strings.Trim(trimmed, "`'\"")
	trimmed = strings.TrimRight(trimmed, ".,;")
	return trimmed
}

func buildModeParams(
	toolName string,
	modes []string,
	dispatchParam string,
	paramNames []string,
	paramDetails map[string]any,
) map[string]any {
	if len(modes) == 0 {
		return map[string]any{}
	}

	modeParams := make(map[string]any, len(modes))
	for _, mode := range modes {
		spec := modeParamSpec{
			Required: nil,
			Optional: append([]string(nil), paramNames...),
		}
		if dispatchParam != "" {
			spec.Required = append(spec.Required, dispatchParam)
		}

		if toolName == "configure" {
			if configureSpec, ok := configureModeSpecs[mode]; ok {
				spec = modeParamSpec{
					Required: append([]string{}, configureSpec.Required...),
					Optional: append([]string{}, configureSpec.Optional...),
				}
				if dispatchParam != "" && !containsString(spec.Required, dispatchParam) {
					spec.Required = append([]string{dispatchParam}, spec.Required...)
				}
			}
		} else if modeSpecs, ok := toolModeSpecs[toolName]; ok {
			if modeSpec, ok := modeSpecs[mode]; ok {
				spec.Required = append(spec.Required, modeSpec.Required...)
				if modeSpec.Optional != nil {
					spec.Optional = append([]string{}, modeSpec.Optional...)
				}
			}
		}

		spec.Required = filterKnownParams(spec.Required, paramDetails)
		spec.Optional = filterKnownParams(spec.Optional, paramDetails)
		spec.Optional = removeAll(spec.Optional, spec.Required)
		sort.Strings(spec.Required)
		sort.Strings(spec.Optional)

		params := make(map[string]any)
		for _, name := range append(append([]string{}, spec.Required...), spec.Optional...) {
			if metaRaw, ok := paramDetails[name]; ok {
				if meta, ok := metaRaw.(map[string]any); ok {
					params[name] = cloneAnyMap(meta)
				}
			}
		}

		applyConfigureModeDefaults(toolName, mode, params)

		modeParams[mode] = map[string]any{
			"required": spec.Required,
			"optional": spec.Optional,
			"params":   params,
		}
	}

	return modeParams
}

func applyConfigureModeDefaults(toolName, mode string, params map[string]any) {
	if toolName != "configure" {
		return
	}

	switch mode {
	case "store":
		ensureParamDefault(params, "store_action", "list")
	case "noise_rule":
		ensureParamDefault(params, "noise_action", "list")
	}
}

func ensureParamDefault(params map[string]any, name, defaultValue string) {
	metaRaw, ok := params[name]
	if !ok {
		return
	}
	meta, ok := metaRaw.(map[string]any)
	if !ok {
		return
	}
	if _, exists := meta["default"]; !exists {
		meta["default"] = defaultValue
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func filterKnownParams(names []string, details map[string]any) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := details[name]; ok {
			out = append(out, name)
		}
	}
	return dedupeStrings(out)
}

func removeAll(input []string, blocked []string) []string {
	blockedSet := make(map[string]bool, len(blocked))
	for _, name := range blocked {
		blockedSet[name] = true
	}
	out := make([]string, 0, len(input))
	for _, name := range input {
		if !blockedSet[name] {
			out = append(out, name)
		}
	}
	return dedupeStrings(out)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toStringSlice(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
