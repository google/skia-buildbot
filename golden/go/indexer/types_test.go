package indexer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tjstore"
)

func TestChangelistIndex_Copy(t *testing.T) {
	unittest.SmallTest(t)

	alphaPSID := tjstore.CombinedPSID{CRS: "github", CL: "alpha", PS: "whatever"}
	betaPSID := tjstore.CombinedPSID{CRS: "github", CL: "beta", PS: "whatever"}

	clIdx := &ChangelistIndex{
		LatestPatchset: alphaPSID,
		UntriagedResults: []tjstore.TryJobResult{
			{Digest: "1111"}, {Digest: "2222"},
		},
		ComputedTS: time.Date(2020, time.April, 1, 2, 3, 4, 0, time.UTC),
		ParamSet:   paramtools.ParamSet{"foo": []string{"bar", "car"}},
	}

	copiedIdx := clIdx.Copy()
	assert.Equal(t, clIdx, copiedIdx)

	copiedIdx.ComputedTS = time.Date(2020, time.May, 10, 10, 10, 10, 0, time.UTC)
	assert.NotEqual(t, clIdx, copiedIdx)
	copiedIdx.LatestPatchset = betaPSID
	// Mutate the maps of the copy.
	copiedIdx.UntriagedResults = []tjstore.TryJobResult{{Digest: "3333"}}
	copiedIdx.ParamSet["alpha"] = []string{"beta"}

	// Make sure the original maps didn't get changed.
	assert.Equal(t, []tjstore.TryJobResult{{Digest: "1111"}, {Digest: "2222"}}, clIdx.UntriagedResults)
	assert.Equal(t, alphaPSID, clIdx.LatestPatchset)
}
