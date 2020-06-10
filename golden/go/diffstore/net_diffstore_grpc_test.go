// package diffstore_test houses tests that use the grpc_mocks, which cannot be in the same
// package as the rest of the tests due to a dependency cycle.
package diffstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	mocks "go.skia.org/infra/golden/go/diffstore/grpc_mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestNetDiffStoreGetSunnyDay tests the case where we get the diffs for two digests and there
// are no errors.
func TestNetDiffStoreGetSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	msc := &mocks.DiffServiceClient{}
	defer msc.AssertExpectations(t)

	expectedRequest := &diffstore.GetDiffsRequest{
		MainDigest:   string(digest1),
		RightDigests: []string{string(digest2), string(digest3)},
	}
	dm := map[types.Digest]*diff.DiffMetrics{
		digest2: {
			// It doesn't matter what this is, only that it's distinguishable from other data.
			MaxRGBADiffs: [4]int{1, 2, 3, 4},
		},
		digest3: {
			MaxRGBADiffs: [4]int{5, 6, 7, 8},
		},
	}
	msc.On("GetDiffs", testutils.AnyContext, expectedRequest).Return(&diffstore.GetDiffsResponse{
		Diffs: encodeToBytes(dm),
	}, nil)

	nds := diffstore.NewForTesting(msc, mockAddress)

	metrics, err := nds.Get(context.Background(), digest1, types.DigestSlice{digest2, digest3})
	require.NoError(t, err)
	assert.Len(t, metrics, 2)
	assert.Equal(t, dm, metrics)
}

const (
	mockAddress = "http://not_real:8765"

	// This data is arbitrary, but valid
	digest1 = types.Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digest2 = types.Digest("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	digest3 = types.Digest("cccccccccccccccccccccccccccccccc")
)

func encodeToBytes(dm map[types.Digest]*diff.DiffMetrics) []byte {
	c := util.NewJSONCodec(map[types.Digest]*diff.DiffMetrics{})
	b, err := c.Encode(dm)
	if err != nil {
		// This means invalid data is in our test setup, thus the test is invalid.
		panic(err.Error())
	}
	return b
}
