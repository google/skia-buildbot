package progress

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/util"
)

/*
Package progress implements a simple progress tracking system.
*/

// Tracker is a simple progress tracker.
type Tracker struct {
	mtx   sync.Mutex
	count int64
	total int64
}

// New returns a Tracker instance with the given total.
func New(total int64) *Tracker {
	return &Tracker{
		total: total,
	}
}

// Inc increments the counter by the given amount.
func (t *Tracker) Inc(inc int64) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.count += inc
}

// Get returns the current count and total.
func (t *Tracker) Get() (int64, int64) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.count, t.total
}

// AtInterval runs the given func at the given interval until the given context
// expires or the total count is reached.
func (t *Tracker) AtInterval(ctx context.Context, interval time.Duration, f func(count, total int64)) {
	ctx, cancel := context.WithCancel(ctx)
	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		count, total := t.Get()
		f(count, total)
		if count == total {
			cancel()
		}
	})
}
