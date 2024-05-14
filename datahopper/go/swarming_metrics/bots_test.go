package swarming_metrics

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/metrics2"
	metrics_util "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/go/testutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MOCK_POOL   = "SomePool"
	MOCK_SERVER = "SomeServer"
)

// getPromClient creates a fresh Prometheus Registry and
// a fresh Prometheus Client. This wipes out all previous metrics.
func getPromClient() metrics2.Client {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	return metrics2.NewPromClient()
}

func TestDeadQuarantinedBotMetrics(t *testing.T) {
	ctx := context.Background()
	ms := &mocks.SwarmingV2Client{}
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	type expectations struct {
		botID         string
		quarantined   bool
		isDead        bool
		lastSeenDelta time.Duration
		dimensions    map[string][]string
	}
	ex := []expectations{
		{
			botID:         "bot-a",
			quarantined:   false,
			isDead:        true,
			lastSeenDelta: 18 * time.Minute,
			dimensions: map[string][]string{
				swarming.DIMENSION_OS_KEY:          {"Android"},
				swarming.DIMENSION_DEVICE_TYPE_KEY: {"Nexus5x"},
				swarming.DIMENSION_DEVICE_OS_KEY:   {"P", "PPR1.180610.009"},
				swarming.DIMENSION_QUARANTINED_KEY: {"Device Missing"},
			},
		},
		{
			botID:         "bot-b",
			quarantined:   true,
			isDead:        false,
			lastSeenDelta: 3 * time.Minute,
			dimensions: map[string][]string{
				swarming.DIMENSION_OS_KEY: {"Linux", "Debian-9.8"},
			},
		},
		{
			botID:         "bot-c",
			quarantined:   false,
			isDead:        false,
			lastSeenDelta: 1 * time.Minute,
			dimensions: map[string][]string{
				swarming.DIMENSION_OS_KEY: {"Windows", "Windows-10"},
			},
		},
	}

	b := []*apipb.BotInfo{}
	for _, e := range ex {
		dims := make([]*apipb.StringListPair, 0, len(e.dimensions))
		for k, v := range e.dimensions {
			dims = append(dims, &apipb.StringListPair{
				Key:   k,
				Value: v,
			})
		}
		b = append(b, &apipb.BotInfo{
			BotId:       e.botID,
			LastSeenTs:  timestamppb.New(now.Add(-e.lastSeenDelta)),
			IsDead:      e.isDead,
			Quarantined: e.quarantined,
			FirstSeenTs: timestamppb.New(now.Add(-24 * time.Hour)),
			Dimensions:  dims,
		})
	}

	ms.On("ListBots", testutils.AnyContext, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: swarming.DIMENSION_POOL_KEY, Value: MOCK_POOL},
		},
		Limit: 1000,
	}).Return(&apipb.BotInfoListResponse{
		Items: b,
	}, nil)
	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "bot-a",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)
	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "bot-b",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)
	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "bot-c",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(ctx, now, ms, pc, MOCK_POOL, MOCK_SERVER)
	require.NoError(t, err)
	require.Len(t, newMetrics, 21, "3 bots * 7 metrics each = 21 expected metrics")

	for _, e := range ex {
		tags := map[string]string{
			"bot":      e.botID,
			"pool":     MOCK_POOL,
			"swarming": MOCK_SERVER,
		}
		for _, d := range []string{
			swarming.DIMENSION_OS_KEY,
			swarming.DIMENSION_DEVICE_TYPE_KEY,
			swarming.DIMENSION_DEVICE_OS_KEY,
			swarming.DIMENSION_GPU_KEY,
			swarming.DIMENSION_QUARANTINED_KEY,
		} {
			tags[d] = ""
			if len(e.dimensions[d]) > 0 {
				tags[d] = e.dimensions[d][len(e.dimensions[d])-1]
			}
		}

		// even though this is a (really big) int, JSON notation returns scientific notation
		// for large enough ints, which means we need to ParseFloat, the only parser we have
		// that can read Scientific notation.
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, measurementSwarmingBotsLastSeen, tags), 64)
		require.NoError(t, err)
		require.Equalf(t, int64(e.lastSeenDelta), int64(actual), "Wrong last seen time for metric %s", measurementSwarmingBotsLastSeen)

		toCheck := []string{"too_hot", "low_battery", "available", "<none>"}
		for _, extraTag := range toCheck {
			tags["device_state"] = extraTag
			actual, err = strconv.ParseFloat(metrics_util.GetRecordedMetric(t, "swarming_bots_quarantined", tags), 64)
			require.NoError(t, err)
			expected := 0
			if e.quarantined && extraTag == "<none>" {
				expected = 1
			}
			require.Equalf(t, int64(expected), int64(actual), "Wrong is quarantined for metric %s + tag %s", measurementSwarmingBotsQuarantined, extraTag)
		}

	}
}

func TestLastTaskBotMetrics(t *testing.T) {

	ctx := context.Background()
	ms := &mocks.SwarmingV2Client{}
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ms.On("ListBots", testutils.AnyContext, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: swarming.DIMENSION_POOL_KEY, Value: MOCK_POOL},
		},
		Limit: 1000,
	}).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{
				BotId:       "my-bot",
				LastSeenTs:  timestamppb.New(now.Add(-time.Minute)),
				IsDead:      false,
				Quarantined: false,
				Dimensions: []*apipb.StringListPair{
					{
						Key:   swarming.DIMENSION_OS_KEY,
						Value: []string{"Android"},
					},
					{
						Key:   swarming.DIMENSION_DEVICE_TYPE_KEY,
						Value: []string{"Nexus5x"},
					},
					{
						Key:   swarming.DIMENSION_DEVICE_OS_KEY,
						Value: []string{"P", "PPR1.180610.009"},
					},
					{
						Key:   swarming.DIMENSION_GPU_KEY,
						Value: []string{"102b:0534"},
					},
					{
						Key:   swarming.DIMENSION_QUARANTINED_KEY,
						Value: []string{"Device Missing"},
					},
				},
			},
		},
	}, nil)

	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "my-bot",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{
			{
				ModifiedTs: timestamppb.New(now.Add(-31 * time.Minute)),
			},
		},
	}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(ctx, now, ms, pc, MOCK_POOL, MOCK_SERVER)
	require.NoError(t, err)
	require.Len(t, newMetrics, 7, "1 bot * 7 metrics = 7 expected metrics")

	tags := map[string]string{
		"bot":                              "my-bot",
		"pool":                             MOCK_POOL,
		"swarming":                         MOCK_SERVER,
		swarming.DIMENSION_OS_KEY:          "Android",
		swarming.DIMENSION_DEVICE_TYPE_KEY: "Nexus5x",
		swarming.DIMENSION_DEVICE_OS_KEY:   "PPR1.180610.009",
		swarming.DIMENSION_GPU_KEY:         "102b:0534",
		swarming.DIMENSION_QUARANTINED_KEY: "Device Missing",
	}
	// even though this is a (really big) int, JSON notation returns scientific notation
	// for large enough ints, which means we need to ParseFloat, the only parser we have
	// that can read Scientific notation.
	actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, measurementSwarmingBotsLastTask, tags), 64)
	require.NoError(t, err)
	require.Equalf(t, int64(31*time.Minute), int64(actual), "Wrong last seen time for metric %s", measurementSwarmingBotsLastTask)

}

func TestBotTemperatureMetrics(t *testing.T) {

	ctx := context.Background()
	ms := &mocks.SwarmingV2Client{}
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ms.On("ListBots", testutils.AnyContext, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: swarming.DIMENSION_POOL_KEY, Value: MOCK_POOL},
		},
		Limit: 1000,
	}).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{
				BotId:      "my-bot-no-temp",
				LastSeenTs: timestamppb.New(now.Add(-3 * time.Minute)),
				State:      `{}`,
			},
			{
				BotId:      "my-bot-no-device",
				LastSeenTs: timestamppb.New(now.Add(-2 * time.Minute)),
				State:      `{"temp": {"thermal_zone0": 27.8,"thermal_zone1": 29.8,"thermal_zone2": 36}}`,
			},
			{
				BotId:      "my-bot-device",
				LastSeenTs: timestamppb.New(now.Add(-time.Minute)),
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
		},
	}, nil)

	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "my-bot-no-temp",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)
	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "my-bot-no-device",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)
	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "my-bot-device",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)

	pc := getPromClient()

	newMetrics, err := reportBotMetrics(ctx, now, ms, pc, MOCK_POOL, MOCK_SERVER)
	require.NoError(t, err)
	require.Len(t, newMetrics, 34, "24 bot metrics + 10 temp metrics = 31 expected metrics")

	expected := map[string]float64{
		"thermal_zone0": 28.0,
		"thermal_zone1": 30.0,
		"thermal_zone2": 36.0,
	}
	for z, v := range expected {
		tags := map[string]string{
			"bot":                              "my-bot-no-device",
			"pool":                             MOCK_POOL,
			"swarming":                         MOCK_SERVER,
			"temp_zone":                        z,
			swarming.DIMENSION_OS_KEY:          "",
			swarming.DIMENSION_DEVICE_TYPE_KEY: "",
			swarming.DIMENSION_DEVICE_OS_KEY:   "",
			swarming.DIMENSION_GPU_KEY:         "",
			swarming.DIMENSION_QUARANTINED_KEY: "",
		}
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, measurementSwarmingBotsDeviceTemp, tags), 64)
		require.NoError(t, err)
		require.Equalf(t, v, actual, "Wrong temperature seen for metric %s - %s", measurementSwarmingBotsDeviceTemp, z)
	}

	expected = map[string]float64{
		"battery_direct":   25.0,
		"merble":           2879.0,
		"gerble":           40.0,
		"battery":          26.0,
		"thermal_zone0":    43.0,
		"tsens_tz_sensor1": 37.0,
		"tsens_tz_sensor2": 41.0,
	}
	for z, v := range expected {
		tags := map[string]string{
			"bot":                              "my-bot-device",
			"pool":                             MOCK_POOL,
			"swarming":                         MOCK_SERVER,
			"temp_zone":                        z,
			swarming.DIMENSION_OS_KEY:          "",
			swarming.DIMENSION_DEVICE_TYPE_KEY: "",
			swarming.DIMENSION_DEVICE_OS_KEY:   "",
			swarming.DIMENSION_GPU_KEY:         "",
			swarming.DIMENSION_QUARANTINED_KEY: "",
		}
		actual, err := strconv.ParseFloat(metrics_util.GetRecordedMetric(t, measurementSwarmingBotsDeviceTemp, tags), 64)
		require.NoError(t, err)
		require.Equalf(t, v, actual, "Wrong temperature seen for metric %s - %s", measurementSwarmingBotsDeviceTemp, z)
	}
}

func TestBotUptimeMetrics(t *testing.T) {

	ctx := context.Background()
	ms := &mocks.SwarmingV2Client{}
	defer ms.AssertExpectations(t)

	now := time.Date(2017, 9, 1, 12, 0, 0, 0, time.UTC)

	ms.On("ListBots", testutils.AnyContext, &apipb.BotsRequest{
		Dimensions: []*apipb.StringPair{
			{Key: swarming.DIMENSION_POOL_KEY, Value: MOCK_POOL},
		},
		Limit: 1000,
	}).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{
				BotId:      "my-bot",
				LastSeenTs: timestamppb.New(now.Add(-2 * time.Minute)),
				State:      `{"uptime": 153}`,
			},
		},
	}, nil)

	ms.On("ListBotTasks", testutils.AnyContext, &apipb.BotTasksRequest{
		BotId: "my-bot",
		Limit: 1,
		State: apipb.StateQuery_QUERY_ALL,
	}).Return(&apipb.TaskListResponse{}, nil)

	pc := getPromClient()

	_, err := reportBotMetrics(ctx, now, ms, pc, MOCK_POOL, MOCK_SERVER)
	require.NoError(t, err)

	tags := map[string]string{}
	actual := metrics_util.GetRecordedMetric(t, measurementSwarmingBotsUptime, tags)
	require.Equal(t, "153", actual)
}
