package tilesource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metrics_utils "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	mock_updater "go.skia.org/infra/golden/go/code_review/mocks"
	"go.skia.org/infra/golden/go/ignore"
	mock_ignorestore "go.skia.org/infra/golden/go/ignore/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/publicparams"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// TestUpdateTileSunnyDay tests building the tile for the first time (e.g. after reboot).
func TestUpdateTileSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.ChangelistLandedUpdater{}
	mvcs := &mock_vcs.VCS{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mu.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		zerothCommitHash, data.FirstCommitHash, data.SecondCommitHash, data.ThirdCommitHash, fourthCommitHash,
	}, false).Return(makeSparseLongCommits(), nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	mu.On("UpdateChangelistsAsLanded", testutils.AnyContext, makeSparseLongCommits()).Return(nil)

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	require.Nil(t, ts.lastCpxTile)

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.IncludeIgnoredTraces))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileEmptyTile tests when there is no data for any commit
func TestUpdateTileEmptyTile(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.ChangelistLandedUpdater{}
	mvcs := &mock_vcs.VCS{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mu.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		zerothCommitHash, data.FirstCommitHash, data.SecondCommitHash, data.ThirdCommitHash, fourthCommitHash,
	}, false).Return(makeSparseLongCommits(), nil)

	empty := &tiling.Tile{
		Commits: nil,
		Traces:  map[tiling.TraceID]*tiling.Trace{},
	}

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(empty, makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	mu.On("UpdateChangelistsAsLanded", testutils.AnyContext, makeSparseLongCommits()).Return(nil)

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	require.Nil(t, ts.lastCpxTile)

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, "0", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileNilCLUpdater tests building the tile with a nil updater (e.g. skia-public
// instance).
func TestUpdateTileNilCLUpdater(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   nil,
		VCS:         mvcs,
	})
	require.Nil(t, ts.lastCpxTile)

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.IncludeIgnoredTraces))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileHasPreviousPartial tests the case where some commits have already been
// processed previously via the Updater.
func TestUpdateTileHasPreviousPartial(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.ChangelistLandedUpdater{}
	mvcs := &mock_vcs.VCS{}
	// TODO(kjlubick) It's probably best to make a real ComplexTile here and below instead
	//  of a mock. go/mocks#prefer-testing-state
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mu.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	// We should only process the last part of the commits
	longCommits := makeSparseLongCommits()[2:]

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		data.SecondCommitHash, data.ThirdCommitHash, fourthCommitHash,
	}, false).Return(longCommits, nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	mu.On("UpdateChangelistsAsLanded", testutils.AnyContext, longCommits).Return(nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits()[:2])

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.IncludeIgnoredTraces))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileHasPreviousAll tests the case where all commits have already been
// processed previously via the Updater.
func TestUpdateTileHasPreviousAll(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		// CLUpdater isn't called if we've already processed all commits before.
		VCS: mvcs,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.IncludeIgnoredTraces))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileWithPublicParams tests the case where we are only allowed to show select devices.
func TestUpdateTileWithPublicParams(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return(nil, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	publicMatcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		"gm": {
			"device": {"angler", "bullhead"},
		},
	})
	require.NoError(t, err)

	ts := New(CachedTileSourceConfig{
		NCommits:               nCommits,
		IgnoreStore:            mis,
		TraceStore:             mts,
		VCS:                    mvcs,
		PubliclyViewableParams: publicMatcher,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err = ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())

	trimmedTile := data.MakeTestTile()
	delete(trimmedTile.Traces, data.CrosshatchAlphaTraceID)
	delete(trimmedTile.Traces, data.CrosshatchBetaTraceID)
	trimmedTile.ParamSet["device"] = []string{data.AnglerDevice, data.BullheadDevice}
	assert.Equal(t, trimmedTile, cpxTile.GetTile(types.ExcludeIgnoredTraces))
	// The removed traces should *not* come back even if we say to show ignored traces.
	// PubliclyViewableParams is stronger than ignores.
	assert.Equal(t, trimmedTile, cpxTile.GetTile(types.IncludeIgnoredTraces))
	// Normally, there were 5 traces with data (6 total, 1 empty). We should expect two traces
	// have been removed due to the public list. As it happens, it's one of the crossHatch traces
	// that is empty, so when we delete both of them, we are left with 4 traces with data.
	assert.Equal(t, "4", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

// TestUpdateTileWithRules tests the case where some traces are ignored.
func TestUpdateTileWithRules(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.Store{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List", testutils.AnyContext).Return([]ignore.Rule{
		{
			Query: "device=crosshatch&name=test_beta", // hides one trace
		},
		{
			Query: "device=angler", // hides two traces
		},
	}, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	ts := New(CachedTileSourceConfig{
		NCommits:    nCommits,
		IgnoreStore: mis,
		TraceStore:  mts,
		VCS:         mvcs,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	require.NoError(t, err)

	cpxTile := ts.GetTile()
	require.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, nCommits, cpxTile.FilledCommits())

	trimmedTile := data.MakeTestTile()
	delete(trimmedTile.Traces, data.AnglerAlphaTraceID)
	delete(trimmedTile.Traces, data.AnglerBetaTraceID)
	delete(trimmedTile.Traces, data.CrosshatchBetaTraceID)
	assert.Equal(t, trimmedTile, cpxTile.GetTile(types.ExcludeIgnoredTraces))
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.IncludeIgnoredTraces))
	assert.Equal(t, "5", metrics_utils.GetRecordedMetric(t, filledTracesAtHeadMetric, nil))
	assert.Equal(t, "1", metrics_utils.GetRecordedMetric(t, emptyCommitsAtHeadMetric, nil))
}

const (
	// zerothCommitHash and fourthCommitHash are commits with no data, bolted on to the data in
	// three_devices_data to emulate "sparse" commits.
	zerothCommitHash = "000d148c4fb5b79ee6d40ac0308cf34f0c5b1ef6"
	fourthCommitHash = "44459d416aa25919f421acdf6cd1fac336a4b7d6"

	// nCommits is how many commits actually have data in the tile we generate.
	nCommits = 3
)

// makeSparseLongCommits returns 5 vcsinfo.LongCommits; 3 are from three_devices_data with two
// extra commits, representing a "sparse" tile, where not every commit has data.
func makeSparseLongCommits() []*vcsinfo.LongCommit {
	var rv []*vcsinfo.LongCommit
	for _, tc := range makeSparseTilingCommits() {
		rv = append(rv, &vcsinfo.LongCommit{
			// tilesource doesn't use any of these objects, so just fill out the hash
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: tc.Hash,
			},
		})
	}
	return rv
}

// makeSparseLongCommits returns 5 tiling.Commit, 3 are from three_devices_data with two
// extra commits, representing a "sparse" tile, where not every commit has data.
func makeSparseTilingCommits() []tiling.Commit {
	denseCommits := data.MakeTestCommits()
	return []tiling.Commit{
		{
			Hash:       zerothCommitHash,
			CommitTime: time.Date(2019, time.April, 22, 12, 0, 3, 0, time.UTC),
			Author:     "zero@example.com",
		},
		denseCommits[0], denseCommits[1], denseCommits[2],
		{
			Hash:       fourthCommitHash,
			CommitTime: time.Date(2019, time.April, 28, 12, 0, 3, 0, time.UTC),
			Author:     "four@example.com",
		},
	}
}
