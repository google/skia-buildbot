package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
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
		PodName:             "rpi-swarming-1235-987",
		KubernetesImage:     "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean",
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
