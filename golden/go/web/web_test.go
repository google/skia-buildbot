package web

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/summary"
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

	mim.On("CalcSummaires", nil, query, types.ExcludeIgnoredTraces, true).Return(exampleSummaryMap, nil)

	wh := WebHandlers{
		Indexer: mim,
	}

	output, err := wh.computeByBlame(query)
	assert.NoError(t, err)

	assert.NotEmpty(t, output)

	assert.Fail(t, "not implemented")
}

var (
	exampleSummaryMap = map[types.TestName]*summary.SummaryMap{}
)
