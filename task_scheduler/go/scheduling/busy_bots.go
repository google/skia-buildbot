package scheduling

import (
	"sync"
	"time"
)

// busyBots is a struct used for marking a bot as busy for a time period after
// triggering a Task on it.
type busyBots struct {
	bots     map[string]bool
	busyTime time.Duration
	mtx      sync.Mutex
}

// newBusyBots returns a busyBots instance.
func newBusyBots(busyTime time.Duration) *busyBots {
	return &busyBots{
		bots:     map[string]bool{},
		busyTime: busyTime,
	}
}

// Put marks a bot as busy.
func (b *busyBots) Put(bot string) {
	if b.busyTime == 0 {
		return
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.bots[bot] = true
	go func() {
		time.Sleep(b.busyTime)
		b.mtx.Lock()
		defer b.mtx.Unlock()
		delete(b.bots, bot)
	}()
}

// Get returns true iff the bot is busy.
func (b *busyBots) Get(bot string) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.bots[bot]
}
