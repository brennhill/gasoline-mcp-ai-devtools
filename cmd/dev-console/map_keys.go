// map_keys.go — Generic sorted-map-keys utility for handler registry validation.

package main

import (
	"sort"
	"strings"
)

// sortedMapKeys returns a sorted, comma-separated list of keys from a string-keyed map.
func sortedMapKeys[T any](m map[string]T) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
