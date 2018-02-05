package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"testing"

	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/indexer"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func TestSearch(t *testing.T) {
	testutils.MediumTest(t)

	storages, idx, tile, ixr := getStoragesIndexTile(t, gcs.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(t, err)
	exp, err := storages.ExpectationsStore.Get()
	assert.NoError(t, err)
	var buf bytes.Buffer

	// test basic search
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&unt=true&pos=true&neg=true&head=true", paramQuery)
	checkQuery(t, api, idx, qStr, exp, &buf)

	// test restricting to a commit range.
	commits := tile.Commits[0 : tile.LastCommitIndex()+1]
	middle := len(commits) / 2
	beginIdx := middle - 2
	endIdx := middle + 2
	fBegin := commits[beginIdx].Hash
	fEnd := commits[endIdx].Hash

	testQueryCommitRange(t, api, idx, tile, exp, fBegin, fEnd)
	for i := 0; i < tile.LastCommitIndex(); i++ {
		testQueryCommitRange(t, api, idx, tile, exp, commits[i].Hash, commits[i].Hash)
	}
}

func testQueryCommitRange(t *testing.T, api *SearchAPI, idx *indexer.SearchIndex, tile *tiling.Tile, exp *expstorage.Expectations, startHash, endHash string) {
	var buf bytes.Buffer
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&fbegin=%s&fend=%s&unt=true&pos=true&neg=true&head=true", paramQuery, startHash, endHash)
	checkQuery(t, api, idx, qStr, exp, &buf)
}

func checkQuery(t assert.TestingT, api *SearchAPI, idx *indexer.SearchIndex, qStr string, exp *expstorage.Expectations, buf *bytes.Buffer) int {
	q := &Query{}

	// We ignore incorrect queries. They are tested somewhere else.
	err := clearParseQuery(q, qStr)
	if err != nil {
		return 0
	}

	// tile := randomize(idx.GetTile(q.IncludeIgnores))
	tile := idx.GetTile(q.IncludeIgnores)

	// TODO(stephana): Remove the lines below to also exercise the search for
	// issues. This requires to refresh the set of input queries.

	// Ignore queries for gerrit issues right now.
	if q.Issue > 0 {
		return 0
	}

	// Ignore queries with blames since they are ephemeral.
	if q.BlameGroupID != "" {
		return 0
	}

	// Addjust the old default value for MaxRGBA
	if q.FRGBAMax < 0 {
		q.FRGBAMax = 255
	}

	resp, err := api.Search(q)
	assert.NoError(t, err)

	// Serialize the response to json.
	buf.Reset()
	assert.NoError(t, json.NewEncoder(buf).Encode(resp))

	expDigests := getTargetDigests(t, q, tile, exp)

	foundDigests := util.StringSet{}
	for _, digestRec := range resp.Digests {
		foundDigests[digestRec.Digest] = true
	}

	set1 := expDigests.Keys()
	set2 := foundDigests.Keys()
	sort.Strings(set1)
	sort.Strings(set2)
	assert.Equal(t, set1, set2)
	return 1
}

func getTargetDigests(t assert.TestingT, q *Query, tile *tiling.Tile, exp *expstorage.Expectations) util.StringSet {
	// Account for a given commit range.
	startIdx := 0
	endIdx := tile.LastCommitIndex()

	if q.FCommitBegin != "" {
		startIdx, _ = tiling.FindCommit(tile.Commits, q.FCommitBegin)
		assert.True(t, startIdx >= 0)
	}

	if q.FCommitEnd != "" {
		endIdx, _ = tiling.FindCommit(tile.Commits, q.FCommitEnd)
		assert.True(t, endIdx >= 0)
	}
	assert.True(t, startIdx <= endIdx)

	digestSet := util.StringSet{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		digestSet.AddLists(gTrace.Values)
	}
	allDigests := map[string]int{}
	for idx, digest := range digestSet.Keys() {
		allDigests[digest] = idx
	}

	result := util.StringSet{}
	lastIdx := endIdx - startIdx
	for _, trace := range tile.Traces {
		if tiling.Matches(trace, q.Query) {
			gTrace := trace.(*types.GoldenTrace)
			vals := gTrace.Values[startIdx : endIdx+1]
			p := gTrace.Params_
			test := p[types.PRIMARY_KEY_FIELD]

			relevantDigests := []string(nil)
			if q.Head {
				idx := lastIdx
				for (idx >= 0) && (vals[idx] == types.MISSING_DIGEST) {
					idx--
				}
				if idx >= 0 {
					relevantDigests = []string{vals[idx]}
				}
			} else {
				relevantDigests = vals
			}

			for _, digest := range relevantDigests {
				if !q.excludeClassification(exp.Classification(test, digest)) {
					result[digest] = true
				}
			}
		}
	}
	delete(result, types.MISSING_DIGEST)
	return result
}
