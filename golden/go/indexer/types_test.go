package indexer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tjstore"
)

func TestChangeListIndex_Copy(t *testing.T) {
	unittest.SmallTest(t)

	alphaPSID := tjstore.CombinedPSID{CRS: "github", CL: "alpha", PS: "whatever"}
	betaPSID := tjstore.CombinedPSID{CRS: "github", CL: "beta", PS: "whatever"}

	clIdx := &ChangeListIndex{
		UntriagedResults: map[tjstore.CombinedPSID][]tjstore.TryJobResult{
			alphaPSID: {{Digest: "1111"}, {Digest: "2222"}},
		},
		ComputedTS: time.Date(2020, time.April, 1, 2, 3, 4, 0, time.UTC),
	}

	copiedIdx := clIdx.Copy()
	assert.Equal(t, clIdx, copiedIdx)

	copiedIdx.ComputedTS = time.Date(2020, time.May, 10, 10, 10, 10, 0, time.UTC)
	assert.NotEqual(t, clIdx, copiedIdx)

	// Mutate the map of the copy.
	copiedIdx.UntriagedResults[alphaPSID] = []tjstore.TryJobResult{{Digest: "3333"}}
	copiedIdx.UntriagedResults[betaPSID] = []tjstore.TryJobResult{{Digest: "3333"}}

	// Make sure the original map didn't get changed.
	assert.Len(t, clIdx.UntriagedResults, 1) // still should just have one psID entry
	assert.Equal(t, []tjstore.TryJobResult{{Digest: "1111"}, {Digest: "2222"}},
		clIdx.UntriagedResults[alphaPSID])
}
