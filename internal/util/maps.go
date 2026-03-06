// maps.go — Generic map utilities.
package util

import "sort"

// SortedMapKeys returns the keys of a string-keyed map in sorted order.
func SortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
