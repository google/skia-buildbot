package scheduling

import (
	"sync"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"
)

// busyBots is a struct used for marking a bot as busy while it runs a Task.
type busyBots struct {
	bots map[string]string
	mtx  sync.Mutex
}

// newBusyBots returns a busyBots instance.
func newBusyBots() *busyBots {
	return &busyBots{
		bots: map[string]string{},
	}
}

// Reserve marks a bot as busy.
func (b *busyBots) Reserve(bot, task string) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.bots[bot] = task
}

// Filter returns a copy of the given slice of bots with the busy bots removed.
func (b *busyBots) Filter(bots []*swarming_api.SwarmingRpcsBotInfo) []*swarming_api.SwarmingRpcsBotInfo {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	// TODO(benjaminwagner): Remove debug logging.
	androidbots := map[string]string{}
	for _, bot := range bots {
		task, ok := b.bots[bot.BotId]
		if ok {
			if hasDim(bot, "device_type", "*") {
				androidbots[bot.BotId] = "busy " + task
			}
		} else {
			if hasDim(bot, "device_type", "*") {
				androidbots[bot.BotId] = "available"
			}
			rv = append(rv, bot)
		}
	}
	glog.Infof("DEBUG: android bots: %s", androidbots)
	return rv
}

// Busy returns true iff the bot is busy.
func (b *busyBots) Busy(bot string) bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	_, ok := b.bots[bot]
	return ok
}

// Release marks the bot as not busy.
func (b *busyBots) Release(bot, task string) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	if b.bots[bot] == task {
		delete(b.bots, bot)
	}
}
