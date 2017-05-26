package decider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/power/go/testdata"
)

type testcase struct {
	bot              *swarming.SwarmingRpcsBotInfo
	shouldPowercycle bool
}

const MOCK_BOT_ID = "some-bot"

func TestShouldPowercycleBot(t *testing.T) {
	// This test assumes all the bots involved are powercyclable
	tests := map[string]testcase{
		"TooHot": {
			bot:              mockBot(testdata.TOO_HOT_JSON),
			shouldPowercycle: false,
		},
		"USBFailure": {
			bot:              mockBot(testdata.USB_FAILURE_JSON),
			shouldPowercycle: false,
		},
		"MissingDevice": {
			bot:              mockBot(testdata.MISSING_DEVICE_JSON),
			shouldPowercycle: false,
		},
		"DeadBot": {
			bot:              mockBot(testdata.DEAD_BOT_JSON),
			shouldPowercycle: true,
		},
		"DeadQuarantinedBot": {
			bot:              mockBot(testdata.DEAD_AND_QUARANTINED_JSON),
			shouldPowercycle: true,
		},
		"DeletedBot": {
			bot:              mockBot(testdata.DELETED_BOT_JSON),
			shouldPowercycle: false,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{MOCK_BOT_ID}),
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleBot(c.bot))
		})
	}
}

func TestShouldPowercycleDevice(t *testing.T) {
	// This test assumes all the bots involved are powercyclable
	tests := map[string]testcase{
		"TooHot": {
			bot:              mockBot(testdata.TOO_HOT_JSON),
			shouldPowercycle: false,
		},
		"USBFailure": {
			bot:              mockBot(testdata.USB_FAILURE_JSON),
			shouldPowercycle: true,
		},
		"MissingDevice": {
			bot:              mockBot(testdata.MISSING_DEVICE_JSON),
			shouldPowercycle: true,
		},
		"DeadBot": {
			bot:              mockBot(testdata.DEAD_BOT_JSON),
			shouldPowercycle: false,
		},
		"DeadQuarantinedBot": {
			bot:              mockBot(testdata.DEAD_AND_QUARANTINED_JSON),
			shouldPowercycle: false,
		},
		"DeletedBot": {
			bot:              mockBot(testdata.DELETED_BOT_JSON),
			shouldPowercycle: false,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{MOCK_BOT_ID + "-device"}),
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleDevice(c.bot))
		})
	}
}

func mockBot(j string) *swarming.SwarmingRpcsBotInfo {
	return mockBotAndId(j, MOCK_BOT_ID)
}

func mockBotAndId(j, id string) *swarming.SwarmingRpcsBotInfo {
	b := bytes.NewBufferString(j)
	var s swarming.SwarmingRpcsBotInfo
	d := json.NewDecoder(b)
	if err := d.Decode(&s); err != nil {
		fmt.Println("Error parsing json: %s", err)
		return nil
	}
	s.BotId = id
	return &s
}

func TestIDBasedPowercycleBot(t *testing.T) {
	// This test tests the enabledBots logic
	tests := map[string]testcase{
		"SunnyDay": {
			bot:              mockBotAndId(testdata.DEAD_BOT_JSON, "bot-001"),
			shouldPowercycle: true,
		},
		"NotEnabled": {
			bot:              mockBotAndId(testdata.DEAD_BOT_JSON, "not-enabled"),
			shouldPowercycle: false,
		},
		"JustBotInList": {
			bot:              mockBotAndId(testdata.DEAD_BOT_JSON, "bot-002"),
			shouldPowercycle: true,
		},
		"JustDeviceInList": {
			bot:              mockBotAndId(testdata.DEAD_BOT_JSON, "bot-003"),
			shouldPowercycle: false,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{
			"bot-001",
			"bot-001-device",
			"bot-002",
			"bot-003-device",
		}),
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleBot(c.bot))
		})
	}
}

func TestIDBasedPowercycleDevice(t *testing.T) {
	// This test tests the enabledBots logic
	tests := map[string]testcase{
		"SunnyDay": {
			bot:              mockBotAndId(testdata.MISSING_DEVICE_JSON, "bot-001"),
			shouldPowercycle: true,
		},
		"NotEnabled": {
			bot:              mockBotAndId(testdata.MISSING_DEVICE_JSON, "not-enabled"),
			shouldPowercycle: false,
		},
		"JustBotInList": {
			bot:              mockBotAndId(testdata.MISSING_DEVICE_JSON, "bot-002"),
			shouldPowercycle: false,
		},
		"JustDeviceInList": {
			bot:              mockBotAndId(testdata.MISSING_DEVICE_JSON, "bot-003"),
			shouldPowercycle: true,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{
			"bot-001",
			"bot-001-device",
			"bot-002",
			"bot-003-device",
		}),
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleDevice(c.bot))
		})
	}
}
