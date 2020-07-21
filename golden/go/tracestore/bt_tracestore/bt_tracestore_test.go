package bt_tracestore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	"go.skia.org/infra/golden/go/testutils/data_bug_revert"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/types"
)

// TestBTTraceStorePutGet adds a bunch of entries one at a time and
// then retrieves the full tile.
func TestBTTraceStorePutGet(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// With no data, we should get an empty tile
	actualTile, _, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)
	require.NotNil(t, actualTile)
	require.Empty(t, actualTile.Traces)

	putTestTile(t, traceStore, commits, false /*=options*/)

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, data.MakeTestTile(), actualTile)
	require.Equal(t, commits, actualCommits)
}

func assertTilesEqual(t *testing.T, a *tiling.Tile, b *tiling.Tile) {
	assert.Equal(t, a.ParamSet, b.ParamSet)
	assert.Equal(t, a.Commits, b.Commits)
	// We can't do a naive comparison of the traces because unexported values may not exactly match
	// and don't care if they do (i.e. the cached values for TestName, Corpus)
	assert.Equal(t, len(a.Traces), len(b.Traces))
	for id, traceA := range a.Traces {
		assert.Contains(t, b.Traces, id)
		traceB := b.Traces[id]
		assert.Equal(t, traceA.Keys(), traceB.Keys())
		assert.Equal(t, traceA.Digests, traceB.Digests)
	}
}

func putTestTile(t *testing.T, traceStore tracestore.TraceStore, commits []tiling.Commit, options bool) {
	// This time is an arbitrary point in time
	now := time.Date(2019, time.May, 5, 1, 3, 4, 0, time.UTC)

	// Build a tile up from the individual data points, one at a time
	traces := data.MakeTestTile().Traces
	for _, trace := range traces {

		// Put them in backwards, just to test that order doesn't matter
		for i := len(trace.Digests) - 1; i >= 0; i-- {
			if trace.Digests[i] == tiling.MissingDigest {
				continue
			}
			e := tracestore.Entry{
				Digest: trace.Digests[i],
				Params: trace.Keys(),
			}
			if options {
				if i == 0 {
					e.Options = makeOptionsOne()
				} else {
					e.Options = makeOptionsTwo()
				}
			}
			err := traceStore.Put(context.Background(), commits[i].Hash, []*tracestore.Entry{&e}, now)
			require.NoError(t, err)
			// roll forward the clock by an arbitrary amount of time
			now = now.Add(7 * time.Second)
		}
	}
}

// Tests that a digest put temporally later will override what is there.
func TestBTTraceStorePutGetOverride(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)
	putTestTile(t, traceStore, commits, false /*=options*/)

	alphaParams := data.MakeTestTile().Traces[data.AnglerAlphaTraceID].Keys()
	require.NotEmpty(t, alphaParams)

	veryOldDigest := types.Digest("00069e4bb9c71ba0f7e2c7e03bf96699")
	veryOldTime := time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)
	veryOldEntry := []*tracestore.Entry{
		{
			Digest: veryOldDigest,
			Params: alphaParams,
		},
	}
	// This should not show up
	err = traceStore.Put(context.Background(), data.FirstCommitHash, veryOldEntry, veryOldTime)
	require.NoError(t, err)

	veryNewDigest := types.Digest("fffeb0c1980670adc5fe0bc52e7402b7")
	veryNewTime := time.Now()
	veryNewEntry := []*tracestore.Entry{
		{
			Digest: veryNewDigest,
			Params: alphaParams,
		},
	}
	// This should show up
	err = traceStore.Put(context.Background(), data.ThirdCommitHash, veryNewEntry, veryNewTime)
	require.NoError(t, err)

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	expectedTile := data.MakeTestTile()
	// This is the edit we applied
	gt := expectedTile.Traces[data.AnglerAlphaTraceID]
	gt.Digests[2] = veryNewDigest

	assertTilesEqual(t, expectedTile, actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStorePutGetOptions adds a bunch of entries (with options) one at a time and
// then retrieves the full tile.
func TestBTTraceStorePutGetOptions(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_options",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	putTestTile(t, traceStore, commits, true /*=options*/)

	// Get the tile back and make make sure the options are there.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, makeTestTileWithOptions(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStorePutGetSpanTile is like TestBTTraceStorePutGet except the 3 commits
// are lined up to go across two tiles.
func TestBTTraceStorePutGetSpanTile(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, DefaultTileSize-2)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_span",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// With no data, we should get an empty tile
	actualTile, _, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)
	require.NotNil(t, actualTile)
	require.Empty(t, actualTile.Traces)

	putTestTile(t, traceStore, commits, false /*=options*/)

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, data.MakeTestTile(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStorePutGetSpanOptionsTile is like TestBTTraceStorePutGetOptions except the 3 commits
// are lined up to go across two tiles.
func TestBTTraceStorePutGetOptionsSpanTile(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, DefaultTileSize-2)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_span",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	putTestTile(t, traceStore, commits, true /*=options*/)

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, makeTestTileWithOptions(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStorePutGetGrouped adds a bunch of entries batched by device and
// then retrieves the full Tile.
func TestBTTraceStorePutGetGrouped(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_grouped",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// With no data, we should get an empty tile
	actualTile, _, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)
	require.NotNil(t, actualTile)
	require.Empty(t, actualTile.Traces)

	// Build a tile up from the individual data points, one at a time
	now := time.Date(2019, time.May, 5, 1, 3, 4, 0, time.UTC)

	// Group the traces by device, so we should have 3 groups of 2 traces.
	traces := data.MakeTestTile().Traces
	byDevice := map[string][]*tiling.Trace{
		data.AnglerDevice:     nil,
		data.BullheadDevice:   nil,
		data.CrosshatchDevice: nil,
	}
	for _, trace := range traces {
		require.Len(t, trace.Digests, len(commits), "test data should have one digest per commit")
		dev := trace.Keys()["device"]
		byDevice[dev] = append(byDevice[dev], trace)
	}
	require.Len(t, byDevice, 3, "test data should have exactly 3 devices")

	// for each trace, report a group of two digests for each commit.
	for dev, gTraces := range byDevice {
		require.Len(t, gTraces, 2, "test data for %s should have exactly 2 traces", dev)

		for i := 0; i < len(commits); i++ {
			var entries []*tracestore.Entry
			for _, gTrace := range gTraces {
				entries = append(entries, &tracestore.Entry{
					Digest: gTrace.Digests[i],
					Params: gTrace.Keys(),
				})
			}

			err = traceStore.Put(ctx, commits[i].Hash, entries, now)
			require.NoError(t, err)
			// roll forward the clock by an arbitrary amount of time
			now = now.Add(3 * time.Minute)
		}
	}

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, data.MakeTestTile(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStorePutGetThreaded is like TestBTTraceStorePutGet, just
// with a bunch of reads/writes done in simultaneous go routines in
// an effort to catch any race conditions.
func TestBTTraceStorePutGetThreaded(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_threaded",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)

	now := time.Date(2019, time.May, 5, 1, 3, 4, 0, time.UTC)

	readTile := func() {
		defer wg.Done()
		_, _, err := traceStore.GetTile(ctx, len(commits))
		require.NoError(t, err)
	}
	go readTile()

	// Build a tile up from the individual data points, one at a time
	traces := data.MakeTestTile().Traces
	for _, tr := range traces {
		// Put them in backwards, just to test that order doesn't matter
		for i := len(tr.Digests) - 1; i >= 0; i-- {
			wg.Add(1)
			go func(now time.Time, i int, trace *tiling.Trace) {
				defer wg.Done()
				e := tracestore.Entry{
					Digest: trace.Digests[i],
					Params: trace.Keys(),
				}
				err := traceStore.Put(ctx, commits[i].Hash, []*tracestore.Entry{&e}, now)
				require.NoError(t, err)
			}(now, i, tr)
			now = now.Add(7 * time.Second)
		}
	}
	go readTile()

	wg.Wait()

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, data.MakeTestTile(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestBTTraceStoreGetDenseTile makes sure we get an empty tile
func TestBTTraceStoreGetDenseTileEmpty(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	realCommitIndices := []int{300, 501, 557}
	totalCommits := 1101
	mvcs, _ := mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_dense_empty",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// With no data, we should get an empty tile
	actualTile, actualCommits, err := traceStore.GetDenseTile(ctx, len(commits))
	require.NoError(t, err)
	require.NotNil(t, actualTile)
	require.Empty(t, actualCommits)
	require.Empty(t, actualTile.Traces)

}

// TestBTTraceStoreGetDenseTile puts in a few data points sparsely spaced throughout
// time and makes sure we can call GetDenseTile to get them condensed together
// (i.e. with all the empty commits tossed out). It puts them in a variety of conditions
// to try to identify any edge cases.
func TestBTTraceStoreGetDenseTile(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	// 3 commits, arbitrarily spaced out across the last tile
	commits := data.MakeTestCommits()
	realCommitIndices := []int{795, 987, 1001}
	totalCommits := (256 * 4) - 1
	mvcs, lCommits := mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile := data.MakeTestTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)

	// 3 commits, arbitrarily spaced out across 3 tiles, with no data
	// in the most recent tile
	commits = data.MakeTestCommits()
	realCommitIndices = []int{300, 501, 557}
	totalCommits = 1101
	mvcs, lCommits = mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile = data.MakeTestTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)

	// As above, just 2 commits
	commits = data.MakeTestCommits()[1:]
	realCommitIndices = []int{501, 557}
	totalCommits = 1101
	mvcs, lCommits = mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile = makeTrimmedTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)

	// All commits are on the first commit of their tile
	commits = data.MakeTestCommits()
	realCommitIndices = []int{0, 256, 512}
	totalCommits = 1101
	mvcs, lCommits = mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile = data.MakeTestTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)

	// All commits are on the last commit of their tile
	commits = data.MakeTestCommits()
	realCommitIndices = []int{255, 511, 767}
	totalCommits = 1101
	mvcs, lCommits = mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile = data.MakeTestTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)

	// Empty tiles between commits
	commits = data.MakeTestCommits()
	realCommitIndices = []int{50, 800, 1100}
	totalCommits = 1101
	mvcs, lCommits = mockSparseVCSWithCommits(commits, realCommitIndices, totalCommits)
	expectedTile = data.MakeTestTile()
	testDenseTile(t, expectedTile, mvcs, commits, lCommits, realCommitIndices)
}

func makeTrimmedTile() *tiling.Tile {
	return &tiling.Tile{
		Commits: data.MakeTestCommits(),
		Traces: map[tiling.TraceID]*tiling.Trace{
			data.AnglerAlphaTraceID: tiling.NewTrace(
				types.DigestSlice{data.AlphaNegativeDigest, data.AlphaPositiveDigest},
				map[string]string{
					"device":              data.AnglerDevice,
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),
			data.AnglerBetaTraceID: tiling.NewTrace(
				types.DigestSlice{data.BetaPositiveDigest, data.BetaPositiveDigest},
				map[string]string{
					"device":              data.AnglerDevice,
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),

			data.BullheadAlphaTraceID: tiling.NewTrace(
				types.DigestSlice{data.AlphaNegativeDigest, data.AlphaUntriagedDigest},
				map[string]string{
					"device":              data.BullheadDevice,
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),
			data.BullheadBetaTraceID: tiling.NewTrace(
				types.DigestSlice{data.BetaPositiveDigest, data.BetaPositiveDigest},
				map[string]string{
					"device":              data.BullheadDevice,
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),

			data.CrosshatchAlphaTraceID: tiling.NewTrace(
				types.DigestSlice{data.AlphaNegativeDigest, data.AlphaPositiveDigest},
				map[string]string{
					"device":              data.CrosshatchDevice,
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),
			data.CrosshatchBetaTraceID: tiling.NewTrace(
				types.DigestSlice{tiling.MissingDigest, tiling.MissingDigest},
				map[string]string{
					"device":              data.CrosshatchDevice,
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     data.GMCorpus,
				},
			),
		},

		// Summarizes all the keys and values seen in this tile
		// The values should be in alphabetical order (see paramset.Normalize())
		ParamSet: map[string][]string{
			"device":              {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
			types.PrimaryKeyField: {string(data.AlphaTest), string(data.BetaTest)},
			types.CorpusField:     {data.GMCorpus},
		},
	}
}

// testDenseTile takes the data from tile, Puts it into BT, then pulls the tile given
// the commit layout in VCS and returns it.
func testDenseTile(t *testing.T, tile *tiling.Tile, mvcs *mock_vcs.VCS, commits []tiling.Commit, lCommits []*vcsinfo.LongCommit, realCommitIndices []int) {
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_test_dense",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// This time is an arbitrary point in time
	now := time.Date(2019, time.May, 5, 1, 3, 4, 0, time.UTC)

	// Build a tile up from the individual data points, one at a time
	traces := tile.Traces
	for _, trace := range traces {
		// Put them in backwards, just to test that order doesn't matter
		for i := len(trace.Digests) - 1; i >= 0; i-- {
			e := tracestore.Entry{
				Digest: trace.Digests[i],
				Params: trace.Keys(),
			}
			err := traceStore.Put(ctx, commits[i].Hash, []*tracestore.Entry{&e}, now)
			require.NoError(t, err)
			// roll forward the clock by an arbitrary amount of time
			now = now.Add(7 * time.Second)
		}
	}

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, allCommits, err := traceStore.GetDenseTile(ctx, len(commits))
	require.NoError(t, err)
	require.Len(t, allCommits, len(lCommits)-realCommitIndices[0])

	// In mockSparseVCSWithCommits, we change the time of the commits, so we need
	// to update the expected times to match.
	for i := range commits {
		commits[i].CommitTime = lCommits[realCommitIndices[i]].Timestamp
	}
	tile.Commits = commits

	assertTilesEqual(t, tile, actualTile)
}

// TestBTTraceStoreOverwrite makes sure that options and digests can be overwritten by
// later Put calls.
func TestBTTraceStoreOverwrite(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	commits := data.MakeTestCommits()
	mvcs := mockVCSWithCommits(commits, 0)
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "three_devices_overwrite",
		VCS:        mvcs,
	}

	// This digest should be not seen in the final tile.
	badDigest := types.Digest("badc918f358a30d920f0b4e571ef20bd")

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// an arbitrary time that takes place before putTestTile's time.
	now := time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC)

	// Write some data to trace AnglerAlphaTraceID that should be overwritten
	for i := 0; i < len(commits); i++ {
		e := tracestore.Entry{
			Digest: badDigest,
			Params: map[string]string{
				"device":              data.AnglerDevice,
				types.PrimaryKeyField: string(data.AlphaTest),
				types.CorpusField:     "gm",
			},
			Options: map[string]string{
				"should": "be overwritten",
			},
		}
		err := traceStore.Put(context.Background(), commits[i].Hash, []*tracestore.Entry{&e}, now)
		require.NoError(t, err)
	}

	// Now overwrite it.
	putTestTile(t, traceStore, commits, true /*=options*/)

	// Get the tile back and make sure it exactly matches the tile
	// we hand-crafted for the test data.
	actualTile, actualCommits, err := traceStore.GetTile(ctx, len(commits))
	require.NoError(t, err)

	assertTilesEqual(t, makeTestTileWithOptions(), actualTile)
	require.Equal(t, commits, actualCommits)
}

// TestGetTileKey tests the internal workings of deriving a
// TileKey from the commit index. See BIGTABLE.md for more.
func TestGetTileKey(t *testing.T) {
	unittest.SmallTest(t)

	type testStruct struct {
		InputRepoIndex int

		ExpectedKey   TileKey
		ExpectedIndex int
	}
	// test data is valid, but arbitrary.
	tests := []testStruct{
		{
			InputRepoIndex: 0,
			ExpectedKey:    TileKey(2147483647),
			ExpectedIndex:  0,
		},
		{
			InputRepoIndex: 10,
			ExpectedKey:    TileKey(2147483647),
			ExpectedIndex:  10,
		},
		{
			InputRepoIndex: 300,
			ExpectedKey:    TileKey(2147483646),
			ExpectedIndex:  44,
		},
		{
			InputRepoIndex: 123456,
			ExpectedKey:    TileKey(2147483165),
			ExpectedIndex:  64,
		},
	}

	for _, test := range tests {
		key, index := GetTileKey(test.InputRepoIndex)
		require.Equal(t, test.ExpectedKey, key)
		require.Equal(t, test.ExpectedIndex, index)
	}
}

// TestCalcShardedRowName tests the internal workings of sharding
// a given subkey.
func TestCalcShardedRowName(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	btConf := BTConfig{
		// Leaving other things blank because we won't actually hit BT
		// or use the VCS.
	}

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	type testStruct struct {
		InputKey     TileKey
		InputRowType string
		InputSubKey  string

		ExpectedRowName string
	}
	// test data is valid, but arbitrary.
	tests := []testStruct{
		{
			InputKey:     TileKey(2147483647),
			InputRowType: typeTrace,
			InputSubKey:  ",0=1,1=3,3=0,",

			ExpectedRowName: "09:ts:t:2147483647:,0=1,1=3,3=0,",
		},
		{
			InputKey:     TileKey(2147483647),
			InputRowType: typeTrace,
			InputSubKey:  ",0=1,1=3,9=0,",

			ExpectedRowName: "13:ts:t:2147483647:,0=1,1=3,9=0,",
		},
	}

	for _, test := range tests {
		row := traceStore.calcShardedRowName(test.InputKey, test.InputRowType, test.InputSubKey)
		require.Equal(t, test.ExpectedRowName, row)
	}
}

// TestPutUpdate tests that if vcs.IndexOf fails, we call Update
// and then insert the data.
func TestPutUpdate(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	mvcs := &mock_vcs.VCS{}
	defer mvcs.AssertExpectations(t)

	btConf := BTConfig{
		ProjectID:  "should-use-the-emulator",
		InstanceID: "testinstance",
		TableID:    "update",
		VCS:        mvcs,
	}

	require.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	require.NoError(t, InitBT(context.Background(), btConf))

	notFound := errors.New("commit not found")
	mvcs.On("IndexOf", testutils.AnyContext, data.FirstCommitHash).Return(-1, notFound).Once()
	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("IndexOf", testutils.AnyContext, data.FirstCommitHash).Return(4001, nil).Once()
	mvcs.On("Details", testutils.AnyContext, data.FirstCommitHash, false).Return(&vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    data.FirstCommitHash,
			Author:  "example@example.com",
			Subject: "test commit",
		},
		Timestamp: time.Now(),
	}, nil).Once()

	ctx := context.Background()
	traceStore, err := New(ctx, btConf, true)
	require.NoError(t, err)

	// put some arbitrary data (we only care that the write
	// did not return an error)
	err = traceStore.Put(ctx, data.FirstCommitHash, []*tracestore.Entry{
		{
			Digest: data.AlphaPositiveDigest,
			Options: map[string]string{
				"ext":        "png",
				"resolution": "1000px",
			},
			Params: map[string]string{
				"name": string(data.AlphaTest),
			},
		},
	}, time.Now())
	require.NoError(t, err)
}

// TestCommitsFromVCSSimultaneousCommits tests that we properly turn commit indices into real
// commits, even when we have simultaneous commits.
func TestCommitsFromVCSSimultaneousCommits(t *testing.T) {
	unittest.SmallTest(t)
	mvcs := &mock_vcs.VCS{}
	defer mvcs.AssertExpectations(t)

	// For this test, we assume we have 10 commits that we don't care about, a commit that isn't part
	// of the tile, then 4 commits that are. The commits have been constructed such that the first two
	// (which we are imagining to be commit index 10 and 11) share the same timestamp.
	tCommits, lCommits, hashes := makeSimultaneousCommits()

	mvcs.On("ByIndex", testutils.AnyContext, 11).Return(lCommits[1], nil)

	tMatcher := mock.MatchedBy(func(ts time.Time) bool {
		return ts.Before(lCommits[1].Timestamp)
	})
	mvcs.On("From", tMatcher).Return(hashes, nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, hashes[1:], false).Return(lCommits[1:], nil)

	b := &BTTraceStore{
		vcs: mvcs,
	}
	commitsWithData := []int{11, 12, 14}

	allCommits, denseCommits, err := b.commitsFromVCS(context.Background(), commitsWithData)
	require.NoError(t, err)
	assert.Equal(t, []tiling.Commit{tCommits[1], tCommits[2], tCommits[4]}, denseCommits)
	assert.Equal(t, tCommits[1:], allCommits)
}

// makeSimultaneousCommits returns the data for 5 commits, of which the first two share a timestamp.
func makeSimultaneousCommits() ([]tiling.Commit, []*vcsinfo.LongCommit, []string) {
	commits := data_bug_revert.MakeTestCommits()
	commits[1].CommitTime = commits[0].CommitTime

	longCommits := make([]*vcsinfo.LongCommit, 0, len(commits))
	hashes := make([]string, 0, len(commits))
	for _, c := range commits {
		longCommits = append(longCommits, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    c.Hash,
				Author:  c.Author,
				Subject: c.Subject,
			},
			Timestamp: c.CommitTime,
		})
		hashes = append(hashes, c.Hash)
	}
	return commits, longCommits, hashes
}

func mockVCSWithCommits(commits []tiling.Commit, offset int) *mock_vcs.VCS {
	mvcs := &mock_vcs.VCS{}

	indexCommits := make([]*vcsinfo.IndexCommit, 0, len(commits))
	hashes := make([]string, 0, len(commits))
	longCommits := make([]*vcsinfo.LongCommit, 0, len(commits))
	for i, c := range commits {
		mvcs.On("IndexOf", testutils.AnyContext, c.Hash).Return(i+offset, nil).Maybe()

		indexCommits = append(indexCommits, &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     i + offset,
			Timestamp: c.CommitTime,
		})
		hashes = append(hashes, c.Hash)
		longCommits = append(longCommits, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    c.Hash,
				Author:  c.Author,
				Subject: c.Subject,
			},
			Timestamp: c.CommitTime,
		})

		mvcs.On("Details", testutils.AnyContext, c.Hash, false).Return(longCommits[i], nil).Maybe()
	}

	mvcs.On("LastNIndex", len(commits)).Return(indexCommits)
	mvcs.On("DetailsMulti", testutils.AnyContext, hashes, false).Return(longCommits, nil)

	return mvcs
}

func mockSparseVCSWithCommits(commits []tiling.Commit, realCommitIndices []int, totalCommits int) (*mock_vcs.VCS, []*vcsinfo.LongCommit) {
	mvcs := &mock_vcs.VCS{}
	if len(commits) != len(realCommitIndices) {
		panic("commits should be same length as realCommitIndices")
	}

	// Create many synthetic commits.
	indexCommits := make([]*vcsinfo.IndexCommit, totalCommits)
	longCommits := make([]*vcsinfo.LongCommit, totalCommits)
	hashes := []string{}
	for i := 0; i < totalCommits; i++ {
		h := fmt.Sprintf("%040d", i)
		indexCommits[i] = &vcsinfo.IndexCommit{
			Hash:  h,
			Index: i,
			// space the commits 1700 seconds apart, starting at the epoch
			// This is an arbitrary amount of space.
			Timestamp: time.Unix(int64(i*1700), 0),
		}

		longCommits[i] = &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:   h,
				Author: "nobody@example.com",
			},
			Timestamp: time.Unix(int64(i*1700), 0),
		}
		hashes = append(hashes, h)

	}

	for i, c := range commits {
		index := realCommitIndices[i]
		mvcs.On("IndexOf", testutils.AnyContext, c.Hash).Return(index, nil).Maybe()
		indexCommits[index] = &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     index,
			Timestamp: time.Unix(int64(index*1700), 0),
		}
		hashes[index] = c.Hash
		longCommits[index] = &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    c.Hash,
				Author:  c.Author,
				Subject: c.Subject,
			},
			Timestamp: time.Unix(int64(index*1700), 0),
		}

		mvcs.On("Details", testutils.AnyContext, c.Hash, false).Return(longCommits[index], nil).Maybe()
	}

	firstRealCommitIdx := realCommitIndices[0]
	mvcs.On("ByIndex", testutils.AnyContext, firstRealCommitIdx).Return(longCommits[firstRealCommitIdx], nil).Maybe()
	mvcs.On("From", mock.Anything).Return(hashes[firstRealCommitIdx:], nil).Maybe()
	mvcs.On("LastNIndex", 1).Return(indexCommits[totalCommits-1:]).Maybe()
	mvcs.On("DetailsMulti", testutils.AnyContext, hashes[firstRealCommitIdx:], false).Return(longCommits[firstRealCommitIdx:], nil).Maybe()

	return mvcs, longCommits
}

func makeTestTileWithOptions() *tiling.Tile {
	tile := data.MakeTestTile()
	for id, trace := range tile.Traces {
		// CrosshatchBetaTraceID has a digest at index 0 and is missing in all
		// other indices (and this is the only trace for which this occurs).
		// optionsOne are written to index 0 and optionsTwo are for all other
		// indices. Thus, CrosshatchBetaTraceID will be the only trace with
		// optionsOne applied.
		if id == data.CrosshatchBetaTraceID {
			for k, v := range makeOptionsOne() {
				trace.Keys()[k] = v
			}
		} else {
			for k, v := range makeOptionsTwo() {
				trace.Keys()[k] = v
			}
		}
		tile.Traces[id] = trace
	}
	tile.ParamSet["resolution"] = []string{"1080p", "4k"}
	tile.ParamSet["color"] = []string{"orange"}
	return tile
}

func makeOptionsOne() map[string]string {
	return map[string]string{
		"resolution": "1080p",
		"color":      "orange",
	}
}

func makeOptionsTwo() map[string]string {
	return map[string]string{
		"resolution": "4k",
	}
}
