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
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	"go.skia.org/infra/golden/go/serialize"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
)

// TestBTTraceStorePutGet adds a bunch of entries one at a time and
// then retrieves the full Tile.
func TestBTTraceStorePutGet(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := MockVCSWithCommits(commits)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test",
		VCS:        mvcs,
	}

	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, InitBT(btConf))

	ctx := context.TODO()
	traceStore, err := New(ctx, btConf, true)
	assert.NoError(t, err)

	now := time.Date(2019, time.May, 5, 1, 3, 4, 0, time.UTC)

	// Build a tile up from the individual data points, one at a time
	traces := data.MakeTestTile().Traces
	for _, trace := range traces {
		gTrace, ok := trace.(*types.GoldenTrace)
		assert.True(t, ok)

		// Put them in backwards, just to test that order doesn't matter
		for i := len(gTrace.Digests) - 1; i >= 0; i-- {
			e := tracestore.Entry{
				Digest: gTrace.Digests[i],
				Params: gTrace.Keys,
			}
			traceStore.Put(ctx, commits[i].Hash, []*tracestore.Entry{&e}, now)
			now = now.Add(7 * time.Second)
		}
	}

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits), false)
	assert.NoError(t, err)

	assert.Equal(t, data.MakeTestTile(), actualTile)
	assert.Equal(t, commits, actualCommits)
}

// TestBTDigestMap tests the internal workings of storing the
// DigestMap. See BIGTABLE.md for more about the schemas for
// the DigestMap family and the id counter family.
func TestBTDigestMap(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	tileKey := TileKey(123456) // arbitrary

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "digest_map_test",
		VCS:        nil,
	}

	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, InitBT(btConf))

	ctx := context.TODO()
	traceStore, err := New(ctx, btConf, true)
	assert.NoError(t, err)

	dm, err := traceStore.getDigestMap(ctx, tileKey)
	assert.NoError(t, err)
	assert.NotNil(t, dm)
	// should be empty, except for the initial mapping.
	assert.Equal(t, 1, dm.Len())
	i, err := dm.ID(types.MISSING_DIGEST)
	assert.NoError(t, err)
	assert.Equal(t, DigestID(0), i)

	digests := makeTestDigests(0, 100)

	// add 90 of the 100 digests using update
	ninety := make(types.DigestSet, 90)
	for _, d := range digests[0:90] {
		ninety[d] = true
	}
	dm, err = traceStore.updateDigestMap(ctx, tileKey, ninety)
	assert.NoError(t, err)
	assert.NotNil(t, dm)
	assert.Equal(t, 91, dm.Len())
	// We can't check to see if our known digests map to
	// a specific id because the digest map could present
	// the digests in a non-deterministic order.
	// We can spot check one of the ids though
	_, err = dm.Digest(88)
	assert.NoError(t, err)

	ids, err := traceStore.getIDs(ctx, 3)
	// The next 3 numbers should be 91, 92, 93 because they are
	// monotonically increasing
	assert.NoError(t, err)
	assert.Equal(t, []DigestID{91, 92, 93}, ids)
	func() {
		traceStore.availIDsMutex.Lock()
		defer traceStore.availIDsMutex.Unlock()
		assert.NotContains(t, traceStore.availIDs, DigestID(92))
		assert.NotContains(t, traceStore.availIDs, DigestID(93))
		assert.Contains(t, traceStore.availIDs, DigestID(94))
	}()

	// give two ids back (pretend we used id 92)
	traceStore.returnIDs([]DigestID{91, 93})

	func() {
		traceStore.availIDsMutex.Lock()
		defer traceStore.availIDsMutex.Unlock()
		assert.NotContains(t, traceStore.availIDs, DigestID(92))
		assert.Contains(t, traceStore.availIDs, DigestID(93))
		assert.Contains(t, traceStore.availIDs, DigestID(94))
	}()

	// call update with an overlap of new and old
	twenty := make(types.DigestSet, 90)
	for _, d := range digests[80:] {
		twenty[d] = true
	}
	dm, err = traceStore.updateDigestMap(ctx, tileKey, twenty)
	assert.NoError(t, err)
	assert.NotNil(t, dm)
	assert.Equal(t, 101, dm.Len())

	// Get it again and make sure it matches the last update phase.
	dm2, err := traceStore.getDigestMap(ctx, tileKey)
	assert.NoError(t, err)
	assert.NotNil(t, dm2)
	assert.Equal(t, dm, dm2)

	// Add a lot more digests to make sure the bulk requesting works
	for i := 1; i < 10; i++ {
		// 117 is an arbitrary prime number
		ds := make(types.DigestSet, 117)
		ds.AddLists(makeTestDigests(117*i, 117))

		dm, err := traceStore.updateDigestMap(ctx, tileKey, ds)
		assert.NoError(t, err)
		assert.NotNil(t, dm)
	}
}

// makeTestDigests returns n valid digests. These digests are easy
// for humans to understand, as they are just the hex values [0, 99]
// reversed and 0-padded to 32 chars long (a valid md5 hash).
func makeTestDigests(start, n int) []types.Digest {
	xd := make([]types.Digest, n)
	for i := 0; i < n; i++ {
		// Reverse them to exercise the prefixing of the digestMap.
		s := util.ReverseString(fmt.Sprintf("%032x", start+i))
		xd[i] = types.Digest(s)
	}
	return xd
}

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"

	TILE_LENGTH = 50
)

// TestBTTraceStoreLargeTile stores a large amount of data into the tracestore
// and retrieves it.
func TestBTTraceStoreLargeTile(t *testing.T) {
	unittest.LargeTest(t)
	t.Skip("Takes too long")
	unittest.RequiresBigTableEmulator(t)

	btConf, mvcs, tile := setupLargeTile(t)
	defer mvcs.AssertExpectations(t)

	ctx := context.TODO()

	traceStore, err := New(ctx, btConf, true)
	assert.NoError(t, err)

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
			assert.Equal(t, DigestID(0), id)
		} else {
			assert.NotEqual(t, DigestID(0), id)
		}
	}

	traceIDsPerCommit[maxIndex] = []tiling.TraceId{}

	// Randomly add samples from the tile to that
	for len(indices) > 0 {
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
	foundTile, commits, err := traceStore.GetTile(ctx, TILE_LENGTH, false)
	assert.NoError(t, err)
	assert.NotNil(t, commits)
	assert.Equal(t, tile.Commits[len(tile.Commits)-TILE_LENGTH:], commits)

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

				var diff []string
				diffStr := ""
				for idx, digest := range expDigests {
					isDiff := digest != fgt.Digests[idx]
					if isDiff {
						diff = append(diff, fmt.Sprintf("%d", idx))
						diffStr += fmt.Sprintf("    %q  !=  %q   \n", digest, fgt.Digests[idx])
					}
				}
				// Nothing should be different
				assert.Nil(t, diff)
				assert.Equal(t, "", diffStr)

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

func setupLargeTile(t sktest.TestingT) (BTConfig, *mock_vcs.VCS, *tiling.Tile) {
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
		TableID:    "large_tile_test",
		VCS:        mvcs,
	}

	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, InitBT(btConf))
	fmt.Println("BT emulator set up")
	return btConf, mvcs, tile
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

	mvcs.On("LastNIndex", len(commits)).Return(indexCommits)
	mvcs.On("DetailsMulti", ctx, hashes, false).Return(longCommits, nil)

	return mvcs
}

var ctx = mock.AnythingOfType("*context.emptyCtx")
