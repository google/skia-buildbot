package find_breaks

// slice is a struct used for identifying regions of a parent array/slice. Two
// slice instances may be combined into a new slice instance representing the
// region where the two overlap, if any.
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

// makeSlice creates a slice from the given slice of strings as a subslice of
// the given parent slice of strings.
func makeSlice(s []string, parent []string) slice {
	if len(s) == 0 {
		return newSlice(-1, -1)
	}

	// We can't guarantee the order of db.Task.Commits, and it's not
	// convenient to sort chronologically before this point, so rather than
	// iterating through the parent slice to find the sub-slice, we use a map,
	// realizing that the commits will be unique and so the order within the
	// sub-slice is not important.
	m := make(map[string]bool, len(s))
	for _, str := range s {
		m[str] = true
	}
	startIdx := -1
	endIdx := -1
	for idx, c := range parent {
		_, ok := m[c]
		if ok {
			if startIdx == -1 {
				startIdx = idx
			}
			delete(m, c)
		} else {
			if startIdx >= 0 {
				// Some tasks will have blamelists which extend
				// outside of the time period. Therefore, we
				// have to handle sub-slices which aren't
				// contained within the parent slice.
				// Unfortunately, this prevents us from handling
				// the case where the sub-slice does not form a
				// contiguous sub-slice of the parent, ie:
				//
				//	if len(m) != 0 {
				//		// Non-contiguous slice!
				//		return newSlice(-1, -1)
				//	}

				// We've reached the end of the sub-slice.
				return newSlice(startIdx, idx)
			}
		}
	}
	if startIdx >= 0 {
		endIdx = len(parent)
	}
	return newSlice(startIdx, endIdx)
}

// Resolve creates an actual slice of strings from the given slice instance.
func (s slice) Resolve(strs []string) []string {
	if !s.Valid() || s.Empty() {
		return []string{}
	}
	if s.start >= len(strs) || s.end > len(strs) || s.start > s.end {
		return []string{}
	}
	return strs[s.start:s.end]
}
