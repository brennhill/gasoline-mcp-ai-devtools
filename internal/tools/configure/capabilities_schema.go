package configure

// inferDispatchParam selects the canonical mode/action parameter for a tool.
// Primary source is schema.required[0]. For alias-friendly schemas that use
// anyOf/oneOf instead of a top-level required field, fall back to the first
// anyOf/oneOf branch whose required param has an enum in props.
func inferDispatchParam(inputSchema map[string]any) string {
	props, _ := inputSchema["properties"].(map[string]any)
	required := toStringSlice(inputSchema["required"])
	if len(required) > 0 {
		return required[0]
	}
	// Defensive fallback: schemas must not use top-level anyOf/oneOf (invariant
	// enforced by TestAllToolSchemas_NoTopLevelCombiners), but external tool
	// schemas passed to BuildCapabilitiesMap/BuildCapabilitiesSummary may predate
	// that constraint. Return the first branch whose required param has an enum.
	for _, combinerKey := range []string{"anyOf", "oneOf"} {
		combinerRaw, ok := inputSchema[combinerKey]
		if !ok {
			continue
		}
		var branches []map[string]any
		switch v := combinerRaw.(type) {
		case []map[string]any:
			branches = v
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					branches = append(branches, m)
				}
			}
		}
		for _, branch := range branches {
			branchRequired := toStringSlice(branch["required"])
			if len(branchRequired) == 0 {
				continue
			}
			candidate := branchRequired[0]
			if prop, ok := props[candidate].(map[string]any); ok {
				if _, hasEnum := prop["enum"]; hasEnum {
					return candidate
				}
			}
		}
	}
	return ""
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

// toStringSlice converts a raw schema value to a string slice.
// Handles both []string (Go-constructed schemas) and []any (JSON-unmarshaled schemas).
// Empty strings are silently dropped — schema fields must not contain blank entries.
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
