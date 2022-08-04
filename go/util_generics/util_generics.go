// Package util_generics houses utility function which make us of generics. Chrome Infra
// is locked to Go 1.17 to support Mac 10.11, so we cannot ship code to CIPD that uses generics.
// Thus, we partition our common utilities (for now) until they are able to update.
package util_generics

// Get looks up an item in a map, returning a fallback value if absent.
func Get[K comparable, V interface{}](theMap map[K]V, k K, fallback V) V {
	if item, exists := theMap[k]; exists {
		return item
	}
	return fallback
}
