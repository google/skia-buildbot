package scheduling

import (
	"fmt"
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/task_scheduler/go/types"
)

func bot(id string, dims map[string][]string) *types.Machine {
	dimsFlat := make([]string, 0, len(dims))
	for key, values := range dims {
		for _, val := range values {
			dimsFlat = append(dimsFlat, fmt.Sprintf("%s:%s", key, val))
		}
	}
	return &types.Machine{
		ID:         id,
		Dimensions: dimsFlat,
	}
}

func task(id string, dims map[string]string) *types.TaskResult {
	tags := make(map[string][]string, len(dims))
	for k, v := range dims {
		tagKey := fmt.Sprintf("%s%s", types.SWARMING_TAG_DIMENSION_PREFIX, k)
		tags[tagKey] = []string{v}
	}
	return &types.TaskResult{
		ID:   id,
		Tags: tags,
	}
}

func TestBusyBots(t *testing.T) {
	// No bots are busy.
	bb := newBusyBots(BusyBotsDebugLoggingOff)
	b1 := bot("b1", map[string][]string{
		"pool": {"Skia"},
	})
	bots := []*types.Machine{b1}
	assertdeep.Equal(t, bots, bb.Filter(bots))

	// Reserve the bot for a task.
	t1 := task("t1", map[string]string{"pool": "Skia"})
	bb.RefreshTasks([]*types.TaskResult{t1})
	assertdeep.Equal(t, []*types.Machine{}, bb.Filter(bots))

	// Ensure that it's still busy.
	assertdeep.Equal(t, []*types.Machine{}, bb.Filter(bots))

	// It's no longer busy.
	bb.RefreshTasks([]*types.TaskResult{})
	assertdeep.Equal(t, bots, bb.Filter(bots))

	// There are two bots and one task.
	b2 := bot("b2", map[string][]string{
		"pool": {"Skia"},
	})
	bots = append(bots, b2)
	bb.RefreshTasks([]*types.TaskResult{t1})
	assertdeep.Equal(t, []*types.Machine{b2}, bb.Filter(bots))

	// Two tasks and one bot.
	t2 := task("t2", map[string]string{"pool": "Skia"})
	bb.RefreshTasks([]*types.TaskResult{t1, t2})
	assertdeep.Equal(t, []*types.Machine{}, bb.Filter([]*types.Machine{b1}))

	// Differentiate between dimension sets.
	// Since busyBots works in order, if we were arbitrarily picking any
	// bot for each task, then b3 would get filtered out. Verify that b4
	// gets filtered out as we'd expect.
	b3 := bot("b3", linuxBotDims)
	b4 := bot("b4", androidBotDims)
	t3 := task("t3", androidTaskDims)
	bb.RefreshTasks([]*types.TaskResult{t3})
	assertdeep.Equal(t, []*types.Machine{b3}, bb.Filter([]*types.Machine{b3, b4}))

	// Test supersets of dimensions.
	bb.RefreshTasks([]*types.TaskResult{t1, t2, t3})
	assertdeep.Equal(t, []*types.Machine{b3}, bb.Filter([]*types.Machine{b1, b2, b3, b4}))
}

func TestBusyBots_TaskHasNoKnownDimensions_NoBotAppearsBusy(t *testing.T) {
	b1 := bot("b1", map[string][]string{
		"pool": {"Skia"},
		"os":   {"Linux"},
	})
	b2 := bot("b2", map[string][]string{
		"pool": {"Skia"},
		"os":   {"Windows"},
	})
	bots := []*types.Machine{b1, b2}

	bb := newBusyBots(BusyBotsDebugLoggingOff)

	// Add a task which has no known dimensions.
	t1 := task("t1", map[string]string{})
	bb.RefreshTasks([]*types.TaskResult{t1})

	// No bots should appear busy.
	assertdeep.Equal(t, bots, bb.Filter(bots))
}
