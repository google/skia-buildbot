package digesttools

import (
	"math"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCombinedDiffMetric(t *testing.T) {
	unittest.SmallTest(t)
	assert.InDelta(t, 1.0, combinedDiffMetric(0.0, []int{}), 0.000001)
	assert.InDelta(t, 1.0, combinedDiffMetric(1.0, []int{255, 255, 255, 255}), 0.000001)
	assert.InDelta(t, math.Sqrt(0.5), combinedDiffMetric(0.5, []int{255, 255, 255, 255}), 0.000001)
}
