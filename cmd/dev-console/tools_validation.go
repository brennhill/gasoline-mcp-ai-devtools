// Purpose: Owns tools_validation.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// tools_validation.go — Parameter validation and error checking utilities.
// Validates log entries, extracts JSON field names, and unmarshals with warnings.
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// logFieldCounts tracks missing field counts for log quality checking.
type logFieldCounts struct {
	missingTS     int
	missingMsg    int
	missingSource int
	badEntries    int
}

// checkLogQuality scans entries for missing expected fields and returns
// a warning note if anomalies are found. Returns "" if all entries look clean.
func checkLogQuality(entries []LogEntry) string {
	counts := countMissingFields(entries)
	if counts.badEntries == 0 {
		return ""
	}
	return formatQualityWarning(counts, len(entries))
}

func countMissingFields(entries []LogEntry) logFieldCounts {
	var c logFieldCounts
	for _, e := range entries {
		entryBad := false
		if _, ok := e["ts"].(string); !ok {
			c.missingTS++
			entryBad = true
		}
		if _, ok := e["message"].(string); !ok {
			c.missingMsg++
			entryBad = true
		}
		if _, ok := e["source"].(string); !ok {
			c.missingSource++
			entryBad = true
		}
		if entryBad {
			c.badEntries++
		}
	}
	return c
}

func formatQualityWarning(c logFieldCounts, total int) string {
	var parts []string
	if c.missingTS > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'ts'", c.missingTS))
	}
	if c.missingMsg > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'message'", c.missingMsg))
	}
	if c.missingSource > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'source'", c.missingSource))
	}
	return fmt.Sprintf("WARNING: %d/%d entries have incomplete fields (%s). This may indicate a browser extension issue or version mismatch.",
		c.badEntries, total, strings.Join(parts, ", "))
}

// getJSONFieldNames uses reflection to extract the set of known JSON field names
// from a struct's json tags. Fields without a json tag use their Go field name.
// Fields tagged with json:"-" are excluded.
func getJSONFieldNames(v any) map[string]bool {
	known := make(map[string]bool)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return known
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if tag == "" {
			known[field.Name] = true
			continue
		}
		// Strip options like ",omitempty"
		name := strings.Split(tag, ",")[0]
		if name != "" {
			known[name] = true
		}
	}
	return known
}

// unmarshalWithWarnings unmarshals JSON into a struct and returns warnings for
// any unknown top-level fields. This helps LLMs discover misspelled parameters.
func unmarshalWithWarnings(data json.RawMessage, v any) ([]string, error) {
	if err := json.Unmarshal(data, v); err != nil {
		return nil, err
	}
	// Check for unknown fields by unmarshaling into a map
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil // Can't check, skip warnings
	}
	known := getJSONFieldNames(v)
	var warnings []string
	for k := range raw {
		if !known[k] {
			warnings = append(warnings, fmt.Sprintf("unknown parameter '%s' (ignored)", k))
		}
	}
	return warnings, nil
}

// validateParamsAgainstSchema checks incoming JSON keys against a tool's known
// property names from its InputSchema. Returns warnings for unknown fields.
// This validates at the tool level (not handler level), catching typos across
// all parameters defined in the tool's schema.
func validateParamsAgainstSchema(data json.RawMessage, schema map[string]any) []string {
	if len(data) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	var warnings []string
	for k := range raw {
		if _, known := props[k]; !known {
			warnings = append(warnings, fmt.Sprintf("unknown parameter '%s' (ignored)", k))
		}
	}
	return warnings
}
