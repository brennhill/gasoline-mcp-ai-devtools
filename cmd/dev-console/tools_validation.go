// tools_validation.go — Parameter validation and error checking utilities.
// Validates log entries, extracts JSON field names, and unmarshals with warnings.
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// checkLogQuality scans entries for missing expected fields and returns
// a warning note if anomalies are found. Returns "" if all entries look clean.
func checkLogQuality(entries []LogEntry) string {
	var missingTS, missingMsg, missingSource int
	badEntries := 0
	for _, e := range entries {
		entryBad := false
		if _, ok := e["ts"].(string); !ok {
			missingTS++
			entryBad = true
		}
		if _, ok := e["message"].(string); !ok {
			missingMsg++
			entryBad = true
		}
		if _, ok := e["source"].(string); !ok {
			missingSource++
			entryBad = true
		}
		if entryBad {
			badEntries++
		}
	}

	if badEntries == 0 {
		return ""
	}

	var parts []string
	if missingTS > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'ts'", missingTS))
	}
	if missingMsg > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'message'", missingMsg))
	}
	if missingSource > 0 {
		parts = append(parts, fmt.Sprintf("%d missing 'source'", missingSource))
	}
	return fmt.Sprintf("WARNING: %d/%d entries have incomplete fields (%s). This may indicate a browser extension issue or version mismatch.",
		badEntries, len(entries), strings.Join(parts, ", "))
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
