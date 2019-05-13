package decider

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/power/go/testdata"
)

type testcase struct {
	bot              *swarming.SwarmingRpcsBotInfo
	shouldPowercycle bool
}

const MOCK_BOT_ID = "some-bot"

func TestShouldPowercycleBot(t *testing.T) {
	unittest.SmallTest(t)
	// This test assumes all the bots involved are powercyclable
	tests := map[string]testcase{
		"TooHot": {
			bot:              mockBot(t, testdata.TOO_HOT),
			shouldPowercycle: false,
		},
		"USBFailure": {
			bot:              mockBot(t, testdata.USB_FAILURE),
			shouldPowercycle: false,
		},
		"MissingDevice": {
			bot:              mockBot(t, testdata.MISSING_DEVICE),
			shouldPowercycle: false,
		},
		"DeadBot": {
			bot:              mockBot(t, testdata.DEAD_BOT),
			shouldPowercycle: true,
		},
		"DeadQuarantinedBot": {
			bot:              mockBot(t, testdata.DEAD_AND_QUARANTINED),
			shouldPowercycle: true,
		},
		"DeletedBot": {
			bot:              mockBot(t, testdata.DELETED_BOT),
			shouldPowercycle: false,
		},
		"BootingDevice": {
			bot:              mockBot(t, testdata.BOOTING_DEVICE),
			shouldPowercycle: false,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{MOCK_BOT_ID}),
	}
	for name, c := range tests {
		func(name string, c testcase) {
			t.Run(name, func(t *testing.T) {
				unittest.SmallTest(t)
				assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleBot(c.bot))
			})
		}(name, c)
	}
}

func TestShouldPowercycleDevice(t *testing.T) {
	unittest.SmallTest(t)
	// This test assumes all the bots involved are powercyclable
	tests := map[string]testcase{
		"TooHot": {
			bot:              mockBot(t, testdata.TOO_HOT),
			shouldPowercycle: false,
		},
		"USBFailure": {
			bot:              mockBot(t, testdata.USB_FAILURE),
			shouldPowercycle: true,
		},
		"MissingDevice": {
			bot:              mockBot(t, testdata.MISSING_DEVICE),
			shouldPowercycle: true,
		},
		"DeadBot": {
			bot:              mockBot(t, testdata.DEAD_BOT),
			shouldPowercycle: false,
		},
		"DeadQuarantinedBot": {
			bot:              mockBot(t, testdata.DEAD_AND_QUARANTINED),
			shouldPowercycle: false,
		},
		"DeletedBot": {
			bot:              mockBot(t, testdata.DELETED_BOT),
			shouldPowercycle: false,
		},
		"BootingDevice": {
			bot:              mockBot(t, testdata.BOOTING_DEVICE),
			shouldPowercycle: true,
		},
	}
	d := decider{
		enabledBots: util.NewStringSet([]string{MOCK_BOT_ID + "-device"}),
	}
	for name, c := range tests {
		func(name string, c testcase) {
			t.Run(name, func(t *testing.T) {
				unittest.SmallTest(t)
				assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleDevice(c.bot))
			})
		}(name, c)
	}
}

func mockBot(t *testing.T, filename string) *swarming.SwarmingRpcsBotInfo {
	return testdata.MockBotAndId(t, filename, MOCK_BOT_ID)
}

func TestIDBasedPowercycleBot(t *testing.T) {
	unittest.SmallTest(t)
	// This test tests the enabledBots logic
	tests := map[string]testcase{
		"SunnyDay": {
			bot:              testdata.MockBotAndId(t, testdata.DEAD_BOT, "bot-001"),
			shouldPowercycle: true,
		},
		"NotEnabled": {
			bot:              testdata.MockBotAndId(t, testdata.DEAD_BOT, "not-enabled"),
			shouldPowercycle: false,
		},
		"JustBotInList": {
			bot:              testdata.MockBotAndId(t, testdata.DEAD_BOT, "bot-002"),
			shouldPowercycle: true,
		},
		"JustDeviceInList": {
			bot:              testdata.MockBotAndId(t, testdata.DEAD_BOT, "bot-003"),
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
		func(name string, c testcase) {
			t.Run(name, func(t *testing.T) {
				unittest.SmallTest(t)
				assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleBot(c.bot))
			})
		}(name, c)
	}
}

func TestIDBasedPowercycleDevice(t *testing.T) {
	unittest.SmallTest(t)
	// This test tests the enabledBots logic
	tests := map[string]testcase{
		"SunnyDay": {
			bot:              testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "bot-001"),
			shouldPowercycle: true,
		},
		"NotEnabled": {
			bot:              testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "not-enabled"),
			shouldPowercycle: false,
		},
		"JustBotInList": {
			bot:              testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "bot-002"),
			shouldPowercycle: false,
		},
		"JustDeviceInList": {
			bot:              testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "bot-003"),
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
		func(name string, c testcase) {
			t.Run(name, func(t *testing.T) {
				unittest.SmallTest(t)
				assert.Equal(t, c.shouldPowercycle, d.ShouldPowercycleDevice(c.bot))
			})
		}(name, c)
	}
}
