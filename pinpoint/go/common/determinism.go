package common

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

// SortedKeys accepts any map where the keys can be ordered, and returns them as a sorted slice.
func SortedKeys[Map ~map[K]V, K cmp.Ordered, V any](m Map) []K {
	//workflowcheck:ignore
	keys := slices.Collect(maps.Keys(m))
	slices.Sort(keys)
	return keys
}

// SortedRange returns an iterator over a map in sorted key order.
func SortedRange[Map ~map[K]V, K cmp.Ordered, V any](m Map) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range SortedKeys(m) {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}
