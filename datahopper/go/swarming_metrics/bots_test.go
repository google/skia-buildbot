package swarming_metrics

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	assert "github.com/stretchr/testify/require"
	metrics_util "go.skia.org/infra/go/metrics2/testutils"
)

type expectations struct {
	botID         string
	quarantined   bool
	isDead        bool
	lastSeenDelta time.Duration
}

func getPromClient() metrics2.Client {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return metrics2.NewPromClient()
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
			isDead:        true,
			lastSeenDelta: 18 * time.Minute,
		},
		{
			botID:         "bot-b",
			quarantined:   true,
			isDead:        false,
			lastSeenDelta: 3 * time.Minute,
		},
		{
			botID:         "bot-c",
			quarantined:   false,
			isDead:        false,
			lastSeenDelta: 1 * time.Minute,
		},
	}

	b := []*swarming_api.SwarmingRpcsBotInfo{}
	for _, e := range ex {
		b = append(b, &swarming_api.SwarmingRpcsBotInfo{
			BotId:       e.botID,
			LastSeenTs:  now.Add(-e.lastSeenDelta).Format("2006-01-02T15:04:05"),
			IsDead:      e.isDead,
			Quarantined: e.quarantined,
		})
	}

	ms.On("ListBotsForPool", "SomePool").Return(b, nil)

	pc := getPromClient()

	reportBotMetrics(now, ms, pc, "SomePool", "SomeServer")

	for _, e := range ex {
		metric := fmt.Sprintf(`swarming_bots_last_seen{bot="%s",pool="SomePool",swarming="SomeServer"}`, e.botID)
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, metric), 64)
		assert.NoError(t, err)
		assert.Equalf(t, int64(e.lastSeenDelta), int64(actual), "Wrong last seen time for metric %s", metric)

		metric = fmt.Sprintf(`swarming_bots_quarantined{bot="%s",pool="SomePool",swarming="SomeServer"}`, e.botID)
		actual, err = strconv.ParseFloat(metrics_util.GetRecordedMetric(t, metric), 64)
		assert.NoError(t, err)
		expected := 0
		if e.quarantined {
			expected = 1
		}
		assert.Equalf(t, int64(expected), int64(actual), "Wrong last seen time for metric %s", metric)
	}
}
