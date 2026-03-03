// Purpose: Builds per-mode parameter maps for the describe_capabilities tool response.
// Why: Separates mode-parameter mapping from schema inference and param detail extraction.
package configure

import "sort"

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

		if toolSpecs, ok := toolModeSpecs[toolName]; ok {
			if modeSpec, ok := toolSpecs[mode]; ok {
				spec = modeParamSpec{
					Required: append([]string{}, modeSpec.Required...),
					Optional: append([]string{}, modeSpec.Optional...),
				}
				if dispatchParam != "" && !containsString(spec.Required, dispatchParam) {
					spec.Required = append([]string{dispatchParam}, spec.Required...)
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
