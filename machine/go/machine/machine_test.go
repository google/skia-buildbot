package machine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testTime = time.Date(2020, 1, 1, 1, 1, 1, 1, time.UTC)

func TestDupDescription(t *testing.T) {
	in := Description{
		Mode:       ModeAvailable,
		Annotation: Annotation{},
		Dimensions: SwarmingDimensions{
			"foo": []string{"bar"},
		},
		LastUpdated: testTime,
	}
	out := DupDescription(in)
	require.Equal(t, in, out)

	// Confirm that the two Dimensions are separate.
	in.Dimensions["baz"] = []string{"quux"}
	require.NotEqual(t, in, out)
}
