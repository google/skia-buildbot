package find_breaks

import "go.skia.org/infra/go/git/repograph"

// slice is a struct used for slicing subslices of a larger slice while
// supporting finding overlapping subslices.
type slice struct {
	start int
	end   int
}

// newSlice returns a slice instance.
func newSlice(start, end int) slice {
	return slice{
		start: start,
		end:   end,
	}
}

// Valid returns true iff the slice is valid. A slice is valid if it is empty
// (start and end are both < 0), or if both start and end are nonnegative with
// end >= start.
func (s slice) Valid() bool {
	if s.start < 0 && s.end < 0 {
		return true // Empty slices are valid.
	}
	if s.start < 0 || s.end < 0 {
		return false
	}
	if s.start > s.end {
		return false
	}
	return true
}

// Overlap returns a new slice representing the overlapping region of the two
// slices.
func (s slice) Overlap(other slice) slice {
	if !s.Valid() || !other.Valid() {
		return newSlice(-1, -1)
	}
	rv := newSlice(s.start, s.end)
	if other.start > rv.start {
		rv.start = other.start
	}
	if other.end < rv.end {
		rv.end = other.end
	}
	if rv.start >= rv.end {
		rv.start = -1
		rv.end = -1
	}
	return rv
}

// Len returns the length of the slice.
func (s slice) Len() int {
	if !s.Valid() {
		return 0
	}
	return s.end - s.start
}

// Empty returns true iff the slice has zero length.
func (s slice) Empty() bool {
	return s.Len() == 0
}

// Copy returns a copy of this slice.
func (s slice) Copy() slice {
	return newSlice(s.start, s.end)
}

// makeSlice creates a slice from the given slice of commit hashes as a subslice of
// the given slice of commit objects.
func makeSlice(s []string, commits []*repograph.Commit) slice {
	m := make(map[string]bool, len(s))
	for _, c := range s {
		m[c] = true
	}

	startIdx := -1
	endIdx := -1
	sIdx := -1
	for idx, c := range commits {
		if c.Hash == s[0] {
			startIdx = idx
		}
		if startIdx >= 0 {
			sIdx = idx - startIdx
			if sIdx == len(s) {
				// We've reached the end of the sub-slice.
				return newSlice(startIdx, idx)
			} else {
				if s[sIdx] != c.Hash {
					return newSlice(-1, -1) // Error?
				}
			}
		}
	}
	if sIdx != len(s)-1 {
		// We reached the end of the parent slice but not the end of the
		// sub-slice.
		return newSlice(-1, -1)
	}
	if startIdx >= 0 && endIdx == -1 {
		endIdx = len(commits)
	}
	return newSlice(startIdx, endIdx)
}
