package diffstore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestMetricMapCodec(t *testing.T) {
	unittest.SmallTest(t)

	codec := MetricMapCodec{}

	// Initialize a dummy diff.DiffMetrics instance.
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    1,
		PixelDiffPercent: 0.5,
		MaxRGBADiffs:     []int{2, 3, 4, 5},
		DimDiffer:        true,
		Diffs: map[string]float32{
			"testMetric": 0.1,
		},
	}

	// Put diffMetrics into a map with an MD5 digest as the key.
	diffMap := make(map[types.Digest]interface{}, 0)
	testDigest := types.Digest("5460652359b9b272d520baaddaeddb5c")
	diffMap[testDigest] = diffMetrics

	// Encode the data.
	bytes, err := codec.Encode(diffMap)
	assert.NoError(t, err)

	// Decode the serialized data.
	data, err := codec.Decode(bytes)
	assert.NoError(t, err)

	// Verify the deserialized data is the correct type and is structurally
	// equivalent to the encoded data.
	assert.IsType(t, data, map[types.Digest]interface{}{})
	assert.Equal(t, diffMap, data)
}
