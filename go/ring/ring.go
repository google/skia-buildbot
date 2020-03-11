package ring

import (
	"sync"

	"go.skia.org/infra/go/skerr"
)

// StringRing stores the last N strings passed to Put(). It is thread-safe.
type StringRing struct {
	len     int
	content []string
	mtx     sync.Mutex
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
