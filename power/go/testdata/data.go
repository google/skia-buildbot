package testdata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/testutils"
)

// The JSON in these files was recieved from calls to
// https://chromium-swarm.appspot.com/_ah/api/swarming/v1/bot/[bot]/get
// when bots were displaying interesting behavior
var DEAD_BOT = "dead_bot.json"
var DEAD_AND_QUARANTINED = "dead_and_quarantined.json"
var DELETED_BOT = "deleted_bot.json"
var MISSING_DEVICE = "missing_device.json"
var TOO_HOT = "too_hot.json"
var USB_FAILURE = "usb_failure.json"

// MockBotAndId creates a *swarming.SwarmingRpcsBotInfo using the matching JSON
// file in ./testdata
func MockBotAndId(t *testing.T, filename, id string) *swarming.SwarmingRpcsBotInfo {
	j, err := testutils.ReadFile(filename)
	assert.NoError(t, err, "There was a problem reading in the test data")
	var s swarming.SwarmingRpcsBotInfo
	err = json.Unmarshal([]byte(j), &s)
	assert.NoError(t, err, "There was a problem parsing the test data")
	s.BotId = id
	return &s
}
