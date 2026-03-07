// map_keys.go — Generic sorted-map-keys utility for handler registry validation.

package main

import (
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// sortedMapKeys returns a sorted, comma-separated list of keys from a string-keyed map.
func sortedMapKeys[T any](m map[string]T) string {
	return strings.Join(util.SortedMapKeys(m), ", ")
}
