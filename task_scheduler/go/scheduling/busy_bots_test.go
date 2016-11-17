package scheduling

import (
	"fmt"
	"testing"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestBusyBots(t *testing.T) {
	testutils.SmallTest(t)

	bb := newBusyBots()
	b1 := &swarming_api.SwarmingRpcsBotInfo{
		BotId: "b1",
	}
	assert.False(t, bb.Busy(b1.BotId))
	bots := []*swarming_api.SwarmingRpcsBotInfo{b1}
	testutils.AssertDeepEqual(t, bots, bb.Filter(bots))

	bb.Reserve(b1.BotId)
	assert.True(t, bb.Busy(b1.BotId))
	testutils.AssertDeepEqual(t, []*swarming_api.SwarmingRpcsBotInfo{}, bb.Filter(bots))

	bb.Release(b1.BotId)
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
			bb.Reserve(b.BotId)
			assert.True(t, bb.Busy(b.BotId))
		} else {
			assert.False(t, bb.Busy(b.BotId))
			expect = append(expect, b)
		}
	}
	testutils.AssertDeepEqual(t, expect, bb.Filter(bots))
}
