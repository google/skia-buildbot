package ring

import (
	"go.skia.org/infra/go/skerr"
)

// StringRing stores the last N strings passed to Put(). It is not thread-safe.
type StringRing struct {
	len     int
	content []string
}

// NewStringRing returns a StringRing with the given capacity.
func NewStringRing(capacity int) (*StringRing, error) {
	if capacity < 1 {
		return nil, skerr.Fmt("Invalid ring capacity, must be > 0: %d", capacity)
	}
	return &StringRing{
		content: make([]string, capacity),
	}, nil
}

// GetAll returns all values stored in the ring.
func (r *StringRing) GetAll() []string {
	rv := make([]string, 0, r.len%cap(r.content))
	start := r.len - cap(r.content)
	if start < 0 {
		start = 0
	}
	for i := start; i < r.len; i++ {
		rv = append(rv, r.content[i%cap(r.content)])
	}
	return rv
}

// Put appends the given value to the ring, possibly overwriting a previous
// value.
func (r *StringRing) Put(s string) {
	r.content[r.len%cap(r.content)] = s
	r.len++
}
