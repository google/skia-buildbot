package web

import (
	"net/url"
	"testing"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/indexer/mocks"
	bug_revert "go.skia.org/infra/golden/go/testutils/data_bug_revert"
	"go.skia.org/infra/golden/go/types"
)

// A unit test of the /byquery endpoint
func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	query := url.Values{
		types.CORPUS_FIELD: []string{"dm"},
	}

	mim := &mocks.IndexMaker{}
	mis := &mocks.IndexSearcher{}

	// TODO(kjlubick): defer assert expectations

	mim.On("GetIndex").Return(mis)

	mis.On("CalcSummaries", types.TestNameSet(nil), query, types.ExcludeIgnoredTraces, true).
		Return(bug_revert.MakeTestSummaryMapHead(), nil)
	cpxTile := types.NewComplexTile(bug_revert.MakeTestTile())
	mis.On("Tile").Return(cpxTile)
	mis.On("GetBlame", bug_revert.TestTwo, bug_revert.UntriagedDigestFoxtrot, bug_revert.MakeTestCommits()).
		Return(&blame.BlameDistribution{
			Freq: []int{2}, // TODO(kjlubick): I think the data should be determined as
			// "commit index 1 is mostly at fault, with a hint of commit 2, maybe"
			Old: false,
		}, nil)

	wh := WebHandlers{
		Indexer: mim,
	}

	output, err := wh.computeByBlame(query)
	assert.NoError(t, err)

	assert.NotEmpty(t, output)
	spew.Dump(output)

	assert.Fail(t, "not implemented")
}
