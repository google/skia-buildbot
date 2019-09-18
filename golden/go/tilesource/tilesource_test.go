package tilesource

import (
	"context"
	"testing"
	"time"

	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/go/vcsinfo"

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

func TestUpdateTileSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mis := &mock_ignorestore.IgnoreStore{}
	mts := &mocks.TraceStore{}
	mu := &mock_updater.Updater{}
	mvcs := &mock_vcs.VCS{}

	nCommits := 3

	longCommits := []*vcsinfo.LongCommit{
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

	mvcs.On("Update", testutils.AnyContext, true, false).Return(nil)
	mvcs.On("DetailsMulti", testutils.AnyContext, []string{
		ZerothCommitHash, data.FirstCommitHash, data.SecondCommitHash, data.ThirdCommitHash, FourthCommitHash,
	}, false).Return(longCommits, nil)

	denseCommits := data.MakeTestCommits()
	allCommits := []*tiling.Commit{
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
	tile := data.MakeTestTile()
	mts.On("GetDenseTile", testutils.AnyContext, nCommits).Return(tile, allCommits, nil)

	// No ignores in this test
	mis.On("List").Return(nil, nil)

	mu.On("UpdateChangeListsAsLanded", testutils.AnyContext, longCommits).Return(nil)

	ts := New(CachedTileSourceConfig{
		NCommits:    3,
		IgnoreStore: mis,
		TraceStore:  mts,
		CLUpdater:   mu,
		VCS:         mvcs,
	})
	assert.Nil(t, ts.lastCpxTile)
	assert.Zero(t, ts.lastTimeStamp)

	err := ts.updateTile(context.Background())
	assert.NoError(t, err)

	cpxTile, err := ts.GetTile()
	assert.NoError(t, err)
	assert.NotNil(t, cpxTile)

	assert.Equal(t, allCommits, cpxTile.AllCommits())
	assert.Equal(t, denseCommits, cpxTile.DataCommits())
	assert.Equal(t, 3, cpxTile.FilledCommits())
	assert.Equal(t, tile, cpxTile.GetTile(types.ExcludeIgnoredTraces))
}

const (
	ZerothCommitHash = "000d148c4fb5b79ee6d40ac0308cf34f0c5b1ef6"
	FourthCommitHash = "44459d416aa25919f421acdf6cd1fac336a4b7d6"
)
