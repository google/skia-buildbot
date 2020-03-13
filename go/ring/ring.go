package ring

import (
	"sync"
)

// StringRing stores the last N strings passed to Put(). It is thread-safe.
type StringRing struct {
	len     int
	content []string
	mtx     sync.Mutex
}

// NewStringRing returns a StringRing with the given capacity. Panics if the
// capacity is negative.
func NewStringRing(capacity int) *StringRing {
	if capacity < 1 {
		panic("ring capacity must be at least 1")
	}
	return &StringRing{
		content: make([]string, capacity),
	}
}

// GetAll returns all values stored in the ring.
func (r *StringRing) GetAll() []string {
	r.mtx.Lock()
	defer r.mtx.Unlock()
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
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.content[r.len%cap(r.content)] = s
	r.len++
}

// StringRing implements io.Writer for convenience.
func (r *StringRing) Write(b []byte) (int, error) {
	r.Put(string(b))
	return len(b), nil
}
