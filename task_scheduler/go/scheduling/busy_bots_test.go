package scheduling

import (
	"fmt"
	"testing"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestBusyBots(t *testing.T) {
	testutils.SmallTest(t)

	// No bots are busy.
	bb := newBusyBots()
	b1 := &swarming_api.SwarmingRpcsBotInfo{
		BotId: "b1",
	}
	assert.False(t, bb.Busy(b1.BotId))
	bots := []*swarming_api.SwarmingRpcsBotInfo{b1}
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	// Reserve the bot.
	taskId := "task1"
	bb.Reserve(b1.BotId, taskId)
	assert.True(t, bb.Busy(b1.BotId))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// Release the bot, from the wrong task. Ensure it's still busy.
	bb.Release(b1.BotId, "dummy-task")
	assert.True(t, bb.Busy(b1.BotId))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	// Really release the bot.
	bb.Release(b1.BotId, taskId)
	assert.False(t, bb.Busy(b1.BotId))
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	// Test with a bunch of bots.
	for i := 0; i < 10; i++ {
		bots = append(bots, &swarming_api.SwarmingRpcsBotInfo{
			BotId: fmt.Sprintf("b%d", i+2),
		})
	}
	for _, b := range bots {
		assert.False(t, bb.Busy(b.BotId))
	}
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	// Mark some as busy.
	expect := []*swarming_api.SwarmingRpcsBotInfo{}
	for i, b := range bots {
		if i%2 == 0 {
			bb.Reserve(b.BotId, fmt.Sprintf("task%d", i))
			assert.True(t, bb.Busy(b.BotId))
		} else {
			assert.False(t, bb.Busy(b.BotId))
			expect = append(expect, b)
		}
	}
	testutils.AssertDeepEqual(t, expect, bb.Filter(bots))
}
