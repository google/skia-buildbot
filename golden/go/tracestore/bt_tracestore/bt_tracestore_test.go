package bt_tracestore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	"go.skia.org/infra/golden/go/serialize"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"

	TILE_LENGTH = 50
)

func TestBTTraceStore(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	btConf, mvcs := setup(t)
	defer mvcs.AssertExpectations(t)

	ctx := context.TODO()

	traceStore, err := New(ctx, btConf, false)
	assert.NoError(t, err)

	tile := makeSampleTile(t, TEST_DATA_PATH)
	// For each value in tile get the traceIDs that are not empty.
	traceIDsPerCommit := make([]tiling.TraceIdSlice, TILE_LENGTH)
	for traceID, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for i := 0; i < TILE_LENGTH; i++ {
			if gTrace.Digests[i] != types.MISSING_DIGEST {
				traceIDsPerCommit[i] = append(traceIDsPerCommit[i], traceID)
			}
		}
	}
	// rand.Seed(time.Now().UnixNano())
	// for _, tids := range traceIDsPerCommit {
	// 	rand.Shuffle(len(tids), func(i, j int) { tids[i], tids[j] = tids[j], tids[i] })
	// }

	indices := make([]int, TILE_LENGTH)
	maxIndex := 0
	maxLen := len(traceIDsPerCommit[0])
	for idx := range indices {
		if len(traceIDsPerCommit[idx]) > maxLen {
			maxLen = len(traceIDsPerCommit[idx])
			maxIndex = idx
		}
		indices[idx] = idx
	}

	// Ingest the biggest tile.
	entries := []*tracestore.Entry{}
	allDigests := map[types.Digest]bool{"": true}
	for _, traceID := range traceIDsPerCommit[maxIndex] {
		t := tile.Traces[traceID].(*types.GoldenTrace)
		digest := t.Digests[maxIndex]
		allDigests[digest] = true
		entries = append(entries, &tracestore.Entry{Digest: digest, Params: t.Params()})
	}
	fmt.Printf("Max index: %d with len %d: entries: %#v %#v %#v\n", maxIndex, maxLen, *entries[0], *entries[1], *entries[2])
	assert.NoError(t, traceStore.Put(ctx, tile.Commits[maxIndex].Hash, entries, time.Now()))

	maxTileKey, _, err := traceStore.getTileKey(ctx, tile.Commits[maxIndex].Hash)
	assert.NoError(t, err)

	foundDigestMap, err := traceStore.getDigestMap(ctx, maxTileKey)
	assert.NoError(t, err)
	assert.Equal(t, len(allDigests), foundDigestMap.Len())

	for digest := range allDigests {
		id, err := foundDigestMap.ID(digest)
		assert.NoError(t, err)
		if digest == "" {
			assert.Equal(t, int32(0), id)
		} else {
			assert.NotEqual(t, int32(0), id)
		}
	}

	traceIDsPerCommit[maxIndex] = []tiling.TraceId{}
	// rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })

	// Randomly add samples from the tile to that
	for len(indices) > 0 {
		// rand.Shuffle(len(indices), func(i, j int) { indices[i], indices[j] = indices[j], indices[i] })
		idx := indices[0]
		indices = indices[1:]
		if len(traceIDsPerCommit[idx]) == 0 {
			continue
		}

		entries := []*tracestore.Entry{}
		for _, traceID := range traceIDsPerCommit[idx] {
			t := tile.Traces[traceID].(*types.GoldenTrace)
			digest := t.Digests[idx]
			allDigests[digest] = true
			entries = append(entries, &tracestore.Entry{Digest: digest, Params: t.Params()})
		}
		assert.NoError(t, traceStore.Put(ctx, tile.Commits[idx].Hash, entries, time.Now()))
	}

	// Load the tile and verify it's identical.
	foundTile, commits, cardinalities, err := traceStore.GetTile(ctx, TILE_LENGTH, false)
	assert.NoError(t, err)
	assert.NotNil(t, commits)
	assert.Equal(t, tile.Commits[len(tile.Commits)-TILE_LENGTH:], commits)
	assert.NotNil(t, cardinalities)

	// assert.Equal(t, tile, foundTile)
	assert.Equal(t, len(tile.Traces), len(foundTile.Traces))
	for traceID, trace := range tile.Traces {
		gt := trace.(*types.GoldenTrace)
		params := gt.Params()
		found := false

		foundCount := 0
		for _, foundTrace := range foundTile.Traces {
			if deepequal.DeepEqual(params, foundTrace.Params()) {
				foundCount++
			}
		}
		assert.Equal(t, 1, foundCount)

		for foundID, foundTrace := range foundTile.Traces {
			if deepequal.DeepEqual(params, foundTrace.Params()) {
				expDigests := gt.Digests[len(gt.Digests)-TILE_LENGTH:]
				found = true
				fgt := foundTrace.(*types.GoldenTrace)
				assert.Equal(t, len(expDigests), len(fgt.Digests))

				diff := []string{}
				diffStr := ""
				for idx, digest := range expDigests {
					isDiff := digest != fgt.Digests[idx]
					if isDiff {
						diff = append(diff, fmt.Sprintf("%d", idx))
						diffStr += fmt.Sprintf("    %q  !=  %q   \n", digest, fgt.Digests[idx])
					}
				}

				delete(foundTile.Traces, foundID)
				break
			}
		}
		assert.True(t, found)
		delete(tile.Traces, traceID)
	}
	assert.Equal(t, 0, len(foundTile.Traces))
	assert.Equal(t, 0, len(tile.Traces))
}

func setup(t sktest.TestingT) (BTConfig, *mock_vcs.VCS) {
	if !fileutil.FileExists(TEST_DATA_PATH) {
		err := gcs_testutils.DownloadTestDataFile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
		assert.NoError(t, err, "Unable to download testdata.")
	}

	tile := makeSampleTile(t, TEST_DATA_PATH)
	assert.Len(t, tile.Commits, TILE_LENGTH)

	mvcs := MockVCSWithCommits(tile.Commits)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "testtable",
		VCS:        mvcs,
	}

	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, InitBT(btConf))
	fmt.Println("BT emulator set up")
	return btConf, mvcs
}

func makeSampleTile(t sktest.TestingT, fileName string) *tiling.Tile {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample.Tile
}

func MockVCSWithCommits(commits []*tiling.Commit) *mock_vcs.VCS {
	mvcs := &mock_vcs.VCS{}

	indexCommits := make([]*vcsinfo.IndexCommit, 0, len(commits))
	hashes := make([]string, 0, len(commits))
	longCommits := make([]*vcsinfo.LongCommit, 0, len(commits))
	for i, c := range commits {
		mvcs.On("IndexOf", ctx, c.Hash).Return(i, nil).Maybe()

		indexCommits = append(indexCommits, &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     i,
			Timestamp: time.Unix(c.CommitTime, 0),
		})
		hashes = append(hashes, c.Hash)
		longCommits = append(longCommits, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    c.Hash,
				Author:  c.Author,
				Subject: fmt.Sprintf("Commit #%d in test", i),
			},
			Timestamp: time.Unix(c.CommitTime, 0),
		})
	}

	mvcs.On("LastNIndex", TILE_LENGTH).Return(indexCommits)
	mvcs.On("DetailsMulti", ctx, hashes, false).Return(longCommits, nil)

	return mvcs
}

var ctx = mock.AnythingOfType("*context.emptyCtx")
