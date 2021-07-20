package status

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/mocks"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/web/frontend"
)

// TODO(kjlubick) it would be nice to test this with multiple corpora

// TestStatusWatcherInitialLoad tests that the status is initially calculated correctly.
func TestStatusWatcherInitialLoad(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	mts := &mocks.TileSource{}
	defer mes.AssertExpectations(t)
	defer mts.AssertExpectations(t)

	cpxTile := tiling.NewComplexTile(data.MakeTestTile())
	cpxTile.SetSparse(data.MakeTestCommits())
	mts.On("GetTile").Return(cpxTile)

	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	swc := StatusWatcherConfig{
		ExpChangeListener: expectations.NewEventDispatcherForTesting(),
		ExpectationsStore: mes,
		TileSource:        mts,
	}

	watcher, err := New(context.Background(), swc)
	require.NoError(t, err)

	commits := data.MakeTestCommits()

	status := watcher.GetStatus()
	require.Equal(t, &frontend.GUIStatus{
		LastCommit: frontend.FromTilingCommit(commits[2]),
		CorpStatus: []frontend.GUICorpusStatus{
			{
				Name:           "gm",
				UntriagedCount: 2, // These are the values at HEAD, i.e. the most recent data
			},
		},
	}, status)
}

// TestStatusWatcherEventBus tests that the status is re-calculated correctly after
// the expectations get updated.
func TestStatusWatcherExpectationsChange(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	mts := &mocks.TileSource{}
	defer mes.AssertExpectations(t)
	defer mts.AssertExpectations(t)

	cpxTile := tiling.NewComplexTile(data.MakeTestTile())
	cpxTile.SetSparse(data.MakeTestCommits())
	mts.On("GetTile").Return(cpxTile)

	// The first time, we have the normal expectations (with things untriaged), then, we emulate
	// that a user has triaged the two untraiged images
	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil).Once()
	everythingTriaged := data.MakeTestExpectations()
	everythingTriaged.Set(data.AlphaTest, data.AlphaUntriagedDigest, expectations.Positive)
	everythingTriaged.Set(data.BetaTest, data.BetaUntriagedDigest, expectations.Negative)
	mes.On("Get", testutils.AnyContext).Return(everythingTriaged, nil)

	eb := expectations.NewEventDispatcherForTesting()
	swc := StatusWatcherConfig{
		ExpectationsStore: mes,
		TileSource:        mts,
		ExpChangeListener: eb,
	}

	watcher, err := New(context.Background(), swc)
	require.NoError(t, err)

	// status doesn't currently use the values of the delta, but this is what they
	// look like in production.
	eb.NotifyChange(expectations.ID{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaUntriagedDigest,
	})
	eb.NotifyChange(expectations.ID{
		Grouping: data.BetaTest,
		Digest:   data.BetaUntriagedDigest,
	})

	commits := data.MakeTestCommits()
	require.Eventually(t, func() bool {
		status := watcher.GetStatus()
		return deepequal.DeepEqual(&frontend.GUIStatus{
			LastCommit: frontend.FromTilingCommit(commits[2]),
			CorpStatus: []frontend.GUICorpusStatus{
				{
					Name:           "gm",
					UntriagedCount: 0,
				},
			},
		}, status)
	}, 2*time.Second, 100*time.Millisecond)

}
