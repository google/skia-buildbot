package scheduling

import (
	"fmt"
	"sync"

	"go.skia.org/infra/go/trie"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

const (
	MEASUREMENT_BUSY_BOTS = "busy-bots"
)

// busyBots is a struct used for marking a bot as busy while it runs a Task.
type busyBots struct {
	pendingTasks *trie.Trie // map[dimension-set]num-pending
	mtx          sync.Mutex
}

// newBusyBots returns a busyBots instance.
func newBusyBots() *busyBots {
	return &busyBots{
		pendingTasks: trie.New(),
	}
}

// entry is a struct stored in the Trie which busyBots uses to match tasks
// with bots by dimension set.
type entry struct {
	dims []string
	id   string
}

// Filter returns a copy of the given slice of bots with the busy bots removed.
func (b *busyBots) Filter(bots []*swarming_api.SwarmingRpcsBotInfo) []*swarming_api.SwarmingRpcsBotInfo {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	matched := make(map[string]bool, len(bots))
	rv := make([]*swarming_api.SwarmingRpcsBotInfo, 0, len(bots))
	for _, bot := range bots {
		// Collect dimensions.
		dims := make([]string, 0, len(bot.Dimensions))
		for _, d := range bot.Dimensions {
			for _, v := range d.Value {
				dims = append(dims, fmt.Sprintf("%s:%s", d.Key, v))
			}
		}
		// Find matching tasks.
		matches := b.pendingTasks.SearchSubset(dims)
		// Choose the first non-empty entry and pretend that
		// this bot is busy with that task.
		var e *entry
		for _, match := range matches {
			m := match.(*entry)
			if _, ok := matched[m.id]; !ok {
				e = m
				break
			}
		}
		if e != nil {
			matched[e.id] = true
		} else {
			rv = append(rv, bot)
		}
	}
	return rv
}

// RefreshTasks updates the contents of busyBots based on the cached tasks.
func (b *busyBots) RefreshTasks(pending []*swarming_api.SwarmingRpcsTaskRequestMetadata) error {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.pendingTasks = trie.New()
	for _, t := range pending {
		// Collect dimensions.
		dims := make([]string, 0, len(t.Request.Properties.Dimensions))
		for _, d := range t.Request.Properties.Dimensions {
			dims = append(dims, fmt.Sprintf("%s:%s", d.Key, d.Value))
		}
		b.pendingTasks.Insert(dims, &entry{
			dims: dims,
			id:   t.TaskId,
		})
	}
	return nil
}
