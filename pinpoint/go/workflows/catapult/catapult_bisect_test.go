package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/pinpoint/go/compare"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

func TestUpdateStatesWithComparisons_LessThanOneState_Error(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{{}}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	assert.Error(t, err)
}

func TestUpdateStatesWithComparisons_OneComparison_Same(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
	}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	require.NoError(t, err)
	assert.Empty(t, states[0].Comparisons.Prev)
	assert.Equal(t, string(compare.Same), states[0].Comparisons.Next)
	assert.Equal(t, string(compare.Same), states[1].Comparisons.Prev)
	assert.Empty(t, states[1].Comparisons.Next)
}

func TestUpdateStatesWithComparisons_MultiComparison_Different(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
	}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	require.NoError(t, err)
	assert.Empty(t, states[0].Comparisons.Prev)
	assert.Equal(t, string(compare.Same), states[0].Comparisons.Next)
	assert.Equal(t, string(compare.Same), states[1].Comparisons.Prev)
	assert.Equal(t, string(compare.Different), states[1].Comparisons.Next)
	assert.Equal(t, string(compare.Different), states[2].Comparisons.Prev)
	assert.Empty(t, states[2].Comparisons.Next)
}
