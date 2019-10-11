package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/eventbus"
	mock_eventbus "go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/mocks"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// TODO(kjlubick) it would be nice to test this with multiple corpora

// TestStatusWatcherInitialLoad tests that the status is initially calculated correctly.
func TestStatusWatcherInitialLoad(t *testing.T) {
	unittest.SmallTest(t)

	meb := &mock_eventbus.EventBus{}
	mes := &mocks.ExpectationsStore{}
	mts := &mocks.TileSource{}
	defer meb.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mts.AssertExpectations(t)

	cpxTile := types.NewComplexTile(data.MakeTestTile())
	cpxTile.SetSparse(data.MakeTestCommits())
	mts.On("GetTile").Return(cpxTile)

	mes.On("Get").Return(data.MakeTestExpectations(), nil)

	meb.On("SubscribeAsync", expstorage.EV_EXPSTORAGE_CHANGED, mock.Anything)

	swc := StatusWatcherConfig{
		EventBus:          meb,
		ExpectationsStore: mes,
		TileSource:        mts,
	}

	watcher, err := New(swc)
	require.NoError(t, err)

	commits := data.MakeTestCommits()

	status := watcher.GetStatus()
	require.Equal(t, &GUIStatus{
		OK:            false,
		FirstCommit:   commits[0],
		LastCommit:    commits[2],
		TotalCommits:  3,
		FilledCommits: 3,
		CorpStatus: []*GUICorpusStatus{
			{
				Name:           "gm",
				OK:             false,
				UntriagedCount: 2, // These are the values at HEAD, i.e. the most recent data
				NegativeCount:  0,
			},
		},
	}, status)
}

// TestStatusWatcherEventBus tests that the status is re-calculated correctly after
// events fire on the eventbus.
func TestStatusWatcherEventBus(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	mts := &mocks.TileSource{}
	defer mes.AssertExpectations(t)
	defer mts.AssertExpectations(t)

	cpxTile := types.NewComplexTile(data.MakeTestTile())
	cpxTile.SetSparse(data.MakeTestCommits())
	mts.On("GetTile").Return(cpxTile)

	// The first time, we have the normal expectations (with things untriaged), then, we emulate
	// that a user has triaged the two untraiged images
	mes.On("Get").Return(data.MakeTestExpectations(), nil).Once()
	everythingTriaged := data.MakeTestExpectations()
	everythingTriaged.Set(data.AlphaTest, data.AlphaUntriaged1Digest, expectations.Positive)
	everythingTriaged.Set(data.BetaTest, data.BetaUntriaged1Digest, expectations.Negative)
	mes.On("Get").Return(everythingTriaged, nil)

	eb := eventbus.New()
	swc := StatusWatcherConfig{
		ExpectationsStore: mes,
		TileSource:        mts,
		EventBus:          eb,
	}

	watcher, err := New(swc)
	require.NoError(t, err)

	// status doesn't currently use the values of the delta, but this is what they
	// look like in production.
	eb.Publish(expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		ExpectationDelta: expstorage.Delta{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    expectations.Positive,
		},
	}, true)
	eb.Publish(expstorage.EV_EXPSTORAGE_CHANGED, &expstorage.EventExpectationChange{
		ExpectationDelta: expstorage.Delta{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    expectations.Negative,
		},
	}, true)

	commits := data.MakeTestCommits()
	require.Eventually(t, func() bool {
		status := watcher.GetStatus()
		return deepequal.DeepEqual(&GUIStatus{
			OK:            true,
			FirstCommit:   commits[0],
			LastCommit:    commits[2],
			TotalCommits:  3,
			FilledCommits: 3,
			CorpStatus: []*GUICorpusStatus{
				{
					Name:           "gm",
					OK:             true,
					UntriagedCount: 0,
					NegativeCount:  1,
				},
			},
		}, status)
	}, 2*time.Second, 100*time.Millisecond)

}
