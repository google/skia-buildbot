package scheduling

import (
	"fmt"
	"testing"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestBusyBots(t *testing.T) {
	testutils.SmallTest(t)

	bot := func(id string, dims map[string][]string) *swarming_api.SwarmingRpcsBotInfo {
		return &swarming_api.SwarmingRpcsBotInfo{
			BotId:      id,
			Dimensions: swarming.StringMapToBotDimensions(dims),
		}
	}

	task := func(id string, dims map[string]string) *swarming_api.SwarmingRpcsTaskResult {
		tags := make([]string, 0, len(dims))
		for k, v := range dims {
			tags = append(tags, fmt.Sprintf("%s%s:%s", types.SWARMING_TAG_DIMENSION_PREFIX, k, v))
		}
		return &swarming_api.SwarmingRpcsTaskResult{
			Tags:   tags,
			TaskId: id,
		}
	}

	// No bots are busy.
	bb := newBusyBots()
	b1 := bot("b1", map[string][]string{
		"pool": {"Skia"},
	})
	bots := []*swarming_api.SwarmingRpcsBotInfo{b1}
	deepequal.AssertDeepEqual(t, bots, bb.Filter(bots))

	// Reserve the bot for a task.
	t1 := task("t1", map[string]string{"pool": "Skia"})
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{t1})
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// Ensure that it's still busy.
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// It's no longer busy.
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{})
	deepequal.AssertDeepEqual(t, bots, bb.Filter(bots))

	// There are two bots and one task.
	b2 := bot("b2", map[string][]string{
		"pool": {"Skia"},
	})
	bots = append(bots, b2)
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{t1})
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b2}, bb.Filter(bots))

	// Two tasks and one bot.
	t2 := task("t2", map[string]string{"pool": "Skia"})
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{t1, t2})
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b1}))

	// Differentiate between dimension sets.
	// Since busyBots works in order, if we were arbitrarily picking any
	// bot for each task, then b3 would get filtered out. Verify that b4
	// gets filtered out as we'd expect.
	b3 := bot("b3", linuxBotDims)
	b4 := bot("b4", androidBotDims)
	t3 := task("t3", androidTaskDims)
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{t3})
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b3}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b3, b4}))

	// Test supersets of dimensions.
	bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskResult{t1, t2, t3})
	deepequal.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b3}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b1, b2, b3, b4}))
}
