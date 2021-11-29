// Package sets provides functions for operations on sets.
package sets

import "go.skia.org/infra/go/skerr"

// dup makes a copy of an int slice.
func dup(s []int) []int {
	ret := make([]int, len(s))
	copy(ret, s)
	return ret
}

// allZeroes returns true if every int in a slice is zero.
func allZeroes(s []int) bool {
	for _, n := range s {
		if n != 0 {
			return false
		}
	}
	return true
}

// CartesianProduct returns a channel that lists all the combinations of
// elements from the given setSizes to produce the Cartesian Product of all the
// values in the sets.
//
// For example:
//
//    CartesianProduct([]int{3, 2})
//
// Will produce the following int slices:
//
//    {2, 1},
//    {1, 1},
//    {0, 1},
//    {2, 0},
//    {1, 0},
//    {0, 0},
//
// Each setSize must be greater than one.
func CartesianProduct(setSizes []int) (<-chan []int, error) {
	ret := make(chan []int)
	if len(setSizes) == 0 {
		close(ret)
		return ret, nil
	}
	for _, n := range setSizes {
		if n < 1 {
			return nil, skerr.Fmt("set size must be >= 1, got %d", n)
		}
	}

	// Convert the set sizes to indices by subtracting one.
	setMaxIndex := dup(setSizes)
	for i := range setMaxIndex {
		setMaxIndex[i]--
	}

	// curent is the current set of indices we are going to emit on the channel.
	current := dup(setMaxIndex)
	go func() {
		for {
			ret <- dup(current)
			if allZeroes(current) {
				close(ret)
				break
			}

			// Decrement current.
			current[0]--
			for i := 1; i < len(current); i++ {
				if current[i-1] < 0 {
					current[i-1] = setMaxIndex[i-1]
					current[i]--
				}
			}
		}
	}()

	return ret, nil
}
