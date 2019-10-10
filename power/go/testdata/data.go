package testdata

import (
	"encoding/json"

	"github.com/stretchr/testify/require"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
)

// The JSON in these files was recieved from calls to
// https://chromium-swarm.appspot.com/_ah/api/swarming/v1/bot/[bot]/get
// when bots were displaying interesting behavior
const (
	BOOTING_DEVICE       = "booting_device.json"
	DEAD_AND_QUARANTINED = "dead_and_quarantined.json"
	DEAD_BOT             = "dead_bot.json"
	DELETED_BOT          = "deleted_bot.json"
	MISSING_DEVICE       = "missing_device.json"
	TOO_HOT              = "too_hot.json"
	USB_FAILURE          = "usb_failure.json"
)

// MockBotAndId creates a *swarming.SwarmingRpcsBotInfo using the matching JSON
// file in ./testdata
func MockBotAndId(t sktest.TestingT, filename, id string) *swarming.SwarmingRpcsBotInfo {
	j, err := testutils.ReadFile(filename)
	require.NoError(t, err, "There was a problem reading in the test data")
	var s swarming.SwarmingRpcsBotInfo
	err = json.Unmarshal([]byte(j), &s)
	require.NoError(t, err, "There was a problem parsing the test data")
	s.BotId = id
	return &s
}
