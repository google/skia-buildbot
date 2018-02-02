package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	// TEST_STORAGE_DIR_SEARCH_API is the path in the testdata bucket where
	// the test data files are stored.
	TEST_STORAGE_DIR_SEARCH_API = "gold-testdata"

	// TEST_DATA_DIR_SEARCH_API is the local directory where the local copy
	// of the test data are stored.
	TEST_DATA_DIR_SEARCH_API = "testdata_searchapi"

	// SAMPLED_TILE_FNAME is the filename that contains an entire snapshot of the
	// state of Gold at a point in time.
	SAMPLED_TILE_FNAME = "total_skia.sample"

	// QUERIES_FNAME_SEARCH_API contains the file name of the list of queries
	// that were extracted from the Gold application log.
	QUERIES_FNAME_SEARCH_API = "live_queries.txt"

	// STOP_AFTER_N_EMPTY_QUERIES sets the number of non-empty of queries after
	// which to stop. Change during profiling to shorten runs. -1 means to
	// run all queries.
	STOP_AFTER_N_EMPTY_QUERIES = -1
)

func BenchmarkNewSearchAPI(b *testing.B) {
	cloudTilePath := TEST_STORAGE_DIR_SEARCH_API + "/" + SAMPLED_TILE_FNAME + ".gz"
	cloudQueriesPath := TEST_STORAGE_DIR_SEARCH_API + "/" + QUERIES_FNAME_SEARCH_API + ".gz"

	localTilePath := TEST_DATA_DIR_SEARCH_API + "/" + SAMPLED_TILE_FNAME
	localQueriesPath := TEST_DATA_DIR_SEARCH_API + "/" + QUERIES_FNAME_SEARCH_API

	if !fileutil.FileExists(localTilePath) {
		assert.NoError(b, gcs.DownloadTestDataFile(b, gcs.TEST_DATA_BUCKET, cloudTilePath, localTilePath))
	}

	if !fileutil.FileExists(localQueriesPath) {
		assert.NoError(b, gcs.DownloadTestDataFile(b, gcs.TEST_DATA_BUCKET, cloudQueriesPath, localQueriesPath))
	}

	// Load the storage layer.
	storages, exp, ixr := getStoragesAndIndexerFromTile(b, localTilePath)
	fmt.Println("Tile loaded.")

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(b, err)
	idx := ixr.GetIndex()

	qStrings, err := fileutil.ReadLines(localQueriesPath)
	assert.NoError(b, err)

	var buf bytes.Buffer
	nonEmpty := 0
	total := 0
	for _, qStr := range qStrings {
		nonEmpty += checkQuery(b, api, idx, qStr, exp, &buf)
		total++
		fmt.Printf("Queries (non-empty / total): %d / %d\n", nonEmpty, total)

		if (STOP_AFTER_N_EMPTY_QUERIES > 0) && (nonEmpty > STOP_AFTER_N_EMPTY_QUERIES) {
			break
		}
	}
}

func checkQuery(t assert.TestingT, api *SearchAPI, idx *indexer.SearchIndex, qStr string, exp *expstorage.Expectations, buf *bytes.Buffer) int {
	q := &Query{}

	// We ignore incorrect queries. They are tested somewhere else.
	err := clearParseQuery(q, qStr)
	if err != nil {
		return 0
	}
	fmt.Printf("Qu: %s\n", spew.Sdump(q))

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

	minLen := util.MinInt(len(set1), len(set2))
	fmt.Printf("LENGTH: %d   %d   %d\n", minLen, len(set1), len(set2))
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
	lastIdx := tile.LastCommitIndex()
	for _, trace := range tile.Traces {
		if tiling.Matches(trace, q.Query) {
			gTrace := trace.(*types.GoldenTrace)
			vals := gTrace.Values[startIdx : endIdx+1]
			// fmt.Printf("Vals 1: %s\n", strVals(t, allDigests, gTrace.Values))
			// fmt.Printf("Vals 2: %s\n", strVals(t, allDigests, vals))
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

func randomize(tile *tiling.Tile) *tiling.Tile {
	tileLen := tile.LastCommitIndex() + 1
	ret := tile.Copy()
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for i := 0; i < tileLen; i++ {
			gTrace.Values[i] = randString(32)
		}
	}
	return ret
}

const vocab = "0123456789abcdef"

func randString(strLen int) string {
	ret := make([]byte, strLen)
	for i := 0; i < strLen; i++ {
		ret[i] = vocab[int(rand.Uint32())%len(vocab)]
	}
	return string(ret)
}

func strVals(t assert.TestingT, allDigests map[string]int, vals []string) string {
	var buf bytes.Buffer
	for _, val := range vals {
		_, err := buf.Write([]byte(fmt.Sprintf("%5d ", allDigests[val])))
		assert.NoError(t, err)
	}
	ret := buf.String()
	ret = ret[:util.MinInt(60, len(ret))]
	return ret
}

func getStoragesAndIndexerFromTile(t assert.TestingT, path string) (*storage.Storage, *expstorage.Expectations, *indexer.Indexer) {
	loadTimer := timer.New("Loading sample tile")
	sampledState := loadSample(t, path)
	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sampledState.Tile)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	loadTimer.Stop()

	err := expStore.AddChange(sampledState.Expectations.Tests, "testuser")
	assert.NoError(t, err)

	storages := &storage.Storage{
		ExpectationsStore: expStore,
		MasterTileBuilder: tileBuilder,
		DigestStore: &mocks.MockDigestStore{
			FirstSeen: time.Now().Unix(),
			OkValue:   true,
		},
		DiffStore: mocks.NewMockDiffStore(),
		EventBus:  eventBus,
	}

	ixr, err := indexer.New(storages, 240*time.Minute)
	assert.NoError(t, err)

	return storages, sampledState.Expectations, ixr
}
