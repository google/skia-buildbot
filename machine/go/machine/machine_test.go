package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
)

var testTime = time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)

var testDuration = int32(5) // Seconds

func TestCopy(t *testing.T) {
	unittest.SmallTest(t)
	in := Description{
		Mode: ModeAvailable,
		Annotation: Annotation{
			Message:   "take offline",
			User:      "barney@example.com",
			Timestamp: testTime,
		},
		Note: Annotation{
			Message:   "Battery swollen.",
			User:      "wilma@example.com",
			Timestamp: testTime,
		},
		Dimensions: SwarmingDimensions{
			"foo":   []string{"bar"},
			"alpha": []string{"beta", "gamma"},
		},
		SuppliedDimensions: SwarmingDimensions{
			"gpu": []string{"some-gpu"},
		},
		Version:             "v1.2",
		LastUpdated:         testTime,
		Battery:             91,
		Temperature:         map[string]float64{"cpu": 26.4},
		RunningSwarmingTask: true,
		LaunchedSwarming:    true,
		PowerCycle:          true,
		RecoveryStart:       testTime,
		DeviceUptime:        testDuration,
		SSHUserIP:           "root@skia-sparky360-03",
	}
	out := in.Copy()
	require.Equal(t, in, out)
	assertdeep.Copy(t, in, out)

	// Confirm that the two Dimensions are separate.
	in.Dimensions["baz"] = []string{"quux"}
	in.Dimensions["alpha"][0] = "zeta"
	require.NotEqual(t, in, out)
}

func TestAsMetricsTags_EmptyDimensions_ReturnsEmptyTags(t *testing.T) {
	unittest.SmallTest(t)
	emptyTags := map[string]string{
		DimID:         "",
		DimOS:         "",
		DimDeviceType: "",
	}
	assert.Equal(t, emptyTags, SwarmingDimensions{}.AsMetricsTags())
}

func TestAsMetricsTags_MultipleValues_ReturnsTagsWithMostSpecificValues(t *testing.T) {
	unittest.SmallTest(t)
	expected := map[string]string{
		DimID:         "",
		DimOS:         "iOS-13.6",
		DimDeviceType: "",
	}
	assert.Equal(t, expected, SwarmingDimensions{"os": []string{"iOS", "iOS-13.6"}}.AsMetricsTags())
}

func TestNewEvent(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, EventTypeRawState, NewEvent().EventType)
}

func TestNewDescription(t *testing.T) {
	unittest.SmallTest(t)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)
	ctx := now.TimeTravelingContext(serverTime)
	actual := NewDescription(ctx)
	expected := Description{
		Mode:        ModeAvailable,
		Dimensions:  SwarmingDimensions{},
		LastUpdated: serverTime,
	}
	assert.Equal(t, expected, actual)
}
