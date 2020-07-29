package indexer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_diffstore "go.skia.org/infra/golden/go/diffstore/mocks"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/summary"
	gtestutils "go.skia.org/infra/golden/go/testutils"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
	mock_warmer "go.skia.org/infra/golden/go/warmer/mocks"
)

// TestIndexer_ExecutePipeline_NoChangeListsToIndex_Success tests a full indexing run, assuming
// nothing crashes or returns an error..
func TestIndexer_ExecutePipeline_Success(t *testing.T) {
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

	allTestDigests := types.DigestSlice{data.AlphaPositiveDigest, data.AlphaNegativeDigest, data.AlphaUntriagedDigest,
		data.BetaPositiveDigest, data.BetaUntriagedDigest}
	sort.Sort(allTestDigests)

	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	dMatcher := mock.MatchedBy(func(digests types.DigestSlice) bool {
		sort.Sort(digests)

		assert.Equal(t, allTestDigests, digests)
		return true
	})

	async(mgc.On("WriteKnownDigests", testutils.AnyContext, dMatcher).Return(nil))
	// The summary and counter are computed in indexer, so we should spot check their data.
	dataMatcher := mock.MatchedBy(func(wd warmer.Data) bool {
		// There's only one untriaged digest for each test (and they are alphabetical)
		assert.Equal(t, types.DigestSlice{data.AlphaUntriagedDigest}, wd.TestSummaries[0].UntHashes)
		assert.Equal(t, types.DigestSlice{data.BetaUntriagedDigest}, wd.TestSummaries[1].UntHashes)
		// These counts should include the ignored crosshatch traces
		assert.Equal(t, map[types.TestName]digest_counter.DigestCount{
			data.AlphaTest: {
				data.AlphaPositiveDigest:  2,
				data.AlphaNegativeDigest:  6,
				data.AlphaUntriagedDigest: 1,
			},
			data.BetaTest: {
				data.BetaPositiveDigest:  6,
				data.BetaUntriagedDigest: 1,
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
			UntHashes: types.DigestSlice{data.AlphaUntriagedDigest},
		},
	}

	ixr.lastMasterIndex = &SearchIndex{
		searchIndexConfig: searchIndexConfig{
			expectationsStore: mes,
			warmer:            mdw,
		},
		summaries: [2]countsAndBlames{alphaOnly, alphaOnly},
		dCounters: [2]digest_counter.DigestCounter{
			digest_counter.New(partialTile),
			digest_counter.New(fullTile),
		},
		preSliced: map[preSliceGroup][]*tiling.TracePair{},

		cpxTile: ct,
	}
	require.NoError(t, preSliceData(context.Background(), ixr.lastMasterIndex))

	ixr.indexTests(context.Background(), []expectations.ID{
		{
			// Pretend this digest was just triaged.
			Grouping: data.BetaTest,
			Digest:   data.BetaPositiveDigest,
		},
	})

	actualIndex := ixr.GetIndex()
	require.NotNil(t, actualIndex)

	sm := actualIndex.GetSummaries(types.ExcludeIgnoredTraces)
	require.Len(t, sm, 2)
	assert.Equal(t, data.AlphaTest, sm[0].Name)
	assert.Equal(t, data.BetaTest, sm[1].Name)

	// Spot check the summaries themselves.
	require.Equal(t, types.DigestSlice{data.AlphaUntriagedDigest}, sm[0].UntHashes)

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

func TestIndexer_CalcChangeListIndices_NoPreviousIndices_Success(t *testing.T) {
	unittest.SmallTest(t)

	const gerritCRS = "gerrit"
	const firstCLID = "111111"
	const patchsetFoxtrot = "foxtrot"
	const secondCLID = "22222"
	const patchsetSam = "sam"

	firstCombinedID := tjstore.CombinedPSID{CL: firstCLID, CRS: gerritCRS, PS: patchsetFoxtrot}
	secondCombinedID := tjstore.CombinedPSID{CL: secondCLID, CRS: gerritCRS, PS: patchsetSam}

	mcs := &mock_clstore.Store{}
	mes := &mock_expectations.Store{}
	mts := &mock_tjstore.Store{}

	masterExp := expectations.Expectations{}
	masterExp.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive)

	firstCLExp := expectations.Expectations{}
	firstCLExp.Set(data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative)

	// secondCL has no additional expectations
	mes.On("Get", testutils.AnyContext).Return(&masterExp, nil)
	loadChangeListExpectations(mes, gerritCRS, map[string]*expectations.Expectations{
		firstCLID:  &firstCLExp,
		secondCLID: {},
	})

	searchOptionsMatcher := mock.MatchedBy(func(so clstore.SearchOptions) bool {
		assert.Zero(t, so.StartIdx)
		assert.True(t, so.OpenCLsOnly)
		assert.Equal(t, maxCLsToIndex, so.Limit)
		assert.NotZero(t, so.After)
		return true
	})
	mcs.On("GetChangeLists", testutils.AnyContext, searchOptionsMatcher).Return([]code_review.ChangeList{
		// We don't look at the other fields since the index doesn't previously exist for this CL.
		{SystemID: firstCLID},
		{SystemID: secondCLID},
	}, 0, nil)
	// Reminder: only the most recent patchset is indexed.
	mcs.On("GetPatchSets", testutils.AnyContext, firstCLID).Return([]code_review.PatchSet{
		{SystemID: patchsetFoxtrot}, // all other fields ignored from PatchSet.
	}, nil)
	mcs.On("GetPatchSets", testutils.AnyContext, secondCLID).Return([]code_review.PatchSet{
		{SystemID: "not the most recent, so it is ignored"},
		{SystemID: patchsetSam},
	}, nil)

	androidGroup := paramtools.Params{
		"os":    "Android",
		"model": "crosshatch",
	}
	iosGroup := paramtools.Params{
		"os":    "iOS",
		"model": "iphone3",
	}
	firstOptionalGroup := paramtools.Params{
		"color_gamut": "wide",
	}
	secondOptionalGroup := paramtools.Params{
		"color_gamut": "narrow",
		"day_of_week": "wednesday",
	}

	mts.On("GetResults", testutils.AnyContext, firstCombinedID, time.Time{}).Return([]tjstore.TryJobResult{
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Options:      firstOptionalGroup,
			Digest:       data.AlphaPositiveDigest,
			// Other fields ignored
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  iosGroup,
			Options:      firstOptionalGroup,
			Digest:       data.AlphaNegativeDigest,
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Options:      secondOptionalGroup,
			Digest:       data.AlphaUntriagedDigest,
		},
	}, nil)
	mts.On("GetResults", testutils.AnyContext, secondCombinedID, time.Time{}).Return([]tjstore.TryJobResult{
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Options:      firstOptionalGroup,
			Digest:       data.AlphaPositiveDigest,
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  iosGroup,
			Options:      firstOptionalGroup,
			// Note, for this CL, this digest has not yet been triaged.
			Digest: data.AlphaNegativeDigest,
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Options:      firstOptionalGroup,
			Digest:       data.AlphaUntriagedDigest,
		},
	}, nil)

	ctx := context.Background()
	ic := IndexerConfig{
		ExpectationsStore: mes,
		TryJobStore:       mts,
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:    gerritCRS,
				Store: mcs,
				// URLTemplate and Client are unused here
			},
		},
	}
	ixr, err := New(ctx, ic, 0)
	require.NoError(t, err)
	ixr.changeListsReindexed.Reset()

	ixr.calcChangeListIndices(ctx)

	clIdx := ixr.GetIndexForCL(gerritCRS, firstCLID)
	assert.NotNil(t, clIdx)
	assert.Equal(t, firstCombinedID, clIdx.LatestPatchSet)
	assert.Len(t, clIdx.UntriagedResults, 1)
	assert.Equal(t, data.AlphaUntriagedDigest, clIdx.UntriagedResults[0].Digest)
	require.NotNil(t, clIdx.ParamSet)
	clIdx.ParamSet.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"color_gamut": []string{"narrow", "wide"},
		"day_of_week": []string{"wednesday"},
		"model":       []string{"crosshatch", "iphone3"},
		"name":        []string{"test_alpha"},
		"os":          []string{"Android", "iOS"},
	}, clIdx.ParamSet)

	clIdx = ixr.GetIndexForCL(gerritCRS, secondCLID)
	assert.NotNil(t, clIdx)
	assert.Equal(t, secondCombinedID, clIdx.LatestPatchSet)
	assert.Len(t, clIdx.UntriagedResults, 2)
	// Reminder, AlphaNegativeDigest was not triaged in the CL expectations for secondCLID
	assert.Equal(t, data.AlphaNegativeDigest, clIdx.UntriagedResults[0].Digest)
	assert.Equal(t, data.AlphaUntriagedDigest, clIdx.UntriagedResults[1].Digest)
	require.NotNil(t, clIdx.ParamSet)
	clIdx.ParamSet.Normalize()
	// secondCLID does not have the secondOptionalGroup, so it lacks a few key/values
	assert.Equal(t, paramtools.ParamSet{
		"color_gamut": []string{"wide"},
		"model":       []string{"crosshatch", "iphone3"},
		"name":        []string{"test_alpha"},
		"os":          []string{"Android", "iOS"},
	}, clIdx.ParamSet)

	assert.Equal(t, int64(2), ixr.changeListsReindexed.Get())
}

func TestIndexer_CalcChangeListIndices_HasIndexForPreviousPS_Success(t *testing.T) {
	unittest.SmallTest(t)

	const gerritCRS = "gerrit"
	const clID = "111111"
	const firstPatchSet = "firstPS"
	const secondPatchSet = "secondPS"
	firstPatchSetCombinedID := tjstore.CombinedPSID{CL: clID, CRS: gerritCRS, PS: firstPatchSet}
	secondPatchSetCombinedID := tjstore.CombinedPSID{CL: clID, CRS: gerritCRS, PS: secondPatchSet}

	longAgo := time.Date(2020, time.April, 15, 15, 15, 0, 0, time.UTC)
	recently := time.Date(2020, time.May, 5, 12, 12, 0, 0, time.UTC)

	mcs := &mock_clstore.Store{}
	mes := &mock_expectations.Store{}
	mts := &mock_tjstore.Store{}

	masterExp := expectations.Expectations{}
	masterExp.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive)
	masterExp.Set(data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative)

	// The CL has no additional expectations.
	mes.On("Get", testutils.AnyContext).Return(&masterExp, nil)
	loadChangeListExpectations(mes, gerritCRS, map[string]*expectations.Expectations{
		clID: {},
	})

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return([]code_review.ChangeList{
		{
			SystemID: clID,
			Updated:  recently,
		},
	}, 0, nil)

	mcs.On("GetPatchSets", testutils.AnyContext, clID).Return([]code_review.PatchSet{
		{SystemID: firstPatchSet}, // all other fields ignored from patch set.
		{SystemID: secondPatchSet},
	}, nil)

	androidGroup := paramtools.Params{
		"os":    "Android",
		"model": "crosshatch",
	}

	mts.On("GetResults", testutils.AnyContext, secondPatchSetCombinedID, time.Time{}).Return([]tjstore.TryJobResult{
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Digest:       data.AlphaPositiveDigest,
			// Other fields ignored
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Digest:       data.AlphaNegativeDigest,
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Digest:       data.AlphaUntriagedDigest,
		},
	}, nil)

	ctx := context.Background()
	ic := IndexerConfig{
		ExpectationsStore: mes,
		TryJobStore:       mts,
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:    gerritCRS,
				Store: mcs,
				// URLTemplate and Client are unused here
			},
		},
	}
	ixr, err := New(ctx, ic, 0)
	require.NoError(t, err)
	ixr.changeListsReindexed.Reset()

	// The scenario here is that the first PatchSet generated three untriaged digests.After that
	// index was computed, the user triaged AlphaPositiveDigest and AlphaNegativeDigest, and the
	// remainder of the data was uploaded to secondPatchSet. After the index is recomputed, the index
	// should be replaced with the new data (which reflects the new expectations).
	previousIdx := ChangeListIndex{
		LatestPatchSet: firstPatchSetCombinedID,
		UntriagedResults: []tjstore.TryJobResult{
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       data.AlphaPositiveDigest,
				// Other fields ignored
			},
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       data.AlphaNegativeDigest,
			},
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       data.AlphaUntriagedDigest,
			},
		},
		ParamSet: paramtools.ParamSet{
			// This ParamSet is purposely incomplete (i.e. no data.AlphaTest) to make sure new data is
			// merged in correctly.
			types.PrimaryKeyField: []string{"this_test_was_here_before"},
			"os":                  []string{"Android", "iOS"},
			"model":               []string{"bluefish", "redfish"},
		},
		ComputedTS: longAgo,
	}
	ixr.changeListIndices.Set("gerrit_111111", &previousIdx, 0)

	ixr.calcChangeListIndices(ctx)

	clIdx := ixr.GetIndexForCL(gerritCRS, clID)
	assert.NotNil(t, clIdx)
	assert.Equal(t, secondPatchSetCombinedID, clIdx.LatestPatchSet)
	assert.True(t, clIdx.ComputedTS.After(longAgo)) // should be updated
	assert.Len(t, clIdx.UntriagedResults, 1)
	assert.Equal(t, data.AlphaUntriagedDigest, clIdx.UntriagedResults[0].Digest)
	require.NotNil(t, clIdx.ParamSet)
	clIdx.ParamSet.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"model": []string{"bluefish", "crosshatch", "redfish"},
		"name":  []string{"test_alpha", "this_test_was_here_before"},
		"os":    []string{"Android", "iOS"},
	}, clIdx.ParamSet)
	assert.Equal(t, int64(1), ixr.changeListsReindexed.Get())
}

func TestIndexer_CalcChangeListIndices_HasIndexForCurrentPS_IncrementalUpdateSuccess(t *testing.T) {
	unittest.SmallTest(t)

	const gerritCRS = "gerrit"
	const clID = "111111"
	const firstPatchSet = "firstPS"
	const firstUntriagedDigest = types.Digest("11111111111111111111")
	const secondUntriagedDigest = types.Digest("22222222222222222222")
	const thirdUntriagedDigest = types.Digest("33333333333333333333")

	firstPatchSetCombinedID := tjstore.CombinedPSID{CL: clID, CRS: gerritCRS, PS: firstPatchSet}

	longAgo := time.Date(2020, time.April, 15, 15, 15, 0, 0, time.UTC)
	recently := time.Date(2020, time.May, 5, 12, 12, 0, 0, time.UTC)

	mcs := &mock_clstore.Store{}
	mes := &mock_expectations.Store{}
	mts := &mock_tjstore.Store{}

	// All digests are untriaged
	masterExp := expectations.Expectations{}

	// The CL has no additional expectations.
	mes.On("Get", testutils.AnyContext).Return(&masterExp, nil)
	loadChangeListExpectations(mes, gerritCRS, map[string]*expectations.Expectations{
		clID: {},
	})

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return([]code_review.ChangeList{
		{
			SystemID: clID,
			Updated:  recently,
		},
	}, 0, nil)

	mcs.On("GetPatchSets", testutils.AnyContext, clID).Return([]code_review.PatchSet{
		{SystemID: firstPatchSet}, // all other fields ignored from patch set.
	}, nil)

	androidGroup := paramtools.Params{
		"os":    "Android",
		"model": "crosshatch",
	}

	// Note that this time is based on the previous indexed time.
	mts.On("GetResults", testutils.AnyContext, firstPatchSetCombinedID, longAgo).Return([]tjstore.TryJobResult{
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Digest:       secondUntriagedDigest,
		},
		{
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
			GroupParams:  androidGroup,
			Digest:       thirdUntriagedDigest,
		},
	}, nil)

	ctx := context.Background()
	ic := IndexerConfig{
		ExpectationsStore: mes,
		TryJobStore:       mts,
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:    gerritCRS,
				Store: mcs,
				// URLTemplate and Client are unused here
			},
		},
	}
	ixr, err := New(ctx, ic, 0)
	require.NoError(t, err)
	ixr.changeListsReindexed.Reset()

	// The scenario here is that the first index for this patchset identified two untriaged digests.
	// Later, .
	previousIdx := ChangeListIndex{
		LatestPatchSet: firstPatchSetCombinedID,
		UntriagedResults: []tjstore.TryJobResult{
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       firstUntriagedDigest,
				// Other fields ignored
			},
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       secondUntriagedDigest,
			},
		},
		ParamSet: paramtools.ParamSet{
			// This ParamSet is purposely incomplete (i.e. no data.AlphaTest) to make sure new data is
			// merged in correctly.
			types.PrimaryKeyField: []string{"this_test_was_here_before"},
			"os":                  []string{"Android", "iOS"},
			"model":               []string{"bluefish", "redfish"},
		},
		ComputedTS: longAgo,
	}
	ixr.changeListIndices.Set("gerrit_111111", &previousIdx, 0)

	ixr.calcChangeListIndices(ctx)

	clIdx := ixr.GetIndexForCL(gerritCRS, clID)
	assert.NotNil(t, clIdx)
	assert.Equal(t, firstPatchSetCombinedID, clIdx.LatestPatchSet)
	assert.True(t, clIdx.ComputedTS.After(longAgo)) // should be updated
	assert.Len(t, clIdx.UntriagedResults, 3)
	assert.Equal(t, firstUntriagedDigest, clIdx.UntriagedResults[0].Digest)
	assert.Equal(t, secondUntriagedDigest, clIdx.UntriagedResults[1].Digest)
	assert.Equal(t, thirdUntriagedDigest, clIdx.UntriagedResults[2].Digest)
	require.NotNil(t, clIdx.ParamSet)
	clIdx.ParamSet.Normalize()
	assert.Equal(t, paramtools.ParamSet{
		"model": []string{"bluefish", "crosshatch", "redfish"},
		"name":  []string{"test_alpha", "this_test_was_here_before"},
		"os":    []string{"Android", "iOS"},
	}, clIdx.ParamSet)
	assert.Equal(t, int64(1), ixr.changeListsReindexed.Get())
}

func TestIndexer_CalcChangeListIndices_PreviousIndexDoesNotNeedUpdating_Success(t *testing.T) {
	unittest.SmallTest(t)

	const gerritCRS = "gerrit"
	const clID = "111111"
	const thePatchSet = "firstPS"
	thePatchSetCombinedID := tjstore.CombinedPSID{CL: clID, CRS: gerritCRS, PS: thePatchSet}

	now := time.Date(2020, time.May, 15, 15, 15, 0, 0, time.UTC)
	fiveMinAgo := now.Add(-5 * time.Minute)
	tenMinAgo := now.Add(-10 * time.Minute)

	mcs := &mock_clstore.Store{}
	mes := &mock_expectations.Store{}

	masterExp := expectations.Expectations{}
	masterExp.Set(data.AlphaTest, data.AlphaPositiveDigest, expectations.Positive)
	masterExp.Set(data.AlphaTest, data.AlphaNegativeDigest, expectations.Negative)

	// The CL has no additional expectations.
	mes.On("Get", testutils.AnyContext).Return(&masterExp, nil)
	loadChangeListExpectations(mes, gerritCRS, map[string]*expectations.Expectations{
		clID: {},
	})

	mcs.On("GetChangeLists", testutils.AnyContext, mock.Anything).Return([]code_review.ChangeList{
		{
			SystemID: clID,
			// CL was updated 10 minutes ago. That is, the ingester last got data from this CL 10 min
			// ago.
			Updated: tenMinAgo,
		},
	}, 0, nil)

	mcs.On("GetPatchSets", testutils.AnyContext, clID).Return([]code_review.PatchSet{
		{SystemID: thePatchSet}, // all other fields ignored from patch set.
	}, nil)

	ctx := context.Background()
	ic := IndexerConfig{
		ExpectationsStore: mes,
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:    gerritCRS,
				Store: mcs,
				// URLTemplate and Client are unused here
			},
		},
	}
	ixr, err := New(ctx, ic, 0)
	require.NoError(t, err)
	ixr.changeListsReindexed.Reset()

	// The scenario here is that the CL has not been updated since the index was made, so the index
	// should not be updated.
	previousIdx := ChangeListIndex{
		LatestPatchSet: thePatchSetCombinedID,
		UntriagedResults: []tjstore.TryJobResult{
			{
				ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.AlphaTest)},
				Digest:       data.AlphaUntriagedDigest,
				// Other fields ignored
			},
		},
		ParamSet: paramtools.ParamSet{
			// This ParamSet is purposely incomplete (i.e. no data.AlphaTest) to make sure new data is
			// merged in correctly.
			types.PrimaryKeyField: []string{"this_test_was_here_before"},
			"os":                  []string{"Android", "iOS"},
			"model":               []string{"bluefish", "redfish"},
		},
		ComputedTS: fiveMinAgo,
	}
	ixr.changeListIndices.Set("gerrit_111111", &previousIdx, 0)

	ixr.calcChangeListIndices(ctx)

	clIdx := ixr.GetIndexForCL(gerritCRS, clID)
	assert.NotNil(t, clIdx)
	assert.Equal(t, thePatchSetCombinedID, clIdx.LatestPatchSet)
	assert.Equal(t, clIdx.ComputedTS, fiveMinAgo) // should not be updated
	assert.Len(t, clIdx.UntriagedResults, 1)
	assert.Equal(t, int64(0), ixr.changeListsReindexed.Get())
}

// TestPreSlicedTracesCreatedCorrectly makes sure that we pre-slice the data based on IgnoreState,
// then Corpus, then TestName.
func TestPreSlicedTracesCreatedCorrectly(t *testing.T) {
	unittest.SmallTest(t)

	ct, _, _ := makeComplexTileWithCrosshatchIgnores()

	si := &SearchIndex{
		preSliced: map[preSliceGroup][]*tiling.TracePair{},
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
		preSliced: map[preSliceGroup][]*tiling.TracePair{},
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
		UntHashes: types.DigestSlice{data.AlphaUntriagedDigest},
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

func TestSearchIndex_MostRecentPositiveDigest_AllDigestsIdenticalAndPositive_Success(t *testing.T) {
	unittest.SmallTest(t)

	si, traceID := makeSearchIndexWithSingleTrace(goodDigest1, goodDigest1, goodDigest1)

	digest, err := si.MostRecentPositiveDigest(context.Background(), traceID)
	require.NoError(t, err)
	assert.Equal(t, goodDigest1, digest)
}

func TestSearchIndex_MostRecentPositiveDigest_MultiplePositiveDigests_Success(t *testing.T) {
	unittest.SmallTest(t)

	si, traceID := makeSearchIndexWithSingleTrace(goodDigest1, goodDigest2, goodDigest3)

	digest, err := si.MostRecentPositiveDigest(context.Background(), traceID)
	require.NoError(t, err)
	assert.Equal(t, goodDigest3, digest)
}

func TestSearchIndex_MostRecentPositiveDigest_LastPositiveNotAtHead_Success(t *testing.T) {
	unittest.SmallTest(t)

	si, traceID := makeSearchIndexWithSingleTrace(goodDigest1, badDigest1, goodDigest2, badDigest2, untriagedDigest1, tiling.MissingDigest)

	digest, err := si.MostRecentPositiveDigest(context.Background(), traceID)
	require.NoError(t, err)
	assert.Equal(t, goodDigest2, digest)
}

func TestSearchIndex_MostRecentPositiveDigest_NoRecentPositive_ReturnsMissingDigest(t *testing.T) {
	unittest.SmallTest(t)

	si, traceID := makeSearchIndexWithSingleTrace(untriagedDigest1, tiling.MissingDigest, badDigest1, untriagedDigest2, tiling.MissingDigest, badDigest2)

	digest, err := si.MostRecentPositiveDigest(context.Background(), traceID)
	require.NoError(t, err)
	assert.Equal(t, tiling.MissingDigest, digest)
}

func TestSearchIndex_MostRecentPositiveDigest_TraceNotFound_ReturnsMissingDigest(t *testing.T) {
	unittest.SmallTest(t)

	// We will ignore the trace in the index.
	si, _ := makeSearchIndexWithSingleTrace(goodDigest1)

	// Made-up trace ID. Should not be in the index.
	const missingTraceID = ",name=missing_trace,"

	// Assert that the made-up trace ID is actually missing.
	_, ok := si.cpxTile.GetTile(types.IncludeIgnoredTraces).Traces[missingTraceID]
	require.False(t, ok)

	digest, err := si.MostRecentPositiveDigest(context.Background(), missingTraceID)
	require.NoError(t, err)
	assert.Equal(t, tiling.MissingDigest, digest)
}

func TestSearchIndex_MostRecentPositiveDigest_ExpectationsStoreFailure_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	si, traceID := makeSearchIndexWithSingleTrace(goodDigest1)

	// Overwrite the index's expectations.Store with a faulty one.
	mes := &mock_expectations.Store{}
	mes.On("Get", testutils.AnyContext).Return(nil, errors.New("kaboom"))
	si.expectationsStore = mes

	_, err := si.MostRecentPositiveDigest(context.Background(), traceID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kaboom")
}

const (
	// Digests to be used in conjunction with makeSearchIndexWithSingleTrace().
	goodDigest1      = types.Digest("11111111111111111111111111111111")
	goodDigest2      = types.Digest("22222222222222222222222222222222")
	goodDigest3      = types.Digest("33333333333333333333333333333333")
	badDigest1       = types.Digest("bad11111111111111111111111111111")
	badDigest2       = types.Digest("bad22222222222222222222222222222")
	untriagedDigest1 = types.Digest("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	untriagedDigest2 = types.Digest("ffffffffffffffffffffffffffffffff")
)

// makeSearchIndexWithSingleTrace returns a SearchIndex comprised of a single trace with the given
// digests, and an expectations.Store with expectations for the good/badDigestN constants defined
// above.
func makeSearchIndexWithSingleTrace(digests ...types.Digest) (*SearchIndex, tiling.TraceID) {
	const device = "my_device"
	const corpus = "my_corpus"
	const testName = types.TestName("my_test")
	const extension = "png"

	// Constructed to resemble traceIDs seen in production. It shouldn't matter for these tests.
	traceID := tiling.TraceID(fmt.Sprintf(",device=%s,name=%s,source_type=%s,", device, testName, corpus))

	// Generate tile with the given digests.
	tile := &tiling.Tile{
		Traces: map[tiling.TraceID]*tiling.Trace{
			traceID: tiling.NewTrace(digests, map[string]string{
				"device":              device,
				types.PrimaryKeyField: string(testName),
				types.CorpusField:     corpus,
			}, map[string]string{
				"ext": extension,
			}),
		},
		ParamSet: map[string][]string{
			"device":              {device},
			types.PrimaryKeyField: {string(testName)},
			types.CorpusField:     {corpus},
			"ext":                 {extension},
		},
	}

	// Generate expectations for the known digests.
	var exps expectations.Expectations
	exps.Set(testName, goodDigest1, expectations.Positive)
	exps.Set(testName, goodDigest2, expectations.Positive)
	exps.Set(testName, goodDigest3, expectations.Positive)
	exps.Set(testName, badDigest1, expectations.Negative)
	exps.Set(testName, badDigest2, expectations.Negative)

	// Build mock expectations store.
	mockExpStore := &mock_expectations.Store{}
	mockExpStore.On("Get", testutils.AnyContext).Return(&exps, nil)

	// Build return value.
	searchIndex := &SearchIndex{
		searchIndexConfig: searchIndexConfig{
			expectationsStore: mockExpStore,
		},
		cpxTile: tiling.NewComplexTile(tile),
	}

	return searchIndex, traceID
}

// You may be tempted to just use a MockComplexTile here, but I was running into a race
// condition similar to https://github.com/stretchr/testify/issues/625 In essence, try
// to avoid having a mock (A) assert it was called with another mock (B) where the
// mock B is used elsewhere. There's a race because mock B is keeping track of what was
// called on it while mock A records what it was called with. Additionally, the general guidelines
// are to prefer to use the real thing instead of a mock.
func makeComplexTileWithCrosshatchIgnores() (tiling.ComplexTile, *tiling.Tile, *tiling.Tile) {
	fullTile := data.MakeTestTile()
	partialTile := data.MakeTestTile()
	delete(partialTile.Traces, data.CrosshatchAlphaTraceID)
	delete(partialTile.Traces, data.CrosshatchBetaTraceID)

	ct := tiling.NewComplexTile(fullTile)
	ct.SetIgnoreRules(partialTile, []paramtools.ParamSet{
		{
			"device": []string{"crosshatch"},
		},
	})
	return ct, fullTile, partialTile
}

func loadChangeListExpectations(masterExp *mock_expectations.Store, crs string, clExps map[string]*expectations.Expectations) {
	for clID, exp := range clExps {
		clStore := &mock_expectations.Store{}
		clStore.On("Get", testutils.AnyContext).Return(exp, nil)
		masterExp.On("ForChangeList", clID, crs).Return(clStore)
	}
}
