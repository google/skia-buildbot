package search

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"

	assert "github.com/stretchr/testify/require"
)

func TestParseCTQuery(t *testing.T) {
	testutils.SmallTest(t)
	testQuery := CTQuery{
		RowQuery: &Query{
			Pos:            true,
			Neg:            false,
			Head:           true,
			Unt:            true,
			IncludeIgnores: true,
			QueryStr:       "source_type=gm&param=value",
			Limit:          20,
		},
		ColumnQuery: &Query{
			Pos:            true,
			Neg:            false,
			Head:           true,
			Unt:            true,
			IncludeIgnores: true,
			QueryStr:       "source_type=gm&param=value",
		},

		Match: []string{"gamma_correct"},
	}

	jsonBytes, err := json.Marshal(&testQuery)
	assert.NoError(t, err)

	var ctQuery CTQuery
	assert.NoError(t, ParseCTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), 9, &ctQuery))
	exp := url.Values{"source_type": []string{"gm"}, "param": []string{"value"}}
	assert.True(t, util.In(types.PRIMARY_KEY_FIELD, ctQuery.Match))
	assert.Equal(t, exp, ctQuery.RowQuery.Query)
	assert.Equal(t, exp, ctQuery.ColumnQuery.Query)
	assert.Equal(t, 9, ctQuery.ColumnQuery.Limit)

	testQuery.RowQuery.QueryStr = ""
	jsonBytes, err = json.Marshal(&testQuery)
	assert.NoError(t, err)
	assert.Error(t, ParseCTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), 10, &ctQuery))
}
