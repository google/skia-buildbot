package scheduling

import (
	"sync"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

// busyBots is a struct used for marking a bot as busy while it runs a Task.
type busyBots struct {
	bots map[string]bool
	mtx  sync.Mutex
}

// newBusyBots returns a busyBots instance.
func newBusyBots() *busyBots {
	return &busyBots{
		bots: map[string]bool{},
	}
}

// Reserve marks a bot as busy.
func (b *busyBots) Reserve(bot string) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.bots[bot] = true
}

// Filter returns a copy of the given slice of bots with the busy bots removed.
func (b *busyBots) Filter(bots []*swarming_api.SwarmingRpcsBotInfo) []*swarming_api.SwarmingRpcsBotInfo {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, bot := range bots {
		if !b.bots[bot.BotId] {
			rv = append(rv, bot)
		}
	}
	return rv
}

// Busy returns true iff the bot is busy.
func (b *busyBots) Busy(bot string) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.bots[bot]
}

// Release marks the bot as not busy.
func (b *busyBots) Release(bot string) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	delete(b.bots, bot)
}
