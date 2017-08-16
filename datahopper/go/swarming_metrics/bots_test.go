package swarming_metrics

import (
	"testing"
	"time"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	metrics2_mocks "go.skia.org/infra/go/metrics2/mocks"
)

type expectations struct {
	botID         string
	quarantined   bool
	isdead        bool
	lastseenDelta time.Duration
}

func TestDeadQuarantinedBotMetrics(t *testing.T) {
	testutils.SmallTest(t)

	ms := swarming.NewMockApiClient()
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ex := []expectations{
		{
			botID:         "bot-a",
			quarantined:   false,
			isdead:        true,
			lastseenDelta: 18 * time.Minute,
		},
		{
			botID:         "bot-b",
			quarantined:   true,
			isdead:        false,
			lastseenDelta: 3 * time.Minute,
		},
		{
			botID:         "bot-c",
			quarantined:   false,
			isdead:        false,
			lastseenDelta: 1 * time.Minute,
		},
	}

	b := []*swarming_api.SwarmingRpcsBotInfo{}
	for _, e := range ex {
		b = append(b, &swarming_api.SwarmingRpcsBotInfo{
			BotId:       e.botID,
			LastSeenTs:  now.Add(-e.lastseenDelta).Format("2006-01-02T15:04:05"),
			IsDead:      e.isdead,
			Quarantined: e.quarantined,
		})
	}

	ms.On("ListBotsForPool", "SomePool").Return(b, nil)

	mc := &metrics2_mocks.Client{}
	defer mc.AssertExpectations(t)

	for _, e := range ex {
		mim := &metrics2_mocks.Int64Metric{}
		mim.On("Update", int64(e.lastseenDelta)).Return()
		defer mim.AssertExpectations(t)

		mc.On("GetInt64Metric", MEASUREMENT_SWARM_BOTS_LAST_SEEN, map[string]string{
			"bot":      e.botID,
			"pool":     "SomePool",
			"swarming": "SomeServer",
		}).Return(mim)

		mim2 := &metrics2_mocks.Int64Metric{}
		if e.quarantined {
			mim2.On("Update", int64(1)).Return()
		} else {
			mim2.On("Update", int64(0)).Return()
		}

		defer mim2.AssertExpectations(t)

		mc.On("GetInt64Metric", MEASUREMENT_SWARM_BOTS_QUARANTINED, map[string]string{
			"bot":      e.botID,
			"pool":     "SomePool",
			"swarming": "SomeServer",
		}).Return(mim2)
	}

	reportBotMetrics(now, ms, mc, "SomePool", "SomeServer")
}
