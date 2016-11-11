package scheduling

import "time"

// busyBots is a struct used for marking a bot as busy for a time period after
// triggering a Task on it.
type busyBots struct {
	bots     map[string]bool
	busyTime time.Duration
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
	b.bots[bot] = true
	go func() {
		time.Sleep(b.busyTime)
		delete(b.bots, bot)
	}()
}

// Get returns true iff the bot is busy.
func (b *busyBots) Get(bot string) bool {
	return b.bots[bot]
}
