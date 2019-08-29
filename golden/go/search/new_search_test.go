package search

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/indexer"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// TODO(kjlubick): replace this with mock-based code and the tiles that are checked in testutils/data*

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

func TestSearch(t *testing.T) {
	unittest.MediumTest(t)

	api, idx, tile := getAPIIndexTile(t, gcs_testutils.TEST_DATA_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH, false)

	exp, err := api.ExpectationsStore.Get()
	assert.NoError(t, err)
	var buf bytes.Buffer

	// test basic search
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&unt=true&pos=true&neg=true&head=true", paramQuery)
	checkQuery(t, &api, idx, qStr, exp, &buf)

	// test restricting to a commit range.
	commits := tile.Commits[0 : tile.LastCommitIndex()+1]
	middle := len(commits) / 2
	beginIdx := middle - 2
	endIdx := middle + 2
	fBegin := commits[beginIdx].Hash
	fEnd := commits[endIdx].Hash

	testQueryCommitRange(t, &api, idx, tile, exp, fBegin, fEnd)
	for i := 0; i < tile.LastCommitIndex(); i++ {
		testQueryCommitRange(t, &api, idx, tile, exp, commits[i].Hash, commits[i].Hash)
	}
}

func testQueryCommitRange(t assert.TestingT, api *SearchAPI, idx indexer.IndexSearcher, tile *tiling.Tile, exp types.Expectations, startHash, endHash string) {
	var buf bytes.Buffer
	paramQuery := url.QueryEscape("source_type=gm")
	qStr := fmt.Sprintf("query=%s&fbegin=%s&fend=%s&unt=true&pos=true&neg=true&head=true", paramQuery, startHash, endHash)
	checkQuery(t, api, idx, qStr, exp, &buf)
}

func TestSearchThreeDevicesSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	mi := &mock_index.IndexSource{}
	mis := &mock_index.IndexSearcher{}
	mds := &mocks.DiffStore{}
	// defer mes.AssertExpectations(t)

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
			// This is made up, as those digests don't mean anything
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
		Sort:     "asc",
	}

	resp, err := s.Search(context.Background(), q)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	assert.Equal(t, &NewSearchResponse{
		Commits: data.MakeTestCommits(),
		Offset:  0,
		Size:    2,
		Digests: []*SRDigest{
			// AlphaTest comes first because
			{
				Test:   data.AlphaTest,
				Digest: data.AlphaUntriaged1Digest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":                {data.BullheadDevice},
					types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
					types.CORPUS_FIELD:      {"gm"},
				},
				Traces: &Traces{
					TileSize: 3, // 3 commits in tile
					Traces: []Trace{
						{
							Data: []Point{
								{X: 0, Y: 0, S: 1},
								{X: 1, Y: 0, S: 1},
								{X: 2, Y: 0, S: 0},
							},
							ID: ",device=bullhead,name=test_alpha,source_type=gm,",
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
				ClosestRef: REF_CLOSEST_POSTIVE,
				RefDiffs: map[RefClosest]*SRDiffDigest{
					REF_CLOSEST_POSTIVE: {
						DiffMetrics: makeSmallDiffMetric(),
						Test:        "", // TODO(kjlubick): why is this blank?
						Digest:      data.AlphaGood1Digest,
						Status:      "positive",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.CrosshatchDevice},
							types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						N: 2,
					},
					REF_CLOSEST_NEGATIVE: {
						DiffMetrics: makeBigDiffMetric(),
						Test:        "",
						Digest:      data.AlphaBad1Digest,
						Status:      "negative",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
							types.PRIMARY_KEY_FIELD: {string(data.AlphaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						N: 6,
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
				Traces: &Traces{
					TileSize: 3,
					Traces: []Trace{
						{
							Data: []Point{
								{X: 0, Y: 0, S: 0},
								// Other two commits were missing
							},
							ID: ",device=crosshatch,name=test_beta,source_type=gm,",
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
				ClosestRef: REF_CLOSEST_POSTIVE,
				RefDiffs: map[RefClosest]*SRDiffDigest{
					REF_CLOSEST_POSTIVE: {
						DiffMetrics: makeBigDiffMetric(),
						Test:        "",
						Digest:      data.BetaGood1Digest,
						Status:      "positive",
						ParamSet: map[string][]string{
							"device":                {data.AnglerDevice, data.BullheadDevice},
							types.PRIMARY_KEY_FIELD: {string(data.BetaTest)},
							types.CORPUS_FIELD:      {"gm"},
						},
						N: 6,
					},
					REF_CLOSEST_NEGATIVE: nil,
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
