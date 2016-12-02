package scheduling

import (
	"testing"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
)

func TestBusyBots(t *testing.T) {
	testutils.SmallTest(t)

	bot := func(id string, dims map[string][]string) *swarming_api.SwarmingRpcsBotInfo {
		return &swarming_api.SwarmingRpcsBotInfo{
			BotId:      id,
			Dimensions: swarming.StringMapToBotDimensions(dims),
		}
	}

	task := func(id string, dims map[string]string) *swarming_api.SwarmingRpcsTaskRequestMetadata {
		return &swarming_api.SwarmingRpcsTaskRequestMetadata{
			Request: &swarming_api.SwarmingRpcsTaskRequest{
				Properties: &swarming_api.SwarmingRpcsTaskProperties{
					Dimensions: swarming.StringMapToTaskDimensions(dims),
				},
			},
			TaskId: id,
		}
	}

	// No bots are busy.
	bb := newBusyBots()
	b1 := bot("b1", map[string][]string{
		"pool": []string{"Skia"},
	})
	bots := []*swarming_api.SwarmingRpcsBotInfo{b1}
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	// Reserve the bot for a task.
	t1 := task("t1", map[string]string{"pool": "Skia"})
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1}))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// Ensure that it's still busy.
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// It's no longer busy.
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{}))
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	// There are two bots and one task.
	b2 := bot("b2", map[string][]string{
		"pool": []string{"Skia"},
	})
	bots = append(bots, b2)
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1}))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b2}, bb.Filter(bots))

	// Two tasks and one bot.
	t2 := task("t2", map[string]string{"pool": "Skia"})
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2}))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b1}))

	// Differentiate between dimension sets.
	// Since busyBots works in order, if we were arbitrarily picking any
	// bot for each task, then b3 would get filtered out. Verify that b4
	// gets filtered out as we'd expect.
	b3 := bot("b3", linuxBotDims)
	b4 := bot("b4", androidBotDims)
	t3 := task("t3", androidTaskDims)
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t3}))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b3}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b3, b4}))

	// Test supersets of dimensions.
	assert.NoError(t, bb.RefreshTasks([]*swarming_api.SwarmingRpcsTaskRequestMetadata{t1, t2, t3}))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{b3}, bb.Filter([]*swarming_api.SwarmingRpcsBotInfo{b1, b2, b3, b4}))
}
