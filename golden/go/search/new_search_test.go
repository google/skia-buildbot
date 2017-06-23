package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

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

func TestGetDrawableTraces(t *testing.T) {
	localTilePath := TEST_DATA_DIR_SEARCH_API + "/" + "lumafilter_skia.sample"

	// Load the storage layer.
	storages, _, ixr := getStoragesAndIndexerFromTile(t, localTilePath)
	fmt.Println("Tile loaded.")
	tile := ixr.GetIndex().GetTile(true)

	api, err := NewSearchAPI(storages, ixr)
	assert.NoError(t, err)

	qStr := "fdiffmax=-1&fref=false&frgbamax=255&frgbamin=0&head=true&include=false&limit=2000000&match=gamma_correct&match=name&metric=combined&neg=false&offset=0&pos=false&query=source_type%3Dgm&sort=desc&unt=true"
	q := &Query{}
	assert.NoError(t, clearParseQuery(q, qStr))

	resp, err := api.Search(q)
	assert.NoError(t, err)

	// Check the consistency of the traces.
	for _, digest := range resp.Digests {
		for _, oneTrace := range digest.Traces.Traces {
			dataIdx := 0
			vals := tile.Traces[oneTrace.ID].(*types.GoldenTrace).Values
			for idx, val := range vals {
				if val != types.MISSING_DIGEST {
					d := oneTrace.Data[dataIdx]
					dataIdx++
					assert.Equal(t, idx, d.X)
					if d.S != MAX_REF_DIGESTS-1 {
						assert.True(t, val == (digest.Traces.Digests[d.S].Digest))
					}
				}
			}
		}
	}

	// Compare to the old Search results.
	q.Limit = 2000000
	oldResp, err := Search(q, storages, ixr.GetIndex())
	assert.NoError(t, err)

	fmt.Printf("DigestCount: %d : %d\n", len(resp.Digests), len(oldResp.Digests))

	for _, newDigest := range resp.Digests {
		// Find the same digest in the result from the old search.
		found := false
		for _, oldDigest := range oldResp.Digests {
			if (oldDigest.Digest == newDigest.Digest) && (oldDigest.Test == newDigest.Test) {
				found = true
				assert.Equal(t, oldDigest.Status, newDigest.Status)

				// Not all digests in the new response have parameters and traces.
				if newDigest.ParamSet != nil {
					assert.Equal(t, len(oldDigest.ParamSet), len(newDigest.ParamSet))

					// Compare the parameters.
					for k, vals := range oldDigest.ParamSet {
						sort.Strings(vals)
						foundVals := newDigest.ParamSet[k]
						sort.Strings(foundVals)
						assert.Equal(t, vals, foundVals)
					}
					assert.Equal(t, oldDigest.Traces, newDigest.Traces)
				}
				break
			}
		}
		assert.True(t, found, fmt.Sprintf("Could not find %s in new result.", newDigest.Digest))
	}
}

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
		nonEmpty += checkQuery(b, api, storages, idx, qStr, exp, &buf)
		total++
		fmt.Printf("Queries (non-empty / total): %d / %d\n", nonEmpty, total)

		if (STOP_AFTER_N_EMPTY_QUERIES > 0) && (nonEmpty > STOP_AFTER_N_EMPTY_QUERIES) {
			break
		}
	}
}

func checkQuery(t assert.TestingT, api *SearchAPI, storages *storage.Storage, idx *indexer.SearchIndex, qStr string, exp *expstorage.Expectations, buf *bytes.Buffer) int {
	q := &Query{}

	// We ignore incorrect queries. They are tested somewhere else.
	err := clearParseQuery(q, qStr)
	if err != nil {
		return 0
	}

	// Ignore queries with blames since they are ephemeral.
	if (q.BlameGroupID != "") || (q.Issue != "") {
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

	expDigests := getTargetDigests(q, idx.GetTile(q.IncludeIgnores), exp)

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

func getTargetDigests(q *Query, tile *tiling.Tile, exp *expstorage.Expectations) util.StringSet {
	result := util.StringSet{}
	lastIdx := tile.LastCommitIndex()
	for _, trace := range tile.Traces {
		if tiling.Matches(trace, q.Query) {
			gTrace := trace.(*types.GoldenTrace)
			vals := gTrace.Values
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
