package search

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

func checkQuery(t assert.TestingT, api *SearchAPI, idx *indexer.SearchIndex, qStr string, exp types.Expectations, buf *bytes.Buffer) int {
	q := &Query{}

	// We ignore incorrect queries. They are tested somewhere else.
	err := clearParseQuery(q, qStr)
	if err != nil {
		return 0
	}
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

	resp, err := api.Search(context.Background(), q)
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

func getTargetDigests(t assert.TestingT, q *Query, tile *tiling.Tile, exp types.Expectations) util.StringSet {
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

func getStoragesIndexTile(t *testing.T, bucket, storagePath, outputPath string, randomize bool) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile, *indexer.Indexer) {
	err := gcs.DownloadTestDataFile(t, bucket, storagePath, outputPath)
	assert.NoError(t, err, "Unable to download testdata.")
	return getStoragesAndIndexerFromTile(t, outputPath, randomize)
}

func getStoragesAndIndexerFromTile(t assert.TestingT, path string, randomize bool) (*storage.Storage, *indexer.SearchIndex, *tiling.Tile, *indexer.Indexer) {
	sample := loadSample(t, path, randomize)

	tileBuilder := mocks.NewMockTileBuilderFromTile(t, sample.Tile)
	eventBus := eventbus.New()
	expStore := expstorage.NewMemExpectationsStore(eventBus)
	err := expStore.AddChange(sample.Expectations.TestExp(), "testuser")
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
	assert.NoError(t, storages.InitBaseliner())

	ixr, err := indexer.New(storages, 10*time.Minute)
	assert.NoError(t, err)
	idx := ixr.GetIndex()
	tile := idx.GetTile(false)
	return storages, idx, tile, ixr
}

func loadSample(t assert.TestingT, fileName string, randomize bool) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	if randomize {
		sample.Tile = randomizeTile(sample.Tile, sample.Expectations)
	}

	return sample
}

func randomizeTile(tile *tiling.Tile, exp types.Expectations) *tiling.Tile {
	allDigestSet := util.StringSet{}
	testExp := exp.TestExp()
	for _, digests := range testExp {
		for d := range digests {
			allDigestSet[d] = true
		}
	}
	allDigests := allDigestSet.Keys()

	tileLen := tile.LastCommitIndex() + 1
	ret := tile.Copy()
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for i := 0; i < tileLen; i++ {
			gTrace.Values[i] = allDigests[int(rand.Uint32())%len(allDigests)]
		}
	}
	return ret
}
