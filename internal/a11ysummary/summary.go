// Purpose: Normalizes accessibility summary counts across extension and server naming styles.
// Why: Keeps a11y summary payloads semantically consistent while preserving backward compatibility.
// Docs: docs/features/feature/enhanced-wcag-audit/index.md

package a11ysummary

import (
	"encoding/json"
	"strconv"
)

// Counts captures accessibility summary totals.
type Counts struct {
	Violations   int
	Passes       int
	Incomplete   int
	Inapplicable int
}

// BuildSummary returns a normalized summary map that includes canonical and legacy keys.
func BuildSummary(counts Counts) map[string]any {
	return map[string]any{
		// Canonical keys
		"violations":   counts.Violations,
		"passes":       counts.Passes,
		"incomplete":   counts.Incomplete,
		"inapplicable": counts.Inapplicable,
		// Legacy aliases (backward compatibility)
		"violation_count":    counts.Violations,
		"pass_count":         counts.Passes,
		"incomplete_count":   counts.Incomplete,
		"inapplicable_count": counts.Inapplicable,
	}
}

// countsFromAuditResult derives counts from top-level a11y arrays.
func countsFromAuditResult(auditResult map[string]any) Counts {
	return Counts{
		Violations:   arrayLen(auditResult["violations"]),
		Passes:       arrayLen(auditResult["passes"]),
		Incomplete:   arrayLen(auditResult["incomplete"]),
		Inapplicable: arrayLen(auditResult["inapplicable"]),
	}
}

// EnsureAuditSummary adds or normalizes the "summary" field in an a11y result payload.
// It keeps canonical and legacy keys in sync to avoid downstream contract drift.
func EnsureAuditSummary(auditResult map[string]any) {
	if auditResult == nil {
		return
	}
	fallback := countsFromAuditResult(auditResult)

	rawSummary, ok := auditResult["summary"]
	if !ok {
		auditResult["summary"] = BuildSummary(fallback)
		return
	}

	summaryMap, ok := rawSummary.(map[string]any)
	if !ok {
		auditResult["summary"] = BuildSummary(fallback)
		return
	}

	violations := pickCount(summaryMap["violations"], summaryMap["violation_count"], fallback.Violations)
	passes := pickCount(summaryMap["passes"], summaryMap["pass_count"], fallback.Passes)
	incomplete := pickCount(summaryMap["incomplete"], summaryMap["incomplete_count"], fallback.Incomplete)
	inapplicable := pickCount(summaryMap["inapplicable"], summaryMap["inapplicable_count"], fallback.Inapplicable)

	normalized := make(map[string]any, len(summaryMap)+8)
	for k, v := range summaryMap {
		normalized[k] = v
	}

	normalized["violations"] = violations
	normalized["passes"] = passes
	normalized["incomplete"] = incomplete
	normalized["inapplicable"] = inapplicable
	normalized["violation_count"] = violations
	normalized["pass_count"] = passes
	normalized["incomplete_count"] = incomplete
	normalized["inapplicable_count"] = inapplicable

	auditResult["summary"] = normalized
}

func arrayLen(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func pickCount(primary any, fallback any, defaultValue int) int {
	if v, ok := parseCount(primary); ok {
		return v
	}
	if v, ok := parseCount(fallback); ok {
		return v
	}
	return defaultValue
}

func parseCount(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if parsed, err := strconv.Atoi(v.String()); err == nil {
			return parsed, true
		}
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
