package query

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

func TestParseDTQuery(t *testing.T) {
	unittest.SmallTest(t)
	testQuery := DigestTable{
		RowQuery: &Search{
			Pos:            true,
			Neg:            false,
			Head:           true,
			Unt:            true,
			IncludeIgnores: true,
			QueryStr:       "source_type=gm&param=value",
			Limit:          20,
		},
		ColumnQuery: &Search{
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

	var ctQuery DigestTable
	assert.NoError(t, ParseDTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), 9, &ctQuery))
	exp := url.Values{"source_type": []string{"gm"}, "param": []string{"value"}}
	assert.True(t, util.In(types.PRIMARY_KEY_FIELD, ctQuery.Match))
	assert.Equal(t, exp, ctQuery.RowQuery.TraceValues)
	assert.Equal(t, exp, ctQuery.ColumnQuery.TraceValues)
	assert.Equal(t, int32(9), ctQuery.ColumnQuery.Limit)

	testQuery.RowQuery.QueryStr = ""
	jsonBytes, err = json.Marshal(&testQuery)
	assert.NoError(t, err)
	assert.Error(t, ParseDTQuery(ioutil.NopCloser(bytes.NewBuffer(jsonBytes)), 10, &ctQuery))
}

// TestParseQuery spot checks the parsing of a string and makes sure the object produced
// is consistent.
func TestParseQuery(t *testing.T) {
	unittest.SmallTest(t)

	q := &Search{}
	err := clearParseQuery(q, "fdiffmax=-1&fref=false&frgbamax=-1&head=true&include=false&issue=2370153003&limit=50&match=gamma_correct&match=name&metric=combined&neg=false&pos=false&query=source_type%3Dgm&sort=desc&unt=true")
	assert.NoError(t, err)

	assert.Equal(t, &Search{
		Metric:         "combined",
		Sort:           "desc",
		Match:          []string{"gamma_correct", "name"},
		BlameGroupID:   "",
		Pos:            false,
		Neg:            false,
		Head:           true,
		Unt:            true,
		IncludeIgnores: false,
		QueryStr:       "",
		TraceValues: url.Values{
			"source_type": []string{"gm"},
		},
		RQueryStr:       "",
		RTraceValues:    paramtools.ParamSet{},
		ChangeListID:    "2370153003",
		DeprecatedIssue: 2370153003,
		PatchSetsStr:    "",
		PatchSets:       []int64(nil),
		IncludeMaster:   false,
		FCommitBegin:    "",
		FCommitEnd:      "",
		FRGBAMin:        0,
		FRGBAMax:        -1,
		FDiffMax:        -1,
		FGroupTest:      "",
		FRef:            false,
		Offset:          0,
		Limit:           50,
		NoDiff:          false,
		NewCLStore:      false,
	}, q)
}

// TestParseSearchValidList checks a list of queries from live data
// processes as valid.
func TestParseSearchValidList(t *testing.T) {
	unittest.SmallTest(t)

	// Load the list of of live queries.
	contents, err := testutils.ReadFile("valid_queries.txt")
	assert.NoError(t, err)

	queries := strings.Split(contents, "\n")

	for _, qStr := range queries {
		assertQueryValidity(t, true, qStr)
	}
}

// TestParseSearchInvalidList checks a list of queries from live data
// processes as invalid.
func TestParseSearchInvalidList(t *testing.T) {
	unittest.SmallTest(t)

	// Load the list of of live queries.
	contents, err := testutils.ReadFile("invalid_queries.txt")
	assert.NoError(t, err)

	queries := strings.Split(contents, "\n")

	for _, qStr := range queries {
		assertQueryValidity(t, false, qStr)
	}
}

func assertQueryValidity(t *testing.T, isCorrect bool, qStr string) {
	assertFn := assert.NoError
	if !isCorrect {
		assertFn = assert.Error
	}
	q := &Search{}
	assertFn(t, clearParseQuery(q, qStr), qStr)
}

func clearParseQuery(q *Search, qStr string) error {
	*q = Search{}
	r, err := http.NewRequest("GET", "/?"+qStr, nil)
	if err != nil {
		return err
	}
	return ParseSearch(r, q)
}
