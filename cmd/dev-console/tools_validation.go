// tools_validation.go — Parameter validation and error checking utilities.
// General-purpose validation delegates to internal/mcp.
// Log quality checking is observe-specific and stays here.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dev-console/dev-console/internal/mcp"
)

func getJSONFieldNames(v any) map[string]bool {
	return mcp.GetJSONFieldNames(v)
}

// parseArgs unmarshals JSON args into a typed struct, returning a structured error on failure.
// Usage: params, resp, ok := parseArgs[MyParams](req, args); if !ok { return resp }
func parseArgs[T any](req JSONRPCRequest, args json.RawMessage) (T, JSONRPCResponse, bool) {
	var p T
	if err := json.Unmarshal(args, &p); err != nil {
		return p, JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again",
		)}, false
	}
	return p, JSONRPCResponse{}, true
}

func unmarshalWithWarnings(data json.RawMessage, v any) ([]string, error) {
	return mcp.UnmarshalWithWarnings(data, v)
}

func validateParamsAgainstSchema(data json.RawMessage, schema map[string]any) []string {
	return mcp.ValidateParamsAgainstSchema(data, schema)
}

// ============================================
// Log Quality Checking (observe-specific)
// ============================================

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
