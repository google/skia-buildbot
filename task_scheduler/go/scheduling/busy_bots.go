package scheduling

import (
	"sort"
	"strings"
	"sync"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/trie"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// Metric name for free bots.
	MEASUREMENT_FREE_BOT_COUNT = "free_bot_count"

	// FILTER_* are used as the value of the "filter" key in metrics; we record counts for all free
	// bots and all free bots after allocating pending tasks to bots.
	FILTER_ALL_FREE_BOTS       = "all_free_bots"
	FILTER_MINUS_PENDING_TASKS = "minus_pending_tasks"

	// Metric name for pending tasks.
	MEASUREMENT_PENDING_TASK_COUNT = "pending_swarming_task_count"
)

var (
	// includeDimensions includes all dimensions used in
	// https://skia.googlesource.com/skia/+show/42974b73cd6f3515af69c553aac8dd15e3fc1927/infra/bots/gen_tasks.go
	// (except for "image" which has a TODO to remove).
	includeDimensions = []string{
		"cpu",
		"device",
		"device_os",
		"device_type",
		"gpu",
		"machine_type",
		"os",
		"release_version",
		"valgrind",
	}
)

func init() {
	sort.Strings(includeDimensions)
}

type BusyBotsDebugLog bool

const (
	BusyBotsDebugLoggingOn  BusyBotsDebugLog = true
	BusyBotsDebugLoggingOff BusyBotsDebugLog = false
)

// busyBots is a struct used for marking a bot as busy while it runs a Task.
type busyBots struct {
	// map[<filter>]map[<dimensionsString>]<count of bots>
	freeBotMetrics map[string]map[string]metrics2.Int64Metric
	pendingTasks   *trie.Trie
	mtx            sync.Mutex
	debug          BusyBotsDebugLog
}

// newBusyBots returns a busyBots instance.
func newBusyBots(debug BusyBotsDebugLog) *busyBots {
	return &busyBots{
		freeBotMetrics: map[string]map[string]metrics2.Int64Metric{},
		pendingTasks:   trie.New(),
		debug:          debug,
	}
}

// Return a space-separated string of sorted dimensions and values, filtered by
// includeDimensions.
// Similar to flatten in task_scheduler.go. When there are multiple values for a
// dimension, the longest is used (the longest value is usually the most
// interesting).  Therefore, this should only be used for things like metrics,
// not any actual dimension mapping.
func dimensionsString(dims []string) string {
	dimsMap := make(map[string]string, len(includeDimensions))
	for _, dim := range dims {
		split := strings.SplitN(dim, ":", 2)
		if len(split) != 2 {
			continue
		}
		key := split[0]
		val := split[1]
		if util.In(split[0], includeDimensions) {
			if exist, ok := dimsMap[key]; !ok || len(exist) < len(val) {
				dimsMap[key] = val
			}
		}
	}
	rv := make([]string, 0, 2*len(dimsMap))
	for _, key := range includeDimensions {
		if dimsMap[key] != "" {
			rv = append(rv, key, dimsMap[key])
		}
	}
	return strings.Join(rv, " ")
}

// recordBotMetrics updates MEASUREMENT_FREE_BOT_COUNT for the given filter based on bots. Assumes
// b.mtx is locked.
func (b *busyBots) recordBotMetrics(filter string, bots []*types.Machine) {
	metrics, ok := b.freeBotMetrics[filter]
	if !ok {
		metrics = map[string]metrics2.Int64Metric{}
		b.freeBotMetrics[filter] = metrics
	}
	counts := map[string]int64{}
	for _, bot := range bots {
		counts[dimensionsString(bot.Dimensions)]++
	}
	var sum int64 = 0
	for dims, count := range counts {
		metric, ok := metrics[dims]
		if !ok {
			metric = metrics2.GetInt64Metric(MEASUREMENT_FREE_BOT_COUNT, map[string]string{
				"filter":     filter,
				"dimensions": dims,
			})
			metrics[dims] = metric
		}
		metric.Update(count)
		sum += count
	}
	sklog.Debugf("Sum of %s counts: %d", filter, sum)
	for dims, metric := range metrics {
		_, ok := counts[dims]
		if !ok {
			metric.Update(0)
			delete(metrics, dims)
		}
	}
}

// Filter returns a copy of the given slice of bots with the busy bots removed.
func (b *busyBots) Filter(bots []*types.Machine) []*types.Machine {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	botNames := util.StringSet{}
	for _, bot := range bots {
		botNames[bot.ID] = true
	}
	sklog.Debugf("busyBots.Filter num bots %d; unique %d", len(bots), len(botNames))
	b.recordBotMetrics(FILTER_ALL_FREE_BOTS, bots)
	matched := make(map[string]bool, len(bots))
	rv := make([]*types.Machine, 0, len(bots))
	if b.debug {
		sklog.Debugf("Busy Bots: %s", b.pendingTasks.String())
	}
	for _, bot := range bots {
		// Find matching tasks.
		matches := b.pendingTasks.SearchSubset(bot.Dimensions)
		if b.debug {
			sklog.Debugf("%s (%v) matched: %v", bot.ID, bot.Dimensions)
		}
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
	b.recordBotMetrics(FILTER_MINUS_PENDING_TASKS, rv)
	return rv
}

// RefreshTasks updates the contents of busyBots based on the cached tasks.
func (b *busyBots) RefreshTasks(pending []*types.TaskResult) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.pendingTasks = trie.New()
	for _, t := range pending {
		dims := types.DimensionsFromTags(t.Tags)
		if len(dims) > 0 {
			b.pendingTasks.Insert(dims, t.ID)
		}
	}

	taskIds := util.StringSet{}
	for _, task := range pending {
		taskIds[task.ID] = true
	}
	sklog.Debugf("busyBots.RefreshTasks num tasks %d; unique %d; trie len %d", len(pending), len(taskIds), b.pendingTasks.Len())

	metrics2.GetInt64Metric(MEASUREMENT_PENDING_TASK_COUNT).Update(int64(len(pending)))
}
