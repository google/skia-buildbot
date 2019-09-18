package tilesource

import (
	"context"
	"testing"
	"time"

	"go.skia.org/infra/go/paramtools"

	"go.skia.org/infra/go/vcsinfo"

	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/go/tiling"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	mock_vcs "go.skia.org/infra/go/vcsinfo/mocks"
	mock_updater "go.skia.org/infra/golden/go/code_review/mocks"
	mock_ignorestore "go.skia.org/infra/golden/go/ignore/mocks"
	"go.skia.org/infra/golden/go/mocks"
)

// TestUpdateTileSunnyDay tests building the tile for the first time (e.g. after reboot).
func TestUpdateTileSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.Updater{}
	mvcs := &mock_vcs.VCS{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mu.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)

	nCommits := 3

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		ZerothCommitHash, data.FirstCommitHash, data.SecondCommitHash, data.ThirdCommitHash, FourthCommitHash,
	}, false).Return(makeSparseLongCommits(), nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mu.On("UpdateChangeListsAsLanded", testutils.AnyContext, makeSparseLongCommits()).Return(nil)

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	assert.Nil(t, ts.lastCpxTile)

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

// TestUpdateTileHasPreviousPartial tests the case where some commits have already been
// processed previously via the Updater.
func TestUpdateTileHasPreviousPartial(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.Updater{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mu.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	nCommits := 3

	// We should only process the last part of the commits
	longCommits := makeSparseLongCommits()[2:]

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		data.SecondCommitHash, data.ThirdCommitHash, FourthCommitHash,
	}, false).Return(longCommits, nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mu.On("UpdateChangeListsAsLanded", testutils.AnyContext, longCommits).Return(nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits()[:2])

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

// TestUpdateTileHasPreviousAll tests the case where all commits have already been
// processed previously via the Updater.
func TestUpdateTileHasPreviousAll(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	nCommits := 3

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		// CLUpdater isn't called if we've already processed all commits before.
		VCS: mvcs,
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())
	assert.Equal(t, data.MakeTestTile(), cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

// TestUpdateTileWithPublicParams tests the case where we are only allowed to show select devices.
func TestUpdateTileWithPublicParams(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	nCommits := 3

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		VCS:         mvcs,
		PubliclyViewableParams: paramtools.ParamSet{
			"device": []string{data.AnglerDevice, data.BullheadDevice},
		},
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())

	trimmedTile := data.MakeTestTile()
	delete(trimmedTile.Traces, data.CrosshatchAlphaTraceID)
	delete(trimmedTile.Traces, data.CrosshatchBetaTraceID)
	trimmedTile.ParamSet["device"] = []string{data.AnglerDevice, data.BullheadDevice}
	assert.Equal(t, trimmedTile, cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

// TestUpdateTileWithIgnoreRules tests the case where some traces are ignored.
func TestUpdateTileWithIgnoreRules(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mvcs := &mock_vcs.VCS{}
	mct := &mocks.ComplexTile{}
	defer mis.AssertExpectations(t)
	defer mts.AssertExpectations(t)
	defer mvcs.AssertExpectations(t)
	defer mct.AssertExpectations(t)

	nCommits := 3

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)

	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(data.MakeTestTile(), makeSparseTilingCommits(), nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mct.On("AllCommits").Return(makeSparseTilingCommits())

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		VCS:         mvcs,
		PubliclyViewableParams: paramtools.ParamSet{
			"device": []string{data.AnglerDevice, data.BullheadDevice},
		},
	})
	// Pretend there was a tile previously.
	ts.lastCpxTile = mct

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, makeSparseTilingCommits(), cpxTile.AllCommits())
	assert.Equal(t, data.MakeTestCommits(), cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())

	trimmedTile := data.MakeTestTile()
	delete(trimmedTile.Traces, data.CrosshatchAlphaTraceID)
	delete(trimmedTile.Traces, data.CrosshatchBetaTraceID)
	trimmedTile.ParamSet["device"] = []string{data.AnglerDevice, data.BullheadDevice}
	assert.Equal(t, trimmedTile, cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

const (
	ZerothCommitHash = "000d148c4fb5b79ee6d40ac0308cf34f0c5b1ef6"
	FourthCommitHash = "44459d416aa25919f421acdf6cd1fac336a4b7d6"
)

// makeSparseLongCommits returns 5 vcsinfo.LongCommits; 3 are from three_devices_data with two
// extra commits, representing a "sparse" tile, where not every commit has data.
func makeSparseLongCommits() []*vcsinfo.LongCommit {
	return []*vcsinfo.LongCommit{
		// tilesource doesn't use any of these objects, so just fill out the hash
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: ZerothCommitHash,
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: data.FirstCommitHash,
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: data.SecondCommitHash,
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: data.ThirdCommitHash,
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: FourthCommitHash,
			},
		},
	}
}

// makeSparseLongCommits returns 5 tiling.Commit, 3 are from three_devices_data with two
// extra commits, representing a "sparse" tile, where not every commit has data.
func makeSparseTilingCommits() []*tiling.Commit {
	denseCommits := data.MakeTestCommits()
	return []*tiling.Commit{
		{
			Hash:       ZerothCommitHash,
			CommitTime: time.Date(2019, time.April, 22, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "zero@example.com",
		},
		denseCommits[0], denseCommits[1], denseCommits[2],
		{
			Hash:       FourthCommitHash,
			CommitTime: time.Date(2019, time.April, 28, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "four@example.com",
		},
	}
}
