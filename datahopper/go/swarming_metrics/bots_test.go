package swarming_metrics

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
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
	assert.Len(t, newMetrics, 18, "3 bots * 6 metrics each = 18 expected metrics")

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

		toCheck := []string{"too_hot", "low_battery", "available", "<none>"}
		for _, extraTag := range toCheck {
			tags["deviceState"] = extraTag
			actual, err = strconv.ParseFloat(metrics_util.GetRecordedMetric(t, "swarming_bots_quarantined", tags), 64)
			assert.NoError(t, err)
			expected := 0
			if e.quarantined && extraTag == "<none>" {
				expected = 1
			}
			assert.Equalf(t, int64(expected), int64(actual), "Wrong is quarantined for metric %s + tag %s", MEASUREMENT_SWARM_BOTS_QUARANTINED, extraTag)
		}

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
	assert.Len(t, newMetrics, 6, "1 bot * 6 metrics = 6 expected metrics")

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

func TestBotTemperatureMetrics(t *testing.T) {
	testutils.SmallTest(t)

	ms := swarming.NewMockApiClient()
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ms.On("ListBotsForPool", MOCK_POOL).Return([]*swarming_api.SwarmingRpcsBotInfo{
		{
			BotId:      "my-bot-no-temp",
			LastSeenTs: now.Add(-3 * time.Minute).Format("2006-01-02T15:04:05"),
			State:      `{}`,
		},
		{
			BotId:      "my-bot-no-device",
			LastSeenTs: now.Add(-2 * time.Minute).Format("2006-01-02T15:04:05"),
			State:      `{"temp": {"thermal_zone0": 27.8,"thermal_zone1": 29.8,"thermal_zone2": 36}}`,
		},
		{
			BotId:      "my-bot-device",
			LastSeenTs: now.Add(-time.Minute).Format("2006-01-02T15:04:05"),
			State: `{
				"temp": {"thermal_zone0": 42.5000000000000000000000000000001},
				"devices": {
						"abcdefg": {
							"battery": {
								"power": ["USB"],
								"temperature": 248
							},
							"temp": {
								"merble": 2878.9,
								"gerble": 40.03,
								"battery": 26,
								"tsens_tz_sensor1": 37,
								"tsens_tz_sensor2": 412,
								"max77621-gpu": 100,
								"dram": 2
							},
							"state": "too_hot"
						}
					}
				}`,
		},
	}, nil)

	ms.On("ListBotTasks", mock.AnythingOfType("string"), 1).Return([]*swarming_api.SwarmingRpcsTaskResult{
		{
			ModifiedTs: now.Add(-31 * time.Minute).Format("2006-01-02T15:04:05"),
		},
	}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(now, ms, pc, MOCK_POOL, MOCK_SERVER)
	assert.NoError(t, err)
	assert.Len(t, newMetrics, 28, "18 bot metrics + 10 temp metrics = 28 expected metrics")

	expected := map[string]int64{
		"thermal_zone0": 28,
		"thermal_zone1": 30,
		"thermal_zone2": 36,
	}
	for z, v := range expected {
		tags := map[string]string{
			"bot":       "my-bot-no-device",
			"pool":      MOCK_POOL,
			"swarming":  MOCK_SERVER,
			"temp_zone": z,
		}
		actual, err := strconv.ParseInt(metrics_util.GetRecordedMetric(t, MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tags), 10, 64)
		assert.NoError(t, err)
		assert.Equalf(t, v, int64(actual), "Wrong temperature seen for metric %s - %s", MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, z)
	}

	expected = map[string]int64{
		"battery_direct":   25,
		"merble":           2879,
		"gerble":           40,
		"battery":          26,
		"thermal_zone0":    43,
		"tsens_tz_sensor1": 37,
		"tsens_tz_sensor2": 41,
	}
	for z, v := range expected {
		tags := map[string]string{
			"bot":       "my-bot-device",
			"pool":      MOCK_POOL,
			"swarming":  MOCK_SERVER,
			"temp_zone": z,
		}
		actual, err := strconv.ParseInt(metrics_util.GetRecordedMetric(t, MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tags), 10, 64)
		assert.NoError(t, err)
		assert.Equalf(t, v, int64(actual), "Wrong temperature seen for metric %s - %s", MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, z)
	}

}
