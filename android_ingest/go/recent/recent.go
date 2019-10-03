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

	// recentGood is just the last MAX_RECENT requests.
	recentGood []*Request

	// recentBad is just the last MAX_RECENT requests.
	recentBad []*Request
}

func New() *Recent {
	return &Recent{
		recentGood: []*Request{},
		recentBad:  []*Request{},
	}
}

func (r *Recent) add(recent []*Request, b []byte) []*Request {
	// Store locally.
	r.mutex.Lock()
	defer r.mutex.Unlock()

	recent = append([]*Request{{
		TS:   time.Now().UTC().String(),
		JSON: string(b),
	}}, recent...)

	// Keep track of the last N events.
	if len(recent) > MAX_RECENT {
		recent = recent[:MAX_RECENT]
	}
	return recent
}

// AddBad the JSON body that was POST'd to the server.
func (r *Recent) AddBad(b []byte) {
	r.recentBad = r.add(r.recentBad, b)
}

// AddGood the JSON body that was POST'd to the server.
func (r *Recent) AddGood(b []byte) {
	r.recentGood = r.add(r.recentGood, b)
}

// List returns the last MAX_RECENT Good and Bad Requests, with the most recent
// Requests first.
func (r *Recent) List() ([]*Request, []*Request) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	good := make([]*Request, len(r.recentGood), len(r.recentGood))
	for i, req := range r.recentGood {
		good[i] = req
	}
	bad := make([]*Request, len(r.recentBad), len(r.recentBad))
	for i, req := range r.recentBad {
		bad[i] = req
	}
	return good, bad
}
