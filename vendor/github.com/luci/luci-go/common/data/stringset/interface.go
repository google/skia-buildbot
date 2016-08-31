// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stringset

// Set is the interface for all string set implementations in this package.
type Set interface {
	// Has returns true iff the Set contains value.
	Has(value string) bool

	// Add ensures that Set contains value, and returns true if it was added (i.e.
	// it returns false if the Set already contained the value).
	Add(value string) bool

	// Del removes value from the set, and returns true if it was deleted (i.e. it
	// returns false if the Set did not already contain the value).
	Del(value string) bool

	// Peek returns an arbitrary element from the set. If the set was empty, this
	// returns ("", false).
	Peek() (string, bool)

	// Peek removes and returns an arbitrary element from the set. If the set was
	// empty, this returns ("", false).
	Pop() (string, bool)

	// Iter calls `cb` for each item in the set. If `cb` returns false, the
	// iteration stops.
	Iter(cb func(string) bool)

	// Len returns the number of items in this set.
	Len() int

	// Dup returns a duplicate set.
	Dup() Set

	// ToSlice renders this set to a slice of all values.
	ToSlice() []string
}
