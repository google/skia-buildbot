package scheduling

import (
	"sync"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/trie"
	"go.skia.org/infra/task_scheduler/go/db"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

const (
	MEASUREMENT_BUSY_BOTS = "busy-bots"
)

// busyBots is a struct used for marking a bot as busy while it runs a Task.
type busyBots struct {
	pendingTasks *trie.Trie
	mtx          sync.Mutex
}

// newBusyBots returns a busyBots instance.
func newBusyBots() *busyBots {
	return &busyBots{
		pendingTasks: trie.New(),
	}
}

// Filter returns a copy of the given slice of bots with the busy bots removed.
func (b *busyBots) Filter(bots []*swarming_api.SwarmingRpcsBotInfo) []*swarming_api.SwarmingRpcsBotInfo {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	matched := make(map[string]bool, len(bots))
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, bot := range bots {
		// Find matching tasks.
		matches := b.pendingTasks.SearchSubset(swarming.BotDimensionsToStringSlice(bot.Dimensions))
		// Choose the first non-empty entry and pretend that
		// this bot is busy with that task.
		var e string
		for _, match := range matches {
			m := match.(string)
			if _, ok := matched[m]; !ok {
				e = m
				break
			}
		}
		if e != "" {
			matched[e] = true
		} else {
			rv = append(rv, bot)
		}
	}
	return rv
}

// RefreshTasks updates the contents of busyBots based on the cached tasks.
func (b *busyBots) RefreshTasks(pending []*swarming_api.SwarmingRpcsTaskResult) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.pendingTasks = trie.New()
	for _, t := range pending {
		dims := db.DimensionsFromTags(t.Tags)
		b.pendingTasks.Insert(dims, t.TaskId)
	}
}
