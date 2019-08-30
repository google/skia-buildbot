package search

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
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

	s := NewSearchAPI(mds, mes, mi, nil, everythingPublic)

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
	// Negative match
	mds.On("Get", diff.PRIORITY_NOW, data.BetaUntriaged1Digest, types.DigestSlice{}).
		Return(map[types.Digest]interface{}{}, nil)

	q := &Query{
		Issue: types.MasterBranch,
		Unt:   true,
		Head:  true,

		Metric:   diff.METRIC_COMBINED,
		FRGBAMin: 0,
		FRGBAMax: 255,
		FDiffMax: -1,
		Sort:     sortAscending,
	}

	resp, err := s.Search(context.Background(), q)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	assert.Equal(t, &NewSearchResponse{
		Commits: data.MakeTestCommits(),
		Offset:  0,
		Size:    2,
		Digests: []*SRDigest{
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
				Traces: &TraceGroup{
					TileSize: 3, // 3 commits in tile
					Traces: []Trace{
						{
							Data: []Point{
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
					Digests: []DigestStatus{
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
				ClosestRef: positive,
				RefDiffs: map[RefClosest]*SRDiffDigest{
					positive: {
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
					negative: {
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
				Traces: &TraceGroup{
					TileSize: 3,
					Traces: []Trace{
						{
							Data: []Point{
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
					Digests: []DigestStatus{
						{
							Digest: data.BetaUntriaged1Digest,
							Status: "untriaged",
						},
					},
				},
				ClosestRef: positive,
				RefDiffs: map[RefClosest]*SRDiffDigest{
					positive: {
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
					negative: nil,
				},
			},
		},
	}, resp)
}

var everythingPublic = paramtools.ParamSet{}

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
