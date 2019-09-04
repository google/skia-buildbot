package search

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestSearchThreeDevicesSunnyDay searches over the three_devices
// test data for untriaged images at head, essentially the default search.
// We expect to get two untriaged digests, with their closest positive and
// negative images (if any).
func TestSearchThreeDevicesSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	mi := &mock_index.IndexSource{}
	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	defer mes.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)

	s := New(mds, mes, mi, nil, nil, nil, everythingPublic)

	mes.On("Get").Return(data.MakeTestExpectations(), nil)

	mi.On("GetIndex").Return(mis)

	cpxTile := types.NewComplexTile(data.MakeTestTile())
	mis.On("Tile").Return(cpxTile)
	dc := digest_counter.New(data.MakeTestTile())
	mis.On("DigestCountsByTrace", types.ExcludeIgnoredTraces).Return(dc.ByTrace())
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(dc.ByTest())

	ps := paramsets.NewParamSummary(data.MakeTestTile(), dc)
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(ps.GetByTest())

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})
	// Positive match
	mds.On("Get", diff.PRIORITY_NOW, data.AlphaUntriaged1Digest, types.DigestSlice{data.AlphaGood1Digest}).
		Return(map[types.Digest]interface{}{
			data.AlphaGood1Digest: makeSmallDiffMetric(),
		}, nil)
	// Negative match
	mds.On("Get", diff.PRIORITY_NOW, data.AlphaUntriaged1Digest, types.DigestSlice{data.AlphaBad1Digest}).
		Return(map[types.Digest]interface{}{
			data.AlphaBad1Digest: makeBigDiffMetric(),
		}, nil)
	// Positive match
	mds.On("Get", diff.PRIORITY_NOW, data.BetaUntriaged1Digest, types.DigestSlice{data.BetaGood1Digest}).
		Return(map[types.Digest]interface{}{
			data.BetaGood1Digest: makeBigDiffMetric(),
		}, nil)
	// BetaUntriaged1Digest has no negative images to compare against, so diffstore isn't queried.

	q := &query.Search{
		ChangeListID:    "",
		DeprecatedIssue: types.LegacyMasterBranch,
		Unt:             true,
		Head:            true,

		Metric:   diff.METRIC_COMBINED,
		FRGBAMin: 0,
		FRGBAMax: 255,
		FDiffMax: -1,
		Sort:     query.SortAscending,
	}

	resp, err := s.Search(context.Background(), q)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	assert.Equal(t, &frontend.SearchResponse{
		Commits: data.MakeTestCommits(),
		Offset:  0,
		Size:    2,
		Digests: []*frontend.SRDigest{
			// AlphaTest comes first because we are sorting by ascending
			// "combined" metric, and AlphaTest's closest match is the
			// small diff metric, whereas BetaTest's only match is the
			// big diff metric.
			{
				Test:   data.AlphaTest,
				Digest: data.AlphaUntriaged1Digest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":                {data.BullheadDevice},
					types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
					types.CORPUS_FIELD:      {"gm"},
				},
				Traces: &frontend.TraceGroup{
					TileSize: 3, // 3 commits in tile
					Traces: []frontend.Trace{
						{
							Data: []frontend.Point{
								{X: 0, Y: 0, S: 1},
								{X: 1, Y: 0, S: 1},
								{X: 2, Y: 0, S: 0},
							},
							ID: data.BullheadAlphaTraceID,
							Params: map[string]string{
								"device":                data.BullheadDevice,
								types.PRIMARY_KEY_FIELD: string(data.AlphaTest),
								types.CORPUS_FIELD:      "gm",
							},
						},
					},
					Digests: []frontend.DigestStatus{
						{
							Digest: data.AlphaUntriaged1Digest,
							Status: "untriaged",
						},
						{
							Digest: data.AlphaBad1Digest,
							Status: "negative",
						},
					},
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						DiffMetrics: makeSmallDiffMetric(),
						Digest:      data.AlphaGood1Digest,
						Status:      "positive",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.CrosshatchDevice},
							types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						OccurrencesInTile: 2,
					},
					common.NegativeRef: {
						DiffMetrics: makeBigDiffMetric(),
						Digest:      data.AlphaBad1Digest,
						Status:      "negative",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
							types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						OccurrencesInTile: 6,
					},
				},
			},
			{
				Test:   data.BetaTest,
				Digest: data.BetaUntriaged1Digest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":                {data.CrosshatchDevice},
					types.PRIMARY_KEY_FIELD: {string(data.BetaTest)},
					types.CORPUS_FIELD:      {"gm"},
				},
				Traces: &frontend.TraceGroup{
					TileSize: 3,
					Traces: []frontend.Trace{
						{
							Data: []frontend.Point{
								{X: 0, Y: 0, S: 0},
								// Other two commits were missing
							},
							ID: data.CrosshatchBetaTraceID,
							Params: map[string]string{
								"device":                data.CrosshatchDevice,
								types.PRIMARY_KEY_FIELD: string(data.BetaTest),
								types.CORPUS_FIELD:      "gm",
							},
						},
					},
					Digests: []frontend.DigestStatus{
						{
							Digest: data.BetaUntriaged1Digest,
							Status: "untriaged",
						},
					},
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						DiffMetrics: makeBigDiffMetric(),
						Digest:      data.BetaGood1Digest,
						Status:      "positive",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.BullheadDevice},
							types.PRIMARY_KEY_FIELD: {string(data.BetaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						OccurrencesInTile: 6,
					},
					common.NegativeRef: nil,
				},
			},
		},
	}, resp)
}

// TestSearchThreeDevicesChangeListSunnyDay covers the case
// where two tryjobs have been run on a given CL and PS, one on the
// angler bot and one on the bullhead bot. The master branch
// looks like in the ThreeDevices data set. The outputs produced are
// Test  |  Device  | Digest
// ----------------------
// Alpha | Angler   | data.AlphaGood1Digest
// Alpha | Bullhead | data.AlphaUntriaged1Digest
// Beta  | Angler   | data.BetaGood1Digest
// Beta  | Bullhead | BetaBrandNewDigest
//
// The user has triaged the data.AlphaUntriaged1Digest as positive
// but BetaBrandNewDigest remains untriaged.
// With this setup, we do a default query (don't show master,
// only untriaged digests) and expect to see only an entry about
// BetaBrandNewDigest.
func TestSearchThreeDevicesChangeListSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	clInt := int64(1234)
	clID := "1234"
	AlphaNowGoodDigest := data.AlphaUntriaged1Digest
	BetaBrandNewDigest := types.Digest("be7a03256511bec3a7453c3186bb2e07")

	mes := &mocks.ExpectationsStore{}
	issueStore := &mocks.ExpectationsStore{}
	mi := &mock_index.IndexSource{}
	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	defer mes.AssertExpectations(t)
	defer issueStore.AssertExpectations(t)
	defer mi.AssertExpectations(t)
	defer mis.AssertExpectations(t)
	defer mds.AssertExpectations(t)
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mes.On("ForIssue", clInt).Return(issueStore, nil)
	issueStore.On("Get").Return(types.Expectations{
		data.AlphaTest: {
			AlphaNowGoodDigest: types.POSITIVE,
		},
	}, nil)
	mes.On("Get").Return(data.MakeTestExpectations(), nil)

	mi.On("GetIndex").Return(mis)

	cpxTile := types.NewComplexTile(data.MakeTestTile())
	mis.On("Tile").Return(cpxTile)
	dc := digest_counter.New(data.MakeTestTile())
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(dc.ByTest())
	mis.On("GetIgnoreMatcher").Return(paramtools.ParamMatcher{})

	ps := paramsets.NewParamSummary(data.MakeTestTile(), dc)
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(ps.GetByTest())

	mcls.On("GetPatchSets", anyctx, clID).Return([]code_review.PatchSet{
		{
			SystemID:     "first_one",
			ChangeListID: clID,
			Order:        1,
			// All the rest are ignored
		},
		{
			SystemID:     "fourth_one",
			ChangeListID: clID,
			Order:        4,
			// All the rest are ignored
		},
	}, nil)
	mcls.On("System").Return("gerrit")

	expectedID := tjstore.CombinedPSID{
		CL:  clID,
		CRS: "gerrit",
		PS:  "fourth_one", // we didn't specify a PS, so it goes with the most recent
	}
	anglerGroup := map[string]string{
		"device": data.AnglerDevice,
	}
	bullheadGroup := map[string]string{
		"device": data.BullheadDevice,
	}
	options := map[string]string{
		"ext": "png",
	}

	mtjs.On("GetResults", anyctx, expectedID).Return([]tjstore.TryJobResult{
		{
			GroupParams: anglerGroup,
			Options:     options,
			Digest:      data.AlphaGood1Digest,
			ResultParams: map[string]string{
				types.PRIMARY_KEY_FIELD: string(data.AlphaTest),
				types.CORPUS_FIELD:      "gm",
			},
		},
		{
			GroupParams: bullheadGroup,
			Options:     options,
			Digest:      AlphaNowGoodDigest,
			ResultParams: map[string]string{
				types.PRIMARY_KEY_FIELD: string(data.AlphaTest),
				types.CORPUS_FIELD:      "gm",
			},
		},
		{
			GroupParams: anglerGroup,
			Options:     options,
			Digest:      data.BetaGood1Digest,
			ResultParams: map[string]string{
				types.PRIMARY_KEY_FIELD: string(data.BetaTest),
				types.CORPUS_FIELD:      "gm",
			},
		},
		{
			GroupParams: bullheadGroup,
			Options:     options,
			Digest:      BetaBrandNewDigest,
			ResultParams: map[string]string{
				types.PRIMARY_KEY_FIELD: string(data.BetaTest),
				types.CORPUS_FIELD:      "gm",
			},
		},
	}, nil)

	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	mds.On("Get", diff.PRIORITY_NOW, BetaBrandNewDigest, types.DigestSlice{data.BetaGood1Digest}).
		Return(map[types.Digest]interface{}{
			data.BetaGood1Digest: makeSmallDiffMetric(),
		}, nil)

	s := New(mds, mes, mi, nil, mcls, mtjs, everythingPublic)

	q := &query.Search{
		ChangeListID:    clID,
		DeprecatedIssue: clInt,
		NewCLStore:      true,
		IncludeMaster:   false,

		Unt:  true,
		Head: true,

		Metric:   diff.METRIC_COMBINED,
		FRGBAMin: 0,
		FRGBAMax: 255,
		FDiffMax: -1,
		Sort:     query.SortAscending,
	}

	resp, err := s.Search(context.Background(), q)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// make sure the group maps were not mutated.
	assert.Len(t, anglerGroup, 1)
	assert.Len(t, bullheadGroup, 1)
	assert.Len(t, options, 1)

	assert.Equal(t, &frontend.SearchResponse{
		Commits: data.MakeTestCommits(),
		Offset:  0,
		Size:    1,
		Digests: []*frontend.SRDigest{
			{
				Test:   data.BetaTest,
				Digest: BetaBrandNewDigest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":                {data.BullheadDevice},
					types.PRIMARY_KEY_FIELD: {string(data.BetaTest)},
					types.CORPUS_FIELD:      {"gm"},
					"ext":                   {"png"},
				},
				Traces: &frontend.TraceGroup{
					TileSize: 3,
					Traces:   []frontend.Trace{},
					Digests: []frontend.DigestStatus{
						{
							Digest: BetaBrandNewDigest,
							Status: "untriaged",
						},
					},
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						DiffMetrics: makeSmallDiffMetric(),
						Digest:      data.BetaGood1Digest,
						Status:      "positive",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.BullheadDevice},
							types.PRIMARY_KEY_FIELD: {string(data.BetaTest)},
							types.CORPUS_FIELD:      {"gm"},
							// Note: the data from three_devices lacks an "ext" entry, so
							// we don't see one here
						},
						OccurrencesInTile: 6,
					},
					common.NegativeRef: nil,
				},
			},
		},
	}, resp)
}

var everythingPublic = paramtools.ParamSet{}
var anyctx = mock.Anything

// This is arbitrary data.
func makeSmallDiffMetric() *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    8,
		PixelDiffPercent: 0.002,
		MaxRGBADiffs:     []int{0, 48, 12, 0},
		DimDiffer:        false,
		Diffs: map[string]float32{
			diff.METRIC_COMBINED: 0.0005,
			"percent":            0.002,
			"pixel":              8,
		},
	}
}

func makeBigDiffMetric() *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    88812,
		PixelDiffPercent: 0.9868,
		MaxRGBADiffs:     []int{102, 51, 13, 0},
		DimDiffer:        true,
		Diffs: map[string]float32{
			diff.METRIC_COMBINED: 4.7,
			"percent":            0.9868,
			"pixel":              88812,
		},
	}
}
