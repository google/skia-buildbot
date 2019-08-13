package web

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
)

// A unit test of the /byquery endpoint
func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mim := &mocks.IndexMaker{}
	mis := &mocks.IndexSearcher{}

	// TODO(kjlubick): defer assert expectations

	mim.On("GetIndex").Return(mis)

	wh := WebHandlers{
		Indexer: mim,
	}

	query := url.Values{
		types.CORPUS_FIELD: []string{"dm"},
	}

	output, err := wh.computeByBlame(query)
	assert.NoError(t, err)

	assert.NotEmpty(t, output)

	assert.Fail(t, "not implemented")
}
