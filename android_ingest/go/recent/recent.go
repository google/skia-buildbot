// Package recent tracks the last 20 incoming JSON request.
package recent

import (
	"sync"
	"time"
)

const (
	// MAX_RECENT is the largest number of recent requests that will be displayed.
	MAX_RECENT = 20
)

// Request is a record of a single POST request.
type Request struct {
	TS   string
	JSON string
}

// Recent tracks the last MAX_RECENT Requests.
type Recent struct {
	// mutex guards access to recent.
	mutex sync.Mutex

	// recent is just the last MAX_RECENT requests.
	recent []*Request
}

func New() *Recent {
	return &Recent{
		recent: []*Request{},
	}
}

// Add the JSON body that was POST'd to the server.
func (r *Recent) Add(b []byte) {
	// Store locally.
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.recent = append([]*Request{&Request{
		TS:   time.Now().UTC().String(),
		JSON: string(b),
	}}, r.recent...)

	// Keep track of the last N events.
	if len(r.recent) > MAX_RECENT {
		r.recent = r.recent[:MAX_RECENT]
	}
}

// List returns the last MAX_RECENT Requests, with the most recent
// Requests first.
func (r *Recent) List() []*Request {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	ret := make([]*Request, len(r.recent), len(r.recent))
	for i, req := range r.recent {
		ret[i] = req
	}
	return ret
}
