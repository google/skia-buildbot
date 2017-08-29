package gatherer

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/promalertsclient"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/power/go/decider"
	"go.skia.org/infra/power/go/testdata"
)

var cycleTests = map[string]func(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider){
	"NoBots":              testNoBotsCycle,
	"NoAlertingBots":      testNoAlertingBots,
	"OneMissingBot":       testOneMissingBot,
	"OneSilencedBot":      testOneSilencedBot,
	"ThreeMissingDevices": testThreeMissingDevices,
}

// To cut down on the boilerplate of setting up the various mocks and asserting expectations, we (ab)use Go's ability to have "subtests". This allows us to make our test functions take the mocks as input and for us to assert the expectations after it completes. The asserting of the expectations after the test is why we cannot easily have just a setup function that all the tests call to make the mocks. Additionally, the use of package level variables for the mocks is not thread-safe if tests are run in parallel.
// https://golang.org/pkg/testing/#hdr-Subtests_and_Sub_benchmarks
func TestCycle(t *testing.T) {
	testutils.SmallTest(t)
	for name, test := range cycleTests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			// mi = "mock internal" client
			mi := skswarming.NewMockApiClient()
			// me = "mock external" client
			me := skswarming.NewMockApiClient()
			// ma = "mock alerts" client
			ma := promalertsclient.NewMockClient()
			md := decider.NewMockDecider()
			defer mi.AssertExpectations(t)
			defer me.AssertExpectations(t)
			defer ma.AssertExpectations(t)
			defer md.AssertExpectations(t)
			test(t, mi, me, ma, md)
		})
	}
}

func testNoBotsCycle(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC))

	// There is a bit of whitebox testing here.  We can't mock out a call to GetAlerts if it won't be called.
	g := NewPollingGatherer(me, mi, ma, md, nil, 0).(*gatherer)
	g.update()

	bots := g.DownBots()
	assert.Empty(t, bots, "There should be no bots to reboot, because swarming doesn't detect any are down.")
}

func testNoAlertingBots(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-046"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Times(len(skswarming.POOLS_PUBLIC))

	ma.On("GetAlerts", mock.AnythingOfType("func(promalertsclient.Alert) bool")).Return([]promalertsclient.Alert{}, nil).Once()

	g := NewPollingGatherer(me, mi, ma, md, nil, 0).(*gatherer)
	g.update()

	bots := g.DownBots()
	assert.Empty(t, bots, "There should be no bots to reboot, because alerts says none are down.")
}

func testOneMissingBot(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts", mock.AnythingOfType("func(promalertsclient.Alert) bool")).Return([]promalertsclient.Alert{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 30*time.Minute),
	}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(true)

	hostMap := map[string]string{
		"skia-rpi-046": "jumphost-rpi-01",
	}

	g := NewPollingGatherer(me, mi, ma, md, hostMap, 0).(*gatherer)
	g.update()

	bots := g.DownBots()
	assert.Len(t, bots, 1, "There should be 1 bot to reboot.")
	assert.Equal(t, "skia-rpi-046", bots[0].BotID, "That bot should be skia-rpi-046")
	assert.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	assert.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
	assert.Equal(t, "2017-05-04T11:30:00Z", bots[0].Since.Format(time.RFC3339))
	assert.False(t, bots[0].Silenced, "Bot should be silenced")
}

func testOneSilencedBot(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	silenced := mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 30*time.Minute)
	silenced.Silenced = true
	ma.On("GetAlerts", mock.AnythingOfType("func(promalertsclient.Alert) bool")).Return([]promalertsclient.Alert{
		silenced,
	}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(true)

	hostMap := map[string]string{
		"skia-rpi-046": "jumphost-rpi-01",
	}

	g := NewPollingGatherer(me, mi, ma, md, hostMap, 0).(*gatherer)
	g.update()

	bots := g.DownBots()
	assert.Len(t, bots, 1, "There should be 1 bot to reboot.")
	assert.Equal(t, "skia-rpi-046", bots[0].BotID, "That bot should be skia-rpi-046")
	assert.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	assert.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
	assert.Equal(t, "2017-05-04T11:30:00Z", bots[0].Since.Format(time.RFC3339))
	assert.True(t, bots[0].Silenced, "Bot should be silenced")
}

func testThreeMissingDevices(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, md *decider.MockDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-003"),
		testdata.MockBotAndId(t, testdata.USB_FAILURE, "skia-rpi-002"),
		testdata.MockBotAndId(t, testdata.USB_FAILURE, "skia-rpi-120"),
		testdata.MockBotAndId(t, testdata.TOO_HOT, "skia-rpi-121"),
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-vm-001"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts", mock.AnythingOfType("func(promalertsclient.Alert) bool")).Return([]promalertsclient.Alert{
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-003", 65*time.Minute),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-001", 25*time.Minute),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-002", 11*time.Minute), // This one has a usb failure, which is sometimes fixed by a powercycle
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-121", 10*time.Minute), // This one is too hot, and should not be offered for a reboot
	}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(false)
	md.On("ShouldPowercycleDevice", mock.MatchedBy(func(bot *swarming.SwarmingRpcsBotInfo) bool {
		return bot.BotId == "skia-rpi-121"
	})).Return(false)
	md.On("ShouldPowercycleDevice", mock.Anything).Return(true)

	hostMap := map[string]string{
		"skia-rpi-001-device": "jumphost-rpi-01",
		"skia-rpi-002-device": "jumphost-rpi-01",
		"skia-rpi-003-device": "jumphost-rpi-02",
		"skia-rpi-121":        "NOT_USED",
	}

	g := NewPollingGatherer(me, mi, ma, md, hostMap, 0).(*gatherer)
	g.update()

	bots := g.DownBots()
	assert.Len(t, bots, 3, "There should be 3 devices to reboot.")
	assert.Equal(t, "skia-rpi-001", bots[0].BotID, "These should be sorted alphabetically")
	assert.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	assert.Equal(t, "skia-rpi-002", bots[1].BotID, "These should be sorted alphabetically")
	assert.Equal(t, "jumphost-rpi-01", bots[1].HostID)
	assert.Equal(t, "skia-rpi-003", bots[2].BotID, "These should be sorted alphabetically")
	assert.Equal(t, "jumphost-rpi-02", bots[2].HostID)
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[0].Status)
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[1].Status)
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[2].Status)
	assert.Equal(t, "2017-05-04T11:35:00Z", bots[0].Since.Format(time.RFC3339))
	assert.Equal(t, "2017-05-04T11:49:00Z", bots[1].Since.Format(time.RFC3339))
	assert.Equal(t, "2017-05-04T10:55:00Z", bots[2].Since.Format(time.RFC3339))
}

func mockAPIAlert(alertname, bot string, ago time.Duration) promalertsclient.Alert {
	baseTime := time.Date(2017, time.May, 4, 12, 00, 0, 0, time.UTC)
	a := promalertsclient.Alert{
		Alert: model.Alert{
			Labels: model.LabelSet{},
		},
	}
	a.Labels["alertname"] = model.LabelValue(alertname)
	a.Labels["bot"] = model.LabelValue(bot)
	a.StartsAt = baseTime.Add(-ago)
	return a
}
