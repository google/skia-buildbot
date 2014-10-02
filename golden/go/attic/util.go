package attic

import (
	"sort"
	"strings"
)

// Wrapper for the multi-value keys represented by a map[string]string.
type MultiKey map[string]string

// Create a new instance of MultiKey.
func NewMultiKey(key map[string]string) *MultiKey {
	var v MultiKey = key
	return &v
}

// Returns a string representation of this key.
func (m MultiKey) Key() string { return MapToStrKey(m) }

// Convert a map[string]string to a string. To be used throughout
// so we can change the string representation centrally.
// Currently does not include keys (only values) and thus might
// create collisions (very unlikely).
func MapToStrKey(m map[string]string) string {
	keys := make([]string, 0, len(m))
	vals := make([]string, 0, len(m))

	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals = append(vals, m[k])
	}
	return strings.Join(vals, ":")
}

// Returns a copy of a map containing a subset of keys.
func SubMap(m map[string]string, keys []string) map[string]string {
	result := make(map[string]string)
	for _, k := range keys {
		result[k] = m[k]
	}
	return result
}
