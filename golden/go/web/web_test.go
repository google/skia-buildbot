package web

import (
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestByQuerySunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	wh := WebHandlers{}

	query := url.Values{
		types.CORPUS_FIELD: []string{"dm"},
	}

	output, err := wh.computeByBlame(query)
	assert.NoError(t, err)

	assert.NotEmpty(t, output)

	assert.Fail(t, "not implemented")
}
