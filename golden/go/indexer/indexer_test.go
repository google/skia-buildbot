package indexer

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	mock_eventbus "go.skia.org/infra/go/eventbus/mocks"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TestIndexerInitialTriggerSunnyDay tests a full indexing run, assuming
// nothing crashes or returns an error.
func TestIndexerInitialTriggerSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mb := &mocks.Baseliner{}
	mds := &mocks.DiffStore{}
	mdw := &mocks.DiffWarmer{}
	meb := &mock_eventbus.EventBus{}
	mes := &mocks.ExpectationsStore{}
	mgc := &mocks.GCSClient{}

	defer mb.AssertExpectations(t)
	defer mds.AssertExpectations(t)
	defer mdw.AssertExpectations(t)
	defer meb.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	ct, _, _ := makeComplexTileWithCrosshatchIgnores()

	storages := &storage.Storage{
		ExpectationsStore: mes,
		DiffStore:         mds,
		EventBus:          meb,
		GCSClient:         mgc,
		Baseliner:         mb,
	}
	wg, isAsync, asyncWrapper := asyncHelpers()

	allTestDigests := types.DigestSlice{data.AlphaGood1Digest, data.AlphaBad1Digest, data.AlphaUntriaged1Digest,
		data.BetaGood1Digest, data.BetaUntriaged1Digest}
	sort.Sort(allTestDigests)

	mb.On("CanWriteBaseline").Return(true)
	isAsync(mb.On("PushMasterBaselines", ct, "")).Return(nil, nil)

	mes.On("Get").Return(data.MakeTestExpectations(), nil)

	// Return a non-empty map just to make sure things don't crash - this doesn't actually
	// affect any of the assertions.
	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{
		unavailableDigest: {
			Digest: unavailableDigest,
			Reason: "on vacation",
			// Arbitrary date
			TS: time.Date(2017, time.October, 5, 4, 3, 2, 0, time.UTC).UnixNano() / int64(time.Millisecond),
		},
	})

	mds.On("WarmDigests", diff.PRIORITY_NOW, mock.AnythingOfType("types.DigestSlice"), true).Run(asyncWrapper(func(args mock.Arguments) {
		digests := args.Get(1).(types.DigestSlice)
		sort.Sort(digests)

		assert.Equal(t, allTestDigests, digests)
	}))

	mgc.On("WriteKnownDigests", mock.AnythingOfType("types.DigestSlice")).Run(asyncWrapper(func(args mock.Arguments) {
		digests := args.Get(0).(types.DigestSlice)
		sort.Sort(digests)

		assert.Equal(t, allTestDigests, digests)
	})).Return(nil)

	publishedSearchIndex := (*SearchIndex)(nil)

	meb.On("Publish", EV_INDEX_UPDATED, mock.AnythingOfType("*indexer.SearchIndex"), false).Run(func(args mock.Arguments) {
		si := args.Get(1).(*SearchIndex)
		assert.NotNil(t, si)

		publishedSearchIndex = si
	}).Return(nil)

	// The first and third params are computed in indexer, so we should spot check their data
	mdw.On("PrecomputeDiffs", mock.AnythingOfType("summary.SummaryMap"), types.TestNameSet(nil), mock.AnythingOfType("*digest_counter.Counter"), mock.AnythingOfType("*digesttools.Impl")).Run(asyncWrapper(func(args mock.Arguments) {
		sm := args.Get(0).(summary.SummaryMap)
		assert.NotNil(t, sm)
		dCounter := args.Get(2).(*digest_counter.Counter)
		assert.NotNil(t, dCounter)

		// There's only one untriaged digest for each test
		assert.Equal(t, types.DigestSlice{data.AlphaUntriaged1Digest}, sm[data.AlphaTest].UntHashes)
		assert.Equal(t, types.DigestSlice{data.BetaUntriaged1Digest}, sm[data.BetaTest].UntHashes)

		// These counts should include the ignored crosshatch traces
		assert.Equal(t, map[types.TestName]digest_counter.DigestCount{
			data.AlphaTest: {
				data.AlphaGood1Digest:      2,
				data.AlphaBad1Digest:       6,
				data.AlphaUntriaged1Digest: 1,
			},
			data.BetaTest: {
				data.BetaGood1Digest:      6,
				data.BetaUntriaged1Digest: 1,
			},
		}, dCounter.ByTest())
	}))

	ixr, err := New(storages, mdw, 0)
	assert.NoError(t, err)

	err = ixr.executePipeline(ct)
	assert.NoError(t, err)
	assert.NotNil(t, publishedSearchIndex)
	actualIndex := ixr.GetIndex()
	assert.NotNil(t, actualIndex)

	assert.Equal(t, publishedSearchIndex, actualIndex)

	// Block until all async calls are finished so the assertExpectations calls
	// can properly check that their functions were called.
	wg.Wait()
}

// TestIndexerPartialUpdate tests the part of indexer that runs when expectations change
// and we need to re-index a subset of the data, namely that which had tests change
// (e.g. from Untriaged to Positive or whatever).
func TestIndexerPartialUpdate(t *testing.T) {
	unittest.SmallTest(t)

	mb := &mocks.Baseliner{}
	mdw := &mocks.DiffWarmer{}
	meb := &mock_eventbus.EventBus{}
	mes := &mocks.ExpectationsStore{}

	defer mb.AssertExpectations(t)
	defer mdw.AssertExpectations(t)
	defer meb.AssertExpectations(t)
	defer mes.AssertExpectations(t)

	ct, fullTile, partialTile := makeComplexTileWithCrosshatchIgnores()

	wg, isAsync, asyncWrapper := asyncHelpers()

	mes.On("Get").Return(data.MakeTestExpectations(), nil)

	mb.On("CanWriteBaseline").Return(true)
	isAsync(mb.On("PushMasterBaselines", ct, "")).Return(nil, nil)

	meb.On("Publish", EV_INDEX_UPDATED, mock.AnythingOfType("*indexer.SearchIndex"), false).Return(nil)

	// Make sure PrecomputeDiffs is only told to recompute BetaTest.
	tn := types.TestNameSet{data.BetaTest: true}
	mdw.On("PrecomputeDiffs", mock.AnythingOfType("summary.SummaryMap"), tn, mock.AnythingOfType("*digest_counter.Counter"), mock.AnythingOfType("*digesttools.Impl")).Run(asyncWrapper(func(args mock.Arguments) {
		sm := args.Get(0).(summary.SummaryMap)
		assert.NotNil(t, sm)
		dCounter := args.Get(2).(*digest_counter.Counter)
		assert.NotNil(t, dCounter)
	}))

	storages := &storage.Storage{
		ExpectationsStore: mes,
		EventBus:          meb,
		Baseliner:         mb,
	}

	ixr, err := New(storages, mdw, 0)
	assert.NoError(t, err)

	alphaOnly := summary.SummaryMap{
		data.AlphaTest: {
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
		},
	}

	ixr.lastIndex = &SearchIndex{
		storages:  storages,
		summaries: []summary.SummaryMap{alphaOnly, alphaOnly},
		dCounters: []digest_counter.DigestCounter{
			digest_counter.New(partialTile),
			digest_counter.New(fullTile),
		},

		cpxTile: ct,
	}

	ixr.indexTests([]types.Expectations{
		{
			data.BetaTest: {
				// Pretend this digest was just marked positive.
				data.BetaGood1Digest: types.POSITIVE,
			},
		},
	})

	actualIndex := ixr.GetIndex()
	assert.NotNil(t, actualIndex)

	sm := actualIndex.GetSummaries(types.ExcludeIgnoredTraces)
	assert.Contains(t, sm, data.AlphaTest)
	assert.Contains(t, sm, data.BetaTest)

	// Spot check the summaries themselves.
	assert.Equal(t, types.DigestSlice{data.AlphaUntriaged1Digest}, sm[data.AlphaTest].UntHashes)

	assert.Equal(t, &summary.Summary{
		Name:      data.BetaTest,
		Pos:       1,
		Neg:       0,
		Untriaged: 0, // Reminder that the untriaged image for BetaTest was ignored by the rules.
		UntHashes: types.DigestSlice{},
		Num:       1,
		Corpus:    "gm",
		Blame:     []*blame.WeightedBlame{},
	}, sm[data.BetaTest])
	// Block until all async calls are finished so the assertExpectations calls
	// can properly check that their functions were called.
	wg.Wait()
}

const (
	// valid, but arbitrary md5 hash
	unavailableDigest = types.Digest("fed541470e246b63b313930523220de8")
)

// You may be tempted to just use a MockComplexTile here, but I was running into a race
// condition similar to https://github.com/stretchr/testify/issues/625 In essence, try
// to avoid having a mock (A) assert it was called with another mock (B) where the
// mock B is used elsewhere. There's a race because mock B is keeping track of what was
// called on it while mock A records what it was called with.
func makeComplexTileWithCrosshatchIgnores() (types.ComplexTile, *tiling.Tile, *tiling.Tile) {
	fullTile := data.MakeTestTile()
	partialTile := data.MakeTestTile()
	delete(partialTile.Traces, "crosshatch:test_alpha:gm")
	delete(partialTile.Traces, "crosshatch:test_beta:gm")

	ct := types.NewComplexTile(fullTile)
	ct.SetIgnoreRules(partialTile, []paramtools.ParamSet{
		{
			"device": []string{"crosshatch"},
		},
	}, 1)
	return ct, fullTile, partialTile
}

// These helpers assist in wrapping the Run calls in the wait group
// so we can be sure everything actually runs before the function
// terminates. Ideally, I would have liked to be able to chain multiple
// Run calls to the mock, but testify's mocks only allow one
// Run per Call. We have two helpers then, one if a mock does not already
// have a Run function and the other is for wrapping around a Run function
// that already exists.
// Note: do not call defer wg.Wait() because if any assert fails, it will
// panic, possibly before all the wg.Done() are called, causing a deadlock.
func asyncHelpers() (*sync.WaitGroup, func(c *mock.Call) *mock.Call, func(f func(args mock.Arguments)) func(mock.Arguments)) {
	wg := sync.WaitGroup{}
	isAsync := func(c *mock.Call) *mock.Call {
		wg.Add(1)
		return c.Run(func(a mock.Arguments) {
			wg.Done()
		})
	}
	asyncWrapper := func(f func(args mock.Arguments)) func(mock.Arguments) {
		wg.Add(1)
		return func(args mock.Arguments) {
			defer wg.Done()
			f(args)
		}
	}
	return &wg, isAsync, asyncWrapper
}
