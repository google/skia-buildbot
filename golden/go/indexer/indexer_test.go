package indexer

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	mock_diffstore "go.skia.org/infra/golden/go/diffstore/mocks"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/summary"
	gtestutils "go.skia.org/infra/golden/go/testutils"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
	mock_warmer "go.skia.org/infra/golden/go/warmer/mocks"
)

// TestIndexerInitialTriggerSunnyDay tests a full indexing run, assuming
// nothing crashes or returns an error.
func TestIndexerInitialTriggerSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mds := &mock_diffstore.DiffStore{}
	mdw := &mock_warmer.DiffWarmer{}
	mes := &mock_expectations.Store{}
	mgc := &mocks.GCSClient{}

	defer mds.AssertExpectations(t)
	defer mdw.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer mgc.AssertExpectations(t)

	ct, _, _ := makeComplexTileWithCrosshatchIgnores()

	ic := IndexerConfig{
		DiffStore:         mds,
		ExpectationsStore: mes,
		GCSClient:         mgc,
		Warmer:            mdw,
	}
	wg, async, _ := gtestutils.AsyncHelpers()

	allTestDigests := types.DigestSlice{data.AlphaGood1Digest, data.AlphaBad1Digest, data.AlphaUntriaged1Digest,
		data.BetaGood1Digest, data.BetaUntriaged1Digest}
	sort.Sort(allTestDigests)

	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	// Return a non-empty map just to make sure things don't crash - this doesn't actually
	// affect any of the assertions.
	mds.On("UnavailableDigests", testutils.AnyContext).Return(map[types.Digest]*diff.DigestFailure{
		unavailableDigest: {
			Digest: unavailableDigest,
			Reason: "on vacation",
			// Arbitrary date
			TS: time.Date(2017, time.October, 5, 4, 3, 2, 0, time.UTC).UnixNano() / int64(time.Millisecond),
		},
	}, nil)

	dMatcher := mock.MatchedBy(func(digests types.DigestSlice) bool {
		sort.Sort(digests)

		assert.Equal(t, allTestDigests, digests)
		return true
	})

	async(mgc.On("WriteKnownDigests", testutils.AnyContext, dMatcher).Return(nil))
	// The summary and counter are computed in indexer, so we should spot check their data.
	dataMatcher := mock.MatchedBy(func(wd warmer.Data) bool {
		// There's only one untriaged digest for each test (and they are alphabetical)
		assert.Equal(t, types.DigestSlice{data.AlphaUntriaged1Digest}, wd.TestSummaries[0].UntHashes)
		assert.Equal(t, types.DigestSlice{data.BetaUntriaged1Digest}, wd.TestSummaries[1].UntHashes)
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
		}, wd.DigestsByTest)
		assert.Nil(t, wd.SubsetOfTests)
		return true
	})

	async(mdw.On("PrecomputeDiffs", testutils.AnyContext, dataMatcher, mock.AnythingOfType("*digesttools.Impl")).Return(nil))

	ixr, err := New(context.Background(), ic, 0)
	require.NoError(t, err)

	err = ixr.executePipeline(context.Background(), ct)
	require.NoError(t, err)
	actualIndex := ixr.GetIndex()
	require.NotNil(t, actualIndex)

	// Block until all async calls are finished so the assertExpectations calls
	// can properly check that their functions were called.
	wg.Wait()
}

// TestIndexerPartialUpdate tests the part of indexer that runs when expectations change
// and we need to re-index a subset of the data, namely that which had tests change
// (e.g. from Untriaged to Positive or whatever).
func TestIndexerPartialUpdate(t *testing.T) {
	unittest.SmallTest(t)

	mdw := &mock_warmer.DiffWarmer{}
	mes := &mock_expectations.Store{}

	defer mdw.AssertExpectations(t)
	defer mes.AssertExpectations(t)

	ct, fullTile, partialTile := makeComplexTileWithCrosshatchIgnores()
	require.NotEqual(t, fullTile, partialTile)

	wg, async, _ := gtestutils.AsyncHelpers()

	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	// The summary and counter are computed in indexer, so we should spot check their data.
	dataMatcher := mock.MatchedBy(func(wd warmer.Data) bool {
		// Make sure PrecomputeDiffs is only told to recompute BetaTest.
		assert.Equal(t, wd.SubsetOfTests, types.TestNameSet{data.BetaTest: true})
		assert.Len(t, wd.TestSummaries, 2)
		return true
	})

	async(mdw.On("PrecomputeDiffs", testutils.AnyContext, dataMatcher, mock.AnythingOfType("*digesttools.Impl")).Return(nil))

	ic := IndexerConfig{
		ExpectationsStore: mes,
		Warmer:            mdw,
	}

	ixr, err := New(context.Background(), ic, 0)
	require.NoError(t, err)

	alphaOnly := []*summary.TriageStatus{
		{
			Name:      data.AlphaTest,
			Untriaged: 1,
			UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
		},
	}

	ixr.lastIndex = &SearchIndex{
		searchIndexConfig: searchIndexConfig{
			expectationsStore: mes,
			warmer:            mdw,
		},
		summaries: [2]countsAndBlames{alphaOnly, alphaOnly},
		dCounters: [2]digest_counter.DigestCounter{
			digest_counter.New(partialTile),
			digest_counter.New(fullTile),
		},
		preSliced: map[preSliceGroup][]*types.TracePair{},

		cpxTile: ct,
	}
	require.NoError(t, preSliceData(context.Background(), ixr.lastIndex))

	ixr.indexTests(context.Background(), []expectations.Delta{
		{
			// Pretend this digest was just marked positive.
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    expectations.Positive,
		},
	})

	actualIndex := ixr.GetIndex()
	require.NotNil(t, actualIndex)

	sm := actualIndex.GetSummaries(types.ExcludeIgnoredTraces)
	require.Len(t, sm, 2)
	assert.Equal(t, data.AlphaTest, sm[0].Name)
	assert.Equal(t, data.BetaTest, sm[1].Name)

	// Spot check the summaries themselves.
	require.Equal(t, types.DigestSlice{data.AlphaUntriaged1Digest}, sm[0].UntHashes)

	require.Equal(t, &summary.TriageStatus{
		Name:      data.BetaTest,
		Pos:       1,
		Neg:       0,
		Untriaged: 0, // Reminder that the untriaged image for BetaTest was ignored by the rules.
		UntHashes: types.DigestSlice{},
		Num:       1,
		Corpus:    "gm",
		Blame:     []blame.WeightedBlame{},
	}, sm[1])
	// Block until all async calls are finished so the assertExpectations calls
	// can properly check that their functions were called.
	wg.Wait()
}

// TestPreSlicedTracesCreatedCorrectly makes sure that we pre-slice the data based on IgnoreState,
// then Corpus, then TestName.
func TestPreSlicedTracesCreatedCorrectly(t *testing.T) {
	unittest.SmallTest(t)

	ct, _, _ := makeComplexTileWithCrosshatchIgnores()

	si := &SearchIndex{
		preSliced: map[preSliceGroup][]*types.TracePair{},
		cpxTile:   ct,
	}
	require.NoError(t, preSliceData(context.Background(), si))

	// (2 IgnoreStates) + (2 IgnoreStates * 1 corpus) + (2 IgnoreStates * 1 corpus * 2 tests)
	assert.Len(t, si.preSliced, 8)
	allCombos := []preSliceGroup{
		{
			IgnoreState: types.IncludeIgnoredTraces,
		},
		{
			IgnoreState: types.ExcludeIgnoredTraces,
		},
		{
			IgnoreState: types.IncludeIgnoredTraces,
			Corpus:      "gm",
		},
		{
			IgnoreState: types.ExcludeIgnoredTraces,
			Corpus:      "gm",
		},
		{
			IgnoreState: types.IncludeIgnoredTraces,
			Corpus:      "gm",
			Test:        data.AlphaTest,
		},
		{
			IgnoreState: types.IncludeIgnoredTraces,
			Corpus:      "gm",
			Test:        data.BetaTest,
		},
		{
			IgnoreState: types.ExcludeIgnoredTraces,
			Corpus:      "gm",
			Test:        data.AlphaTest,
		},
		{
			IgnoreState: types.ExcludeIgnoredTraces,
			Corpus:      "gm",
			Test:        data.BetaTest,
		},
	}
	for _, psg := range allCombos {
		assert.Contains(t, si.preSliced, psg)
	}
}

// TestPreSlicedTracesQuery tests that querying SlicedTraces returns the correct set of tracePairs
// from the preSliced data. This especially includes if multiple tests are in the query.
func TestPreSlicedTracesQuery(t *testing.T) {
	unittest.SmallTest(t)
	ct, _, _ := makeComplexTileWithCrosshatchIgnores()

	si := &SearchIndex{
		preSliced: map[preSliceGroup][]*types.TracePair{},
		cpxTile:   ct,
	}
	require.NoError(t, preSliceData(context.Background(), si))

	allTraces := si.SlicedTraces(types.IncludeIgnoredTraces, nil)
	assert.Len(t, allTraces, 6)

	withIgnores := si.SlicedTraces(types.ExcludeIgnoredTraces, nil)
	assert.Len(t, withIgnores, 4)

	justCorpus := si.SlicedTraces(types.IncludeIgnoredTraces, map[string][]string{
		types.CorpusField: {"gm"},
	})
	assert.Len(t, justCorpus, 6)

	bothTests := si.SlicedTraces(types.IncludeIgnoredTraces, map[string][]string{
		types.CorpusField:     {"gm"},
		types.PrimaryKeyField: {string(data.BetaTest), string(data.AlphaTest)},
	})
	assert.Len(t, bothTests, 6)

	oneTest := si.SlicedTraces(types.ExcludeIgnoredTraces, map[string][]string{
		types.CorpusField:     {"gm"},
		types.PrimaryKeyField: {string(data.AlphaTest)},
	})
	assert.Len(t, oneTest, 2)

	noMatches := si.SlicedTraces(types.ExcludeIgnoredTraces, map[string][]string{
		types.CorpusField:     {"nope"},
		types.PrimaryKeyField: {string(data.AlphaTest)},
	})
	assert.Empty(t, noMatches)
}

// SummarizeByGrouping tests computing summaries for a given corpus. This emulates the underlying
// call used in the byBlame handler.
func TestSummarizeByGrouping(t *testing.T) {
	unittest.SmallTest(t)
	ct, _, partialTile := makeComplexTileWithCrosshatchIgnores()
	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)
	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	dc := digest_counter.New(partialTile)
	b, err := blame.New(partialTile, data.MakeTestExpectations())
	require.NoError(t, err)

	// We can leave ParamSummary blank because they are unused.
	si, err := SearchIndexForTesting(ct, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{}, mes, b)
	require.NoError(t, err)

	sums, err := si.SummarizeByGrouping(context.Background(), "gm", nil, types.ExcludeIgnoredTraces, true)
	require.NoError(t, err)
	assert.Len(t, sums, 2)
	assert.Contains(t, sums, &summary.TriageStatus{
		Name:      data.AlphaTest,
		Corpus:    "gm",
		Pos:       1,
		Neg:       0,
		Untriaged: 1,
		Num:       2,
		UntHashes: types.DigestSlice{data.AlphaUntriaged1Digest},
		Blame: []blame.WeightedBlame{
			{
				Author: data.ThirdCommitAuthor,
				Prob:   1,
			},
		},
	})
	assert.Contains(t, sums, &summary.TriageStatus{
		Name:      data.BetaTest,
		Corpus:    "gm",
		Pos:       1,
		Neg:       0,
		Untriaged: 0, // this untriaged one was hidden by the ignores
		Num:       1,
		UntHashes: types.DigestSlice{},
		Blame:     []blame.WeightedBlame{},
	})
}

const (
	// valid, but arbitrary md5 hash
	unavailableDigest = types.Digest("fed541470e246b63b313930523220de8")
)

// You may be tempted to just use a MockComplexTile here, but I was running into a race
// condition similar to https://github.com/stretchr/testify/issues/625 In essence, try
// to avoid having a mock (A) assert it was called with another mock (B) where the
// mock B is used elsewhere. There's a race because mock B is keeping track of what was
// called on it while mock A records what it was called with. Additionally, the general guidelines
// are to prefer to use the real thing instead of a mock.
func makeComplexTileWithCrosshatchIgnores() (types.ComplexTile, *tiling.Tile, *tiling.Tile) {
	fullTile := data.MakeTestTile()
	partialTile := data.MakeTestTile()
	delete(partialTile.Traces, data.CrosshatchAlphaTraceID)
	delete(partialTile.Traces, data.CrosshatchBetaTraceID)

	ct := types.NewComplexTile(fullTile)
	ct.SetIgnoreRules(partialTile, []paramtools.ParamSet{
		{
			"device": []string{"crosshatch"},
		},
	})
	return ct, fullTile, partialTile
}
