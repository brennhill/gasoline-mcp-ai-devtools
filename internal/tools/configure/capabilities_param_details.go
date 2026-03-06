// Purpose: Extracts parameter metadata (type, enum, default, description) from tool schemas.
// Why: Separates parameter detail extraction from mode mapping and schema inference.
package configure

import (
	"regexp"
	"strings"
)

var (
	defaultParenPattern = regexp.MustCompile(`(?i)\(default[:\s]*([^)]+)\)`)
	defaultsToPattern   = regexp.MustCompile(`(?i)defaults?\s+to\s+([a-zA-Z0-9_./:-]+)`)
)

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
