package swarming_metrics

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	metrics_util "go.skia.org/infra/go/metrics2/testutils"
)

const (
	MOCK_POOL   = "SomePool"
	MOCK_SERVER = "SomeServer"
)

type expectations struct {
	botID         string
	quarantined   bool
	isDead        bool
	lastSeenDelta time.Duration
}

// getPromClient creates a fresh Prometheus Registry and
// a fresh Prometheus Client. This wipes out all previous metrics.
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
			FirstSeenTs: now.Add(-24 * time.Hour).Format("2006-01-02T15:04:05"),
		})
	}

	ms.On("ListBotsForPool", MOCK_POOL).Return(b, nil)
	ms.On("ListBotTasks", mock.AnythingOfType("string"), 1).Return([]*swarming_api.SwarmingRpcsTaskResult{}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(now, ms, pc, MOCK_POOL, MOCK_SERVER)
	assert.NoError(t, err)
	assert.Len(t, newMetrics, 9, "3 bots * 3 metrics each = 9 expected metrics")

	for _, e := range ex {
		tags := map[string]string{
			"bot":      e.botID,
			"pool":     MOCK_POOL,
			"swarming": MOCK_SERVER,
		}
		// even though this is a (really big) int, JSON notation returns scientific notation
		// for large enough ints, which means we need to ParseFloat, the only parser we have
		// that can read Scientific notation.
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, MEASUREMENT_SWARM_BOTS_LAST_SEEN, tags), 64)
		assert.NoError(t, err)
		assert.Equalf(t, int64(e.lastSeenDelta), int64(actual), "Wrong last seen time for metric %s", MEASUREMENT_SWARM_BOTS_LAST_SEEN)

		actual, err = strconv.ParseFloat(metrics_util.GetRecordedMetric(t, "swarming_bots_quarantined", tags), 64)
		assert.NoError(t, err)
		expected := 0
		if e.quarantined {
			expected = 1
		}
		assert.Equalf(t, int64(expected), int64(actual), "Wrong last seen time for metric %s", MEASUREMENT_SWARM_BOTS_QUARANTINED)
	}
}

func TestLastTaskBotMetrics(t *testing.T) {
	testutils.SmallTest(t)

	ms := swarming.NewMockApiClient()
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ms.On("ListBotsForPool", MOCK_POOL).Return([]*swarming_api.SwarmingRpcsBotInfo{
		{
			BotId:       "my-bot",
			LastSeenTs:  now.Add(-time.Minute).Format("2006-01-02T15:04:05"),
			IsDead:      false,
			Quarantined: false,
		},
	}, nil)

	ms.On("ListBotTasks", "my-bot", 1).Return([]*swarming_api.SwarmingRpcsTaskResult{
		{
			ModifiedTs: now.Add(-31 * time.Minute).Format("2006-01-02T15:04:05"),
		},
	}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(now, ms, pc, MOCK_POOL, MOCK_SERVER)
	assert.NoError(t, err)
	assert.Len(t, newMetrics, 3, "1 bot * 3 metrics = 3 expected metrics")

	tags := map[string]string{
		"bot":      "my-bot",
		"pool":     MOCK_POOL,
		"swarming": MOCK_SERVER,
	}
	// even though this is a (really big) int, JSON notation returns scientific notation
	// for large enough ints, which means we need to ParseFloat, the only parser we have
	// that can read Scientific notation.
	actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, MEASUREMENT_SWARM_BOTS_LAST_TASK, tags), 64)
	assert.NoError(t, err)
	assert.Equalf(t, int64(31*time.Minute), int64(actual), "Wrong last seen time for metric %s", MEASUREMENT_SWARM_BOTS_LAST_TASK)

}
