package gatherer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	mock_alert_client "go.skia.org/infra/am/go/alertclient/mocks"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/silence"
	skswarming "go.skia.org/infra/go/swarming"
	mock_swarming_client "go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/power/go/decider"
	"go.skia.org/infra/power/go/recorder"
	"go.skia.org/infra/power/go/testdata"
	"go.skia.org/infra/skolo/go/powercycle"
)

var cycleTests = map[string]func(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder){
	"NoBots":              testNoBotsCycle,
	"NoAlertingBots":      testNoAlertingBots,
	"OneMissingBot":       testOneMissingBot,
	"OneSilencedBot":      testOneSilencedBot,
	"ThreeMissingDevices": testThreeMissingDevices,
	"DuplicateBots":       testDuplicateBots,
	"RecentlyDownBots":    testRecentlyDownBots,
}

// To cut down on the boilerplate of setting up the various mocks and asserting expectations, we (ab)use Go's ability to have "subtests". This allows us to make our test functions take the mocks as input and for us to assert the expectations after it completes. The asserting of the expectations after the test is why we cannot easily have just a setup function that all the tests call to make the mocks. Additionally, the use of package level variables for the mocks is not thread-safe if tests are run in parallel.
// https://golang.org/pkg/testing/#hdr-Subtests_and_Sub_benchmarks
func TestCycle(t *testing.T) {
	unittest.SmallTest(t)
	for name, test := range cycleTests {
		t.Run(name, func(t *testing.T) {
			unittest.SmallTest(t)
			// mi = "mock internal" client
			mi := &mock_swarming_client.ApiClient{}
			// me = "mock external" client
			me := &mock_swarming_client.ApiClient{}
			// ma = "mock alerts" client
			ma := &mock_alert_client.APIClient{}
			md := decider.NewMockDecider()
			mr := recorder.NewMockRecorder()
			defer mi.AssertExpectations(t)
			defer me.AssertExpectations(t)
			defer ma.AssertExpectations(t)
			defer md.AssertExpectations(t)
			defer mr.AssertExpectations(t)
			test(t, mi, me, ma, md, mr)
		})
	}
}

func setupMockRecorder(mr *recorder.MockRecorder) {
	mr.On("NewlyFixedBots", mock.Anything).Return()
	mr.On("NewlyDownBots", mock.Anything).Return()
}

func testNoBotsCycle(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC))

	// There is a bit of whitebox testing here.  We can't mock out a call to GetAlerts if it won't be called.
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, nil, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Empty(t, bots, "There should be no bots to reboot, because swarming doesn't detect any are down.")
}

func testNoAlertingBots(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-046"),
	}
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return(b, nil).Times(len(skswarming.POOLS_PUBLIC))

	ma.On("GetAlerts").Return([]incident.Incident{}, nil).Once()
	// The GetSilences call is skipped if there are no alerts.

	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, nil, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Empty(t, bots, "There should be no bots to reboot, because alerts says none are down.")
}

func testOneMissingBot(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
	}
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 30*time.Minute),
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(true)

	hostMap := map[powercycle.DeviceID]string{
		"skia-rpi-046": "jumphost-rpi-01",
	}

	setupMockRecorder(mr)
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, hostMap, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Len(t, bots, 1, "There should be 1 bot to reboot.")
	require.Equal(t, "skia-rpi-046", bots[0].BotID, "That bot should be skia-rpi-046")
	require.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	require.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
	require.Equal(t, "2017-05-04T11:30:00Z", bots[0].Since.Format(time.RFC3339))
	require.False(t, bots[0].Silenced, "Bot should be silenced")
}

func testOneSilencedBot(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
	}
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	silenced := mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 30*time.Minute)
	ma.On("GetAlerts").Return([]incident.Incident{
		silenced,
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{
		{
			Active: true,
			ParamSet: map[string][]string{
				"alertname": {ALERT_BOT_MISSING},
				"bot":       {"skia-rpi-046"},
			},
		},
	}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(true)

	hostMap := map[powercycle.DeviceID]string{
		"skia-rpi-046": "jumphost-rpi-01",
	}

	setupMockRecorder(mr)
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, hostMap, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Len(t, bots, 1, "There should be 1 bot to reboot.")
	require.Equal(t, "skia-rpi-046", bots[0].BotID, "That bot should be skia-rpi-046")
	require.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	require.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
	require.Equal(t, "2017-05-04T11:30:00Z", bots[0].Since.Format(time.RFC3339))
	require.True(t, bots[0].Silenced, "Bot should be silenced")
}

func testThreeMissingDevices(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-001"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-003"),
		testdata.MockBotAndId(t, testdata.USB_FAILURE, "skia-rpi-002"),
		testdata.MockBotAndId(t, testdata.USB_FAILURE, "skia-rpi-120"),
		testdata.MockBotAndId(t, testdata.TOO_HOT, "skia-rpi-121"),
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-vm-001"),
	}
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-003", 65*time.Minute),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-001", 25*time.Minute),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-002", 11*time.Minute), // This one has a usb failure, which is sometimes fixed by a powercycle
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-121", 10*time.Minute), // This one is too hot, and should not be offered for a reboot
	}, nil).Once()
	// Add a silence, but it doesn't match any of the above bots
	ma.On("GetSilences").Return([]silence.Silence{
		{
			Active: true,
			ParamSet: map[string][]string{
				"alertname": {ALERT_BOT_MISSING},
				"bot":       {"skia-rpi-046"},
			},
		},
	}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(false)
	md.On("ShouldPowercycleDevice", mock.MatchedBy(func(bot *swarming.SwarmingRpcsBotInfo) bool {
		return bot.BotId == "skia-rpi-121"
	})).Return(false)
	md.On("ShouldPowercycleDevice", mock.Anything).Return(true)

	hostMap := map[powercycle.DeviceID]string{
		"skia-rpi-001-device": "jumphost-rpi-01",
		"skia-rpi-002-device": "jumphost-rpi-01",
		"skia-rpi-003-device": "jumphost-rpi-02",
		"skia-rpi-121":        "NOT_USED",
	}

	setupMockRecorder(mr)
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, hostMap, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Len(t, bots, 3, "There should be 3 devices to reboot.")
	require.Equal(t, "skia-rpi-001", bots[0].BotID, "These should be sorted alphabetically")
	require.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	require.Equal(t, "skia-rpi-002", bots[1].BotID, "These should be sorted alphabetically")
	require.Equal(t, "jumphost-rpi-01", bots[1].HostID)
	require.Equal(t, "skia-rpi-003", bots[2].BotID, "These should be sorted alphabetically")
	require.Equal(t, "jumphost-rpi-02", bots[2].HostID)
	require.Equal(t, STATUS_DEVICE_MISSING, bots[0].Status)
	require.Equal(t, STATUS_DEVICE_MISSING, bots[1].Status)
	require.Equal(t, STATUS_DEVICE_MISSING, bots[2].Status)
	require.Equal(t, "2017-05-04T11:35:00Z", bots[0].Since.Format(time.RFC3339))
	require.Equal(t, "2017-05-04T11:49:00Z", bots[1].Since.Format(time.RFC3339))
	require.Equal(t, "2017-05-04T10:55:00Z", bots[2].Since.Format(time.RFC3339))
}

func testDuplicateBots(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	// ListDownBots will return a dead and quarantined bot twice. We need to dedupe it.
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_AND_QUARANTINED, "skia-rpi-113"),
		testdata.MockBotAndId(t, testdata.DEAD_AND_QUARANTINED, "skia-rpi-113"),
	}
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-113", 30*time.Minute),
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{}, nil).Once()

	md.On("ShouldPowercycleBot", mock.Anything).Return(true)

	hostMap := map[powercycle.DeviceID]string{
		"skia-rpi-113": "jumphost-rpi-01",
	}

	setupMockRecorder(mr)
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, hostMap, 0).(*gatherer)
	g.update(ctx)

	bots := g.DownBots()
	require.Len(t, bots, 1, "There should be 1 bot to reboot.")
	require.Equal(t, "skia-rpi-113", bots[0].BotID, "That bot should be skia-rpi-113")
	require.Equal(t, "jumphost-rpi-01", bots[0].HostID)
	require.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
	require.Equal(t, "2017-05-04T11:30:00Z", bots[0].Since.Format(time.RFC3339))
	require.False(t, bots[0].Silenced, "Bot should be silenced")
}

func testRecentlyDownBots(t *testing.T, mi, me *mock_swarming_client.ApiClient, ma *mock_alert_client.APIClient, md *decider.MockDecider, mr *recorder.MockRecorder) {
	ctx := context.Background()
	// Baseline - no bots down
	mi.On("ListDownBots", testutils.AnyContext, mock.Anything).Return([]*swarming.SwarmingRpcsBotInfo{}, nil)
	md.On("ShouldPowercycleBot", mock.Anything).Return(func(bot *swarming.SwarmingRpcsBotInfo) bool {
		if bot.BotId == "skia-rpi-047" {
			return false
		}
		return true
	})
	md.On("ShouldPowercycleDevice", mock.Anything).Return(true)

	// Only the first pool will have anything in it for this test.
	for i, pool := range skswarming.POOLS_PUBLIC {
		if i == 0 {
			me.On("ListDownBots", testutils.AnyContext, skswarming.POOLS_PUBLIC[0]).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Once()
			continue
		}
		me.On("ListDownBots", testutils.AnyContext, pool).Return([]*swarming.SwarmingRpcsBotInfo{}, nil)
	}

	setupMockRecorder(mr)
	g := NewPollingGatherer(ctx, me, mi, ma, md, mr, nil, 0).(*gatherer)
	g.update(ctx)

	// Step 1 - one bot down
	b := []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
	}
	me.On("ListDownBots", testutils.AnyContext, skswarming.POOLS_PUBLIC[0]).Return(b, nil).Once()

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 30*time.Minute),
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{}, nil).Once()

	g.update(ctx)

	bots := g.DownBots()
	require.Len(t, bots, 1, "There should be 1 bot to reboot.")
	mr.AssertCalled(t, "NewlyDownBots", []string{"skia-rpi-046"})
	mr.AssertCalled(t, "NewlyFixedBots", []string{})

	// Step 2 - two more bots down (3 bots total)
	b = []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-046"),
		testdata.MockBotAndId(t, testdata.MISSING_DEVICE, "skia-rpi-047"),
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-048"),
	}
	me.On("ListDownBots", testutils.AnyContext, skswarming.POOLS_PUBLIC[0]).Return(b, nil).Once()

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046", 35*time.Minute),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-047", 10*time.Minute),
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-048", 11*time.Minute),
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{}, nil).Once()

	g.update(ctx)

	bots = g.DownBots()
	require.Len(t, bots, 3, "There should be 3 bot to reboot.")
	mr.AssertCalled(t, "NewlyDownBots", []string{"skia-rpi-047-device", "skia-rpi-048"})
	mr.AssertCalled(t, "NewlyFixedBots", []string{})

	// Step 3 - two bots fixed, one new bot down (2 bots down total)
	b = []*swarming.SwarmingRpcsBotInfo{
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-020"),
		testdata.MockBotAndId(t, testdata.DEAD_BOT, "skia-rpi-048"),
	}
	me.On("ListDownBots", testutils.AnyContext, skswarming.POOLS_PUBLIC[0]).Return(b, nil).Once()

	ma.On("GetAlerts").Return([]incident.Incident{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-020", 1*time.Minute),
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-048", 15*time.Minute),
	}, nil).Once()
	ma.On("GetSilences").Return([]silence.Silence{}, nil).Once()

	g.update(ctx)

	bots = g.DownBots()
	require.Len(t, bots, 2, "There should be 2 bots to reboot.")
	mr.AssertCalled(t, "NewlyDownBots", []string{"skia-rpi-020"})
	mr.AssertCalled(t, "NewlyFixedBots", []string{"skia-rpi-046", "skia-rpi-047-device"})
}

func mockAPIAlert(alertname, bot string, ago time.Duration) incident.Incident {
	baseTime := time.Date(2017, time.May, 4, 12, 00, 0, 0, time.UTC)
	a := incident.Incident{
		Params: map[string]string{},
	}
	a.Params["alertname"] = alertname
	a.Params["bot"] = bot
	a.Start = baseTime.Add(-ago).Unix()
	return a
}
