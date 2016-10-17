package search

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestParseCTQuery(t *testing.T) {
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
	assert.NoError(t, ParseCTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), "blurimage", 9, &ctQuery))
	exp := url.Values{"source_type": []string{"gm"}, "param": []string{"value"}, "name": []string{"blurimage"}}
	assert.Equal(t, exp, ctQuery.RowQuery.Query)
	assert.Equal(t, exp, ctQuery.ColumnQuery.Query)
	assert.Equal(t, 9, ctQuery.ColumnQuery.Limit)

	testQuery.RowQuery.QueryStr = ""
	jsonBytes, err = json.Marshal(&testQuery)
	assert.NoError(t, err)
	assert.Error(t, ParseCTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), "blurimage", 10, &ctQuery))
}

func TestParseCTQueryFromString(t *testing.T) {
	jsonStr := `{"rowQuery":{
                  "query":"source_type=gm",
                  "head":true,
                  "include":false,
                  "pos":false,
                  "neg":false,
                  "unt":true,
                  "blame":"",
                  "limit":50,
                  "issue":"",
                  "patchsets":""
                },
                "columnQuery":{
                  "query":"source_type=gm",
                  "head":true,"include":false,"pos":false,"neg":false,"unt":true,"blame":"","limit":50,"issue":"",
                  "patchsets":""},
                "match":null,
                "rowN":5,"columnN":5,"sortRows":"count","rowsDir":"desc","sortColumns":"diff","columnsDir":"asc"}`
	var ctQuery CTQuery
	assert.NoError(t, ParseCTQuery(ioutil.NopCloser(bytes.NewBuffer([]byte(jsonStr))), "verttext", 9, &ctQuery))
}
