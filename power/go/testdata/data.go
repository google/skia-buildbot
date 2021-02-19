package testdata

import (
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
	var s swarming.SwarmingRpcsBotInfo
	testutils.ReadJSONFile(t, filename, &s)
	s.BotId = id
	return &s
}
