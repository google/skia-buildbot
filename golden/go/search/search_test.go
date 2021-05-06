package search

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	ttlcache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/comment"
	mock_comment "go.skia.org/infra/golden/go/comment/mocks"
	"go.skia.org/infra/golden/go/comment/trace"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
	web_frontend "go.skia.org/infra/golden/go/web/frontend"
)

// TODO(kjlubick) Add tests for:
//   - When a CL doesn't exist or the CL has not patchsets, patchset doesn't exist,
//     or otherwise no results.
//   - When a CL specifies a PS
//   - IncludeDigestsProducedOnMaster=true

// TestSearch_UntriagedDigestsAtHead_Success searches over the three_devices
// test data for untriaged images at head, essentially the default search.
// We expect to get two untriaged digests, with their closest positive and
// negative images (if any).
func TestSearch_UntriagedDigestsAtHead_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaPositiveDigest, makeSmallDiffMetric())
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaNegativeDigest, makeBigDiffMetric())
	addDiffData(t, &existingData, data.BetaUntriagedDigest, data.BetaPositiveDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	// BetaUntriagedDigest has no negative images to compare against.

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, emptyCommentStore(), everythingPublic, nothingFlaky, db)

	q := &query.Search{
		ChangelistID:                     "",
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortAscending,
	}

	resp, err := s.Search(ctx, q)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, &frontend.SearchResponse{
		Commits: web_frontend.FromTilingCommits(data.MakeTestCommits()),
		Offset:  0,
		Size:    2,
		Results: []*frontend.SearchResult{
			// AlphaTest comes first because we are sorting by ascending
			// "combined" metric, and AlphaTest's closest match is the
			// small diff metric, whereas BetaTest's only match is the
			// big diff metric.
			{
				Test:   data.AlphaTest,
				Digest: data.AlphaUntriagedDigest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":              {data.BullheadDevice},
					types.PrimaryKeyField: {string(data.AlphaTest)},
					types.CorpusField:     {"gm"},
					"ext":                 {data.PNGExtension},
				},
				TraceGroup: frontend.TraceGroup{
					TotalDigests: 2,
					Traces: []frontend.Trace{
						{
							DigestIndices: []int{1, 1, 0},
							ID:            data.BullheadAlphaTraceID,
							Params: map[string]string{
								"device":              data.BullheadDevice,
								types.PrimaryKeyField: string(data.AlphaTest),
								types.CorpusField:     "gm",
								"ext":                 data.PNGExtension,
							},
						},
					},
					Digests: []frontend.DigestStatus{
						{
							Digest: data.AlphaUntriagedDigest,
							Status: "untriaged",
						},
						{
							Digest: data.AlphaNegativeDigest,
							Status: "negative",
						},
					},
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						// Small diff
						NumDiffPixels:    8,
						PixelDiffPercent: 0.02,
						MaxRGBADiffs:     [4]int{0, 48, 12, 0},
						DimDiffer:        false,
						CombinedMetric:   0.0005,
						QueryMetric:      0.0005,
						Digest:           data.AlphaPositiveDigest,
						Status:           "positive",
						ParamSet: map[string][]string{
							"device":              {data.AnglerDevice, data.CrosshatchDevice},
							types.PrimaryKeyField: {string(data.AlphaTest)},
							types.CorpusField:     {"gm"},
							"ext":                 {data.PNGExtension},
						},
					},
					common.NegativeRef: {
						// Big Diff
						NumDiffPixels:    88812,
						PixelDiffPercent: 98.68,
						MaxRGBADiffs:     [4]int{102, 51, 13, 0},
						DimDiffer:        true,
						CombinedMetric:   4.7,
						QueryMetric:      4.7,
						Digest:           data.AlphaNegativeDigest,
						Status:           "negative",
						ParamSet: map[string][]string{
							"device":              {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
							types.PrimaryKeyField: {string(data.AlphaTest)},
							types.CorpusField:     {"gm"},
							"ext":                 {data.PNGExtension},
						},
					},
				},
			},
			{
				Test:   data.BetaTest,
				Digest: data.BetaUntriagedDigest,
				Status: "untriaged",
				ParamSet: map[string][]string{
					"device":              {data.CrosshatchDevice},
					types.PrimaryKeyField: {string(data.BetaTest)},
					types.CorpusField:     {"gm"},
					"ext":                 {data.PNGExtension},
				},
				TraceGroup: frontend.TraceGroup{
					TotalDigests: 1,
					Traces: []frontend.Trace{
						{
							DigestIndices: []int{0, MissingDigestIndex, MissingDigestIndex},
							ID:            data.CrosshatchBetaTraceID,
							Params: map[string]string{
								"device":              data.CrosshatchDevice,
								types.PrimaryKeyField: string(data.BetaTest),
								types.CorpusField:     "gm",
								"ext":                 data.PNGExtension,
							},
						},
					},
					Digests: []frontend.DigestStatus{
						{
							Digest: data.BetaUntriagedDigest,
							Status: "untriaged",
						},
					},
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						// Big Diff
						NumDiffPixels:    88812,
						PixelDiffPercent: 98.68,
						MaxRGBADiffs:     [4]int{102, 51, 13, 0},
						DimDiffer:        true,
						CombinedMetric:   4.7,
						QueryMetric:      4.7,
						Digest:           data.BetaPositiveDigest,
						Status:           "positive",
						ParamSet: paramtools.ParamSet{
							"device":              {data.AnglerDevice, data.BullheadDevice},
							types.PrimaryKeyField: {string(data.BetaTest)},
							types.CorpusField:     {"gm"},
							"ext":                 {data.PNGExtension},
						},
					},
					common.NegativeRef: nil,
				},
			},
		},
		BulkTriageData: web_frontend.TriageRequestData{
			data.AlphaTest: {
				data.AlphaUntriagedDigest: expectations.Positive,
			},
			data.BetaTest: {
				data.BetaUntriagedDigest: expectations.Positive,
			},
		},
	}, resp)
}

// TestSearch_UntriagedWithLimitAndOffset_LimitAndOffsetRespected makes a search setting the limit
// to be less than the total number of results (2) and making sure the results respect both the
// limit and offset inputs.
func TestSearch_UntriagedWithLimitAndOffset_LimitAndOffsetRespected(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaPositiveDigest, makeSmallDiffMetric())
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaNegativeDigest, makeBigDiffMetric())
	addDiffData(t, &existingData, data.BetaUntriagedDigest, data.BetaPositiveDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	// BetaUntriagedDigest has no negative images to compare against.

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, emptyCommentStore(), everythingPublic, nothingFlaky, db)

	q := &query.Search{
		ChangelistID:            "",
		IncludeUntriagedDigests: true,

		Offset: 0,
		Limit:  1,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortAscending,
	}

	resp, err := s.Search(ctx, q)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, resp.Offset, 0)
	assert.Equal(t, resp.Size, 2)
	// This checks that the returned result is the first one of the results we expect.
	assert.Equal(t, data.AlphaUntriagedDigest, resp.Results[0].Digest)
	// BulkTriageData should still be fully filled out for all digests in the full results.
	assert.Equal(t, web_frontend.TriageRequestData{
		data.AlphaTest: {
			data.AlphaUntriagedDigest: expectations.Positive,
		},
		data.BetaTest: {
			data.BetaUntriagedDigest: expectations.Positive,
		},
	}, resp.BulkTriageData)

	q.Offset = 1
	q.Limit = 100 // There's only 2 results in the total search, i.e. one remaining, so set this
	// high to make sure nothing breaks.
	resp, err = s.Search(ctx, q)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, resp.Offset, 1)
	assert.Equal(t, resp.Size, 2)
	// This checks that the returned result is the second one of the results we expect.
	assert.Equal(t, data.BetaUntriagedDigest, resp.Results[0].Digest)
	// BulkTriageData should still be fully filled out for all digests in the full results.
	assert.Equal(t, web_frontend.TriageRequestData{
		data.AlphaTest: {
			data.AlphaUntriagedDigest: expectations.Positive,
		},
		data.BetaTest: {
			data.BetaUntriagedDigest: expectations.Positive,
		},
	}, resp.BulkTriageData)
}

// TestSearchThreeDevicesQueries searches over the three_devices test data using a variety
// of queries. It only spot-checks the returned data (e.g. things are in the right order); other
// tests should do a more thorough check of the return values.
func TestSearchThreeDevicesQueries(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaPositiveDigest, makeSmallDiffMetric())
	addDiffData(t, &existingData, data.AlphaUntriagedDigest, data.AlphaNegativeDigest, makeBigDiffMetric())
	addDiffData(t, &existingData, data.BetaUntriagedDigest, data.BetaPositiveDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	// BetaUntriagedDigest has no negative images to compare against.

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, emptyCommentStore(), everythingPublic, nothingFlaky, db)

	// spotCheck is the subset of data we assert against.
	type spotCheck struct {
		test            types.TestName
		digest          types.Digest
		labelStr        expectations.Label
		closestPositive types.Digest
		closestNegative types.Digest
	}

	test := func(name string, input *query.Search, expectedOutputs []spotCheck) {
		t.Run(name, func(t *testing.T) {
			resp, err := s.Search(ctx, input)
			require.NoError(t, err)
			require.NotNil(t, resp)

			require.Len(t, resp.Results, len(expectedOutputs))
			for i, actualDigest := range resp.Results {
				expected := expectedOutputs[i]
				assert.Equal(t, expected.test, actualDigest.Test)
				assert.Equal(t, expected.digest, actualDigest.Digest)
				assert.Equal(t, expected.labelStr, actualDigest.Status)
				if expected.closestPositive == "" {
					assert.Nil(t, actualDigest.RefDiffs[common.PositiveRef])
				} else {
					cp := actualDigest.RefDiffs[common.PositiveRef]
					require.NotNil(t, cp)
					assert.Equal(t, expected.closestPositive, cp.Digest)
				}
				if expected.closestNegative == "" {
					assert.Nil(t, actualDigest.RefDiffs[common.NegativeRef])
				} else {
					cp := actualDigest.RefDiffs[common.NegativeRef]
					require.NotNil(t, cp)
					assert.Equal(t, expected.closestNegative, cp.Digest)
				}
			}
		})
	}

	test("default query, but in reverse", &query.Search{
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortDescending,
	}, []spotCheck{
		{
			test:            data.BetaTest,
			digest:          data.BetaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.BetaPositiveDigest,
			closestNegative: "",
		},
		{
			test:            data.AlphaTest,
			digest:          data.AlphaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.AlphaPositiveDigest,
			closestNegative: data.AlphaNegativeDigest,
		},
	})

	test("the closest RGBA diff should be at least 50 units away", &query.Search{
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 50,
		RGBAMaxFilter: 255,
		Sort:          query.SortDescending,
	}, []spotCheck{
		{
			test:            data.BetaTest,
			digest:          data.BetaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.BetaPositiveDigest,
			closestNegative: "",
		},
	})

	// note: this matches only the makeSmallDiffMetric
	test("the closest RGBA diff should be no more than 50 units away", &query.Search{
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 50,
		Sort:          query.SortDescending,
	}, []spotCheck{
		{
			test:            data.AlphaTest,
			digest:          data.AlphaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.AlphaPositiveDigest,
			closestNegative: data.AlphaNegativeDigest,
		},
	})

	test("default query, only those with a reference diff (all of them)", &query.Search{
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,
		MustIncludeReferenceFilter:       true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortAscending,
	}, []spotCheck{
		{
			test:            data.AlphaTest,
			digest:          data.AlphaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.AlphaPositiveDigest,
			closestNegative: data.AlphaNegativeDigest,
		},
		{
			test:            data.BetaTest,
			digest:          data.BetaUntriagedDigest,
			labelStr:        "untriaged",
			closestPositive: data.BetaPositiveDigest,
			closestNegative: "",
		},
	})

	test("query matches nothing", &query.Search{
		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,
		TraceValues: map[string][]string{
			"blubber": {"nothing"},
		},

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortDescending,
	}, []spotCheck{})
}

// TestSearch_ThreeDevicesCorpusWithComments_CommentsInResults ensures that search results contain
// comments when it matches the traces.
func TestSearch_ThreeDevicesCorpusWithComments_CommentsInResults(t *testing.T) {
	unittest.LargeTest(t)

	bullheadComment := trace.Comment{
		ID:        "1",
		CreatedBy: "zulu@example.com",
		UpdatedBy: "zulu@example.com",
		CreatedTS: time.Date(2020, time.February, 19, 18, 17, 16, 0, time.UTC),
		UpdatedTS: time.Date(2020, time.February, 19, 18, 17, 16, 0, time.UTC),
		Comment:   "All bullhead devices draw upside down",
		QueryToMatch: paramtools.ParamSet{
			"device": []string{data.BullheadDevice},
		},
	}

	alphaTestComment := trace.Comment{
		ID:        "2",
		CreatedBy: "yankee@example.com",
		UpdatedBy: "xray@example.com",
		CreatedTS: time.Date(2020, time.February, 2, 18, 17, 16, 0, time.UTC),
		UpdatedTS: time.Date(2020, time.February, 20, 18, 17, 16, 0, time.UTC),
		Comment:   "Watch pixel 0,4 to make sure it's not purple",
		QueryToMatch: paramtools.ParamSet{
			types.PrimaryKeyField: []string{string(data.AlphaTest)},
		},
	}

	betaTestBullheadComment := trace.Comment{
		ID:        "4",
		CreatedBy: "victor@example.com",
		UpdatedBy: "victor@example.com",
		CreatedTS: time.Date(2020, time.February, 22, 18, 17, 16, 0, time.UTC),
		UpdatedTS: time.Date(2020, time.February, 22, 18, 17, 16, 0, time.UTC),
		Comment:   "Being upside down, this test should be ABGR instead of RGBA",
		QueryToMatch: paramtools.ParamSet{
			"device":              []string{data.BullheadDevice},
			types.PrimaryKeyField: []string{string(data.BetaTest)},
		},
	}

	commentAppliesToNothing := trace.Comment{
		ID:        "3",
		CreatedBy: "uniform@example.com",
		UpdatedBy: "uniform@example.com",
		CreatedTS: time.Date(2020, time.February, 26, 26, 26, 26, 0, time.UTC),
		UpdatedTS: time.Date(2020, time.February, 26, 26, 26, 26, 0, time.UTC),
		Comment:   "On Wednesdays, this device draws pink",
		QueryToMatch: paramtools.ParamSet{
			"device": []string{"This device does not exist"},
		},
	}

	mcs := &mock_comment.Store{}
	// Return these in an arbitrary, unsorted order
	mcs.On("ListComments", testutils.AnyContext).Return([]trace.Comment{commentAppliesToNothing, alphaTestComment, betaTestBullheadComment, bullheadComment}, nil)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, mcs, everythingPublic, nothingFlaky, db)

	q := &query.Search{
		// Set all to true so all 6 traces show up in the final results.
		IncludeUntriagedDigests:          true,
		IncludePositiveDigests:           true,
		IncludeNegativeDigests:           true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortAscending,
	}

	resp, err := s.Search(ctx, q)
	require.NoError(t, err)
	require.NotNil(t, resp)
	// There are 4 unique digests at HEAD on the three_devices corpus. Do a quick smoke test to make
	// sure we have one search result for each of them.
	require.Len(t, resp.Results, 4)
	f := frontend.ToTraceComment
	// This should be sorted by UpdatedTS.
	assert.Equal(t, []frontend.TraceComment{
		f(bullheadComment), f(alphaTestComment), f(betaTestBullheadComment), f(commentAppliesToNothing),
	}, resp.TraceComments)

	// These numbers are indices into the resp.TraceComments. The nil entries are expected to have
	// no comments that match them.
	expectedComments := map[tiling.TraceID][]int{
		data.AnglerAlphaTraceID:     {1},
		data.AnglerBetaTraceID:      nil,
		data.BullheadAlphaTraceID:   {0, 1},
		data.BullheadBetaTraceID:    {0, 2},
		data.CrosshatchAlphaTraceID: {1},
		data.CrosshatchBetaTraceID:  nil,
	}
	// We only check that the traces have their associated comments. We rely on the other tests
	// to make sure the other fields are correct.
	traceCount := 0
	for _, r := range resp.Results {
		for _, tr := range r.TraceGroup.Traces {
			traceCount++
			assert.Equal(t, expectedComments[tr.ID], tr.CommentIndices, "trace id %q under digest", tr.ID, r.Digest)
		}
	}
	assert.Equal(t, 6, traceCount, "Not all traces were in the final result")
}

// TestSearch_ChangelistResults_ChangelistIndexMiss_Success covers the case
// where two tryjobs have been run on a given CL and PS, one on the
// angler bot and one on the bullhead bot. The master branch
// looks like in the ThreeDevices data set. The outputs produced are
// Test  |  Device  | Digest
// ----------------------
// Alpha | Angler   | data.AlphaPositiveDigest
// Alpha | Bullhead | data.AlphaUntriagedDigest
// Beta  | Angler   | data.BetaPositiveDigest
// Beta  | Bullhead | BetaBrandNewDigest
//
// The user has triaged the data.AlphaUntriagedDigest as positive
// but BetaBrandNewDigest remains untriaged.
// With this setup, we do a default query (don't show master,
// only untriaged digests) and expect to see only an entry about
// BetaBrandNewDigest.
func TestSearch_ChangelistResults_ChangelistIndexMiss_Success(t *testing.T) {
	unittest.LargeTest(t)

	const clAuthor = "broom@example.com"
	const clSubject = "big sweeping changes"
	const clID = "1234"
	const crs = "gerrit"
	const AlphaNowGoodDigest = data.AlphaUntriagedDigest
	const BetaBrandNewDigest = types.Digest("be7a03256511bec3a7453c3186bb2e07")

	// arbitrary time that our pretend CL was created at
	var clTime = time.Date(2020, time.May, 25, 10, 9, 8, 0, time.UTC)

	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}
	defer mcls.AssertExpectations(t)
	defer mtjs.AssertExpectations(t)

	mes := makeThreeDevicesExpectationStore()
	var ie expectations.Expectations
	ie.Set(data.AlphaTest, AlphaNowGoodDigest, expectations.Positive)
	issueStore := addChangelistExpectations(mes, crs, clID, &ie)
	// Hasn't been triaged yet
	issueStore.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, nil)

	mcls.On("GetChangelist", testutils.AnyContext, clID).Return(code_review.Changelist{
		SystemID: clID,
		Owner:    clAuthor,
		Status:   code_review.Open,
		Subject:  clSubject,
		Updated:  clTime,
	}, nil)
	mcls.On("GetPatchsets", testutils.AnyContext, clID).Return([]code_review.Patchset{
		{
			SystemID:     "first_one",
			ChangelistID: clID,
			Order:        1,
			// All the rest are ignored
		},
		{
			SystemID:     "fourth_one",
			ChangelistID: clID,
			Order:        4,
			// All the rest are ignored
		},
	}, nil).Once() // this should be cached after fetch, as it could be expensive to retrieve.

	expectedID := tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  "fourth_one", // we didn't specify a PS, so it goes with the most recent
	}
	anglerGroup := map[string]string{
		"device": data.AnglerDevice,
	}
	bullheadGroup := map[string]string{
		"device": data.BullheadDevice,
	}
	options := map[string]string{
		"ext": data.PNGExtension,
	}

	mtjs.On("GetResults", testutils.AnyContext, expectedID, anyTime).Return([]tjstore.TryJobResult{
		{
			GroupParams: anglerGroup,
			Options:     options,
			Digest:      data.AlphaPositiveDigest,
			ResultParams: map[string]string{
				types.PrimaryKeyField: string(data.AlphaTest),
				types.CorpusField:     "gm",
			},
		},
		{
			GroupParams: bullheadGroup,
			Options:     options,
			Digest:      AlphaNowGoodDigest,
			ResultParams: map[string]string{
				types.PrimaryKeyField: string(data.AlphaTest),
				types.CorpusField:     "gm",
			},
		},
		{
			GroupParams: anglerGroup,
			Options:     options,
			Digest:      data.BetaPositiveDigest,
			ResultParams: map[string]string{
				types.PrimaryKeyField: string(data.BetaTest),
				types.CorpusField:     "gm",
			},
		},
		{
			GroupParams: bullheadGroup,
			Options:     options,
			Digest:      BetaBrandNewDigest,
			ResultParams: map[string]string{
				types.PrimaryKeyField: string(data.BetaTest),
				types.CorpusField:     "gm",
			},
		},
	}, nil).Once() // this should be cached after fetch, as it could be expensive to retrieve.

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, BetaBrandNewDigest, data.BetaPositiveDigest, makeSmallDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	const gerritCRS = "gerrit"
	const gerritInternalCRS = "gerrit-internal"

	reviewSystems := []clstore.ReviewSystem{
		{
			ID:          gerritInternalCRS,
			Store:       nil, // nil to catch errors if this is used (when it shouldn't be)
			URLTemplate: "Should not be used",
			// Client is unused here
		},
		{
			ID:          gerritCRS,
			Store:       mcls,
			URLTemplate: "https://skia-review.googlesource.com/%s",
			// Client is unused here
		},
	}

	s := New(mes, nil, makeThreeDevicesIndexer(), reviewSystems, mtjs, nil, everythingPublic, nothingFlaky, db)

	q := &query.Search{
		CodeReviewSystemID:             gerritCRS,
		ChangelistID:                   clID,
		IncludeDigestsProducedOnMaster: false,

		IncludeUntriagedDigests:          true,
		OnlyIncludeDigestsProducedAtHead: true,

		Metric:        query.CombinedMetric,
		RGBAMinFilter: 0,
		RGBAMaxFilter: 255,
		Sort:          query.SortAscending,
	}

	resp, err := s.Search(ctx, q)
	require.NoError(t, err)
	require.NotNil(t, resp)
	// make sure the group maps were not mutated.
	assert.Len(t, anglerGroup, 1)
	assert.Len(t, bullheadGroup, 1)
	assert.Len(t, options, 1)

	// We expect to see the current CL appended to the list of master branch commits.
	masterBranchCommits := web_frontend.FromTilingCommits(data.MakeTestCommits())
	masterBranchCommitsWithCL := append(masterBranchCommits, web_frontend.Commit{
		CommitTime:    clTime.Unix(),
		Hash:          clID,
		Author:        clAuthor,
		Subject:       clSubject,
		ChangelistURL: "https://skia-review.googlesource.com/1234",
	})
	assert.Equal(t, &frontend.SearchResponse{
		Commits: masterBranchCommitsWithCL,
		Offset:  0,
		Size:    1,
		Results: []*frontend.SearchResult{
			{
				Test:   data.BetaTest,
				Digest: BetaBrandNewDigest,
				Status: "untriaged",
				ParamSet: paramtools.ParamSet{
					"device":              {data.BullheadDevice},
					types.PrimaryKeyField: {string(data.BetaTest)},
					types.CorpusField:     {"gm"},
					"ext":                 {data.PNGExtension},
				},
				TraceGroup: frontend.TraceGroup{
					Traces: []frontend.Trace{
						{
							ID: data.BullheadBetaTraceID,
							// The master branch has digest index 1 for the 3 previous commits on master branch.
							// Then we see index 0 (this digest) be added to the end to preview what the trace
							// would look like if this CL were to land.
							DigestIndices: []int{1, 1, 1, 0},
							Params: paramtools.Params{
								"device":              data.BullheadDevice,
								types.PrimaryKeyField: string(data.BetaTest),
								types.CorpusField:     "gm",
								"ext":                 data.PNGExtension,
							},
						},
					},
					Digests: []frontend.DigestStatus{
						{
							Digest: BetaBrandNewDigest,
							Status: "untriaged",
						},
						{
							Digest: data.BetaPositiveDigest,
							Status: "positive",
						},
					},
					TotalDigests: 1,
				},
				ClosestRef: common.PositiveRef,
				RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
					common.PositiveRef: {
						// Small diff
						NumDiffPixels:    8,
						PixelDiffPercent: 0.02,
						MaxRGBADiffs:     [4]int{0, 48, 12, 0},
						DimDiffer:        false,
						CombinedMetric:   0.0005,
						QueryMetric:      0.0005,
						Digest:           data.BetaPositiveDigest,
						Status:           "positive",
						ParamSet: map[string][]string{
							"device":              {data.AnglerDevice, data.BullheadDevice},
							types.PrimaryKeyField: {string(data.BetaTest)},
							types.CorpusField:     {"gm"},
							"ext":                 {data.PNGExtension},
						},
					},
					common.NegativeRef: nil,
				},
			},
		},
		BulkTriageData: web_frontend.TriageRequestData{
			data.BetaTest: {
				BetaBrandNewDigest: expectations.Positive,
			},
		},
	}, resp)

	// Validate that we cache the .*Store values in two quick responses.
	_, err = s.Search(ctx, q)
	require.NoError(t, err)
}

// TestSearchImpl_ExtractChangelistDigests_CacheHit_Success tests the case where the Changelist
// index can be used (because we are only searching for untriaged data) and makes sure the results
// are used as if they were freshly queried from firestore.
func TestSearchImpl_ExtractChangelistDigests_CacheHit_Success(t *testing.T) {
	unittest.SmallTest(t)

	const clID = "1234"
	const psID = "the_patchset"
	const crs = "gerrit"

	combinedID := tjstore.CombinedPSID{CRS: crs, CL: clID, PS: psID}

	mi := &mock_index.IndexSource{}
	mis := &mock_index.IndexSearcher{}
	mcs := &mock_clstore.Store{}

	mcs.On("GetPatchsets", testutils.AnyContext, clID).Return([]code_review.Patchset{
		{
			SystemID:     psID,
			ChangelistID: clID,
			Order:        1,
			// other fields are ignored
		},
	}, nil)

	anglerGroup := map[string]string{
		"device": data.AnglerDevice,
	}
	bullheadGroup := map[string]string{
		"device": data.BullheadDevice,
	}
	options := map[string]string{
		"ext": data.PNGExtension,
	}
	mi.On("GetIndexForCL", crs, clID).Return(&indexer.ChangelistIndex{
		LatestPatchset: combinedID,
		UntriagedResults: []tjstore.TryJobResult{
			{
				GroupParams: anglerGroup,
				Options:     options,
				Digest:      data.AlphaUntriagedDigest,
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				GroupParams: bullheadGroup,
				Options:     options,
				Digest:      data.BetaUntriagedDigest,
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     "gm",
				},
			},
		},
	})

	// No ignore rules
	mis.On("GetIgnoreMatcher").Return(paramtools.ParamMatcher{})
	// By setting a tile to be empty, we pretend that the new results do not match anything on the
	// master branch.
	emptyMasterTile := tiling.NewComplexTile(&tiling.Tile{
		Traces:   map[tiling.TraceID]*tiling.Trace{},
		ParamSet: paramtools.ParamSet{},
	})
	mis.On("Tile").Return(emptyMasterTile)

	s := SearchImpl{
		indexSource: mi,
		reviewSystems: []clstore.ReviewSystem{
			{
				ID:    crs,
				Store: mcs,
				// Client and URLTemplate are unused here
			},
		},
		tryJobStore: nil, // we should not actually hit the TryJobStore, because the cache was used.
		storeCache:  ttlcache.New(0, 0),

		clIndexCacheHitCounter:  metrics2.GetCounter("hit"),
		clIndexCacheMissCounter: metrics2.GetCounter("miss"),
	}

	q := &query.Search{
		CodeReviewSystemID:             crs,
		ChangelistID:                   clID,
		IncludeDigestsProducedOnMaster: false,

		IncludeUntriagedDigests: true,
	}

	alphaSeenCount := int32(0)
	betaSeenCount := int32(0)
	testAddFn := func(test types.TestName, digest types.Digest, _ paramtools.Params, _ tiling.TracePair) {
		if test == data.AlphaTest && digest == data.AlphaUntriagedDigest {
			atomic.AddInt32(&alphaSeenCount, 1)
		} else if test == data.BetaTest && digest == data.BetaUntriagedDigest {
			atomic.AddInt32(&betaSeenCount, 1)
		} else {
			assert.Failf(t, "unrecognized input", "%s %s", test, digest)
		}
	}

	err := s.extractChangelistDigests(context.Background(), q, mis, expectations.EmptyClassifier(), testAddFn)
	require.NoError(t, err)
	assert.Equal(t, int32(1), alphaSeenCount)
	assert.Equal(t, int32(1), betaSeenCount)
	assert.Equal(t, int64(0), s.clIndexCacheMissCounter.Get())
	assert.Equal(t, int64(1), s.clIndexCacheHitCounter.Get())

}

func TestDigestDetails_PrimaryBranch_Success(t *testing.T) {
	unittest.LargeTest(t)
	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = data.AlphaTest

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, digestWeWantDetailsAbout, data.AlphaNegativeDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, emptyCommentStore(), everythingPublic, nothingFlaky, db)

	details, err := s.GetDigestDetails(ctx, testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.NoError(t, err)
	assert.Equal(t, &frontend.DigestDetails{
		Commits: web_frontend.FromTilingCommits(data.MakeTestCommits()),
		Result: frontend.SearchResult{
			Test:   testWeWantDetailsAbout,
			Digest: digestWeWantDetailsAbout,
			Status: "positive",
			TriageHistory: []frontend.TriageHistory{
				{
					User: userWhoTriaged,
					TS:   alphaPositiveTriageTS,
				},
			},
			ParamSet: map[string][]string{
				"device":              {data.AnglerDevice, data.CrosshatchDevice},
				types.PrimaryKeyField: {string(data.AlphaTest)},
				types.CorpusField:     {"gm"},
				"ext":                 {data.PNGExtension},
			},
			TraceGroup: frontend.TraceGroup{
				TotalDigests: 2,
				Traces: []frontend.Trace{ // the digest we care about appears in two traces
					{
						DigestIndices: []int{1, 1, 0},
						ID:            data.AnglerAlphaTraceID,
						Params: map[string]string{
							"device":              data.AnglerDevice,
							types.PrimaryKeyField: string(data.AlphaTest),
							types.CorpusField:     "gm",
							"ext":                 data.PNGExtension,
						},
					},
					{
						DigestIndices: []int{1, 1, 0},
						ID:            data.CrosshatchAlphaTraceID,
						Params: map[string]string{
							"device":              data.CrosshatchDevice,
							types.PrimaryKeyField: string(data.AlphaTest),
							types.CorpusField:     "gm",
							"ext":                 data.PNGExtension,
						},
					},
				},
				Digests: []frontend.DigestStatus{
					{
						Digest: data.AlphaPositiveDigest,
						Status: "positive",
					},
					{
						Digest: data.AlphaNegativeDigest,
						Status: "negative",
					},
				},
			},
			ClosestRef: common.NegativeRef,
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: nil,
				common.NegativeRef: {
					// Big Diff
					NumDiffPixels:    88812,
					PixelDiffPercent: 98.68,
					MaxRGBADiffs:     [4]int{102, 51, 13, 0},
					DimDiffer:        true,
					CombinedMetric:   4.7,
					QueryMetric:      4.7,
					Digest:           data.AlphaNegativeDigest,
					Status:           "negative",
					ParamSet: map[string][]string{
						"device":              {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
						types.PrimaryKeyField: {string(data.AlphaTest)},
						types.CorpusField:     {"gm"},
						"ext":                 {data.PNGExtension},
					},
				},
			},
		},
	}, details)
}

func TestDigestDetails_ChangelistAltersExpectations_Success(t *testing.T) {
	unittest.LargeTest(t)

	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = data.AlphaTest
	const testCLID = "abc12345"
	const testCRS = "gerritHub"
	const clUser = "changelistUser@"
	var changelistTriageTime = time.Date(2020, time.May, 19, 18, 17, 16, 0, time.UTC)

	// Reminder, this includes triage history.
	mes := makeThreeDevicesExpectationStore()

	// Mock out some Changelist expectations in which the digest we care about is negative
	var ie expectations.Expectations
	ie.Set(testWeWantDetailsAbout, digestWeWantDetailsAbout, expectations.Negative)
	issueStore := addChangelistExpectations(mes, testCRS, testCLID, &ie)
	issueStore.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return([]expectations.TriageHistory{
		{
			User: clUser,
			TS:   changelistTriageTime,
		},
	}, nil)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, digestWeWantDetailsAbout, data.AlphaNegativeDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(mes, nil, makeThreeDevicesIndexer(), nil, nil, emptyCommentStore(), everythingPublic, nothingFlaky, db)

	details, err := s.GetDigestDetails(ctx, testWeWantDetailsAbout, digestWeWantDetailsAbout, testCLID, testCRS)
	require.NoError(t, err)
	assert.Equal(t, details.Result.Status, expectations.Negative)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: clUser,
			TS:   changelistTriageTime,
		},
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, details.Result.TriageHistory)
}

// TestDigestDetails_DigestTooOld_ReturnsComparisonToRecentDigest represents the scenario in which
// a user is requesting data about a digest that just went off the tile. We should return the
// triage status for this old digest and a comparison to digests for the same test that are current.
func TestDigestDetails_DigestTooOld_ReturnsComparisonToRecentDigest(t *testing.T) {
	unittest.LargeTest(t)

	const digestWeWantDetailsAbout = types.Digest("abcdef1234567890abcdef1234567890")
	const testWeWantDetailsAbout = data.BetaTest

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, digestWeWantDetailsAbout, data.BetaPositiveDigest, makeSmallDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	//mds := &mock_diffstore.DiffStore{}
	//addDiffData(mds, digestWeWantDetailsAbout, data.BetaPositiveDigest, makeSmallDiffMetric())

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, db)

	d, err := s.GetDigestDetails(ctx, testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.NoError(t, err)
	// spot check is fine for this test because other tests do a more thorough check of the
	// whole struct.
	assert.Equal(t, digestWeWantDetailsAbout, d.Result.Digest)
	assert.Equal(t, testWeWantDetailsAbout, d.Result.Test)
	assert.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.PositiveRef: {
			// Small diff
			NumDiffPixels:    8,
			PixelDiffPercent: 0.02,
			MaxRGBADiffs:     [4]int{0, 48, 12, 0},
			DimDiffer:        false,
			CombinedMetric:   0.0005,
			QueryMetric:      0.0005,
			Digest:           data.BetaPositiveDigest,
			Status:           "positive",
			ParamSet: paramtools.ParamSet{
				"device":              []string{data.AnglerDevice, data.BullheadDevice},
				types.PrimaryKeyField: []string{string(data.BetaTest)},
				types.CorpusField:     []string{"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
		common.NegativeRef: nil,
	}, d.Result.RefDiffs)
}

// TestDigestDetails_BadDigest_NoError represents the scenario in which a user is requesting
// data about a digest that never existed. In the past, when this has happened, it has broken
// Gold until that digest went away (e.g. because a bot only uploaded a subset of images).
// Therefore, we shouldn't error the search request, because it could break all searches for
// untriaged digests.
func TestDigestDetails_BadDigest_NoError(t *testing.T) {
	unittest.LargeTest(t)

	const digestWeWantDetailsAbout = types.Digest("abcdef1234567890abcdef1234567890")
	const testWeWantDetailsAbout = data.BetaTest

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, db)

	r, err := s.GetDigestDetails(ctx, testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.NoError(t, err)
	// Since we couldn't find the digest, we have nothing to compare against.
	assert.Equal(t, r.Result.Digest, digestWeWantDetailsAbout)
	assert.Equal(t, r.Result.ClosestRef, common.NoRef)
}

func TestDigestDetails_BadTest_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = types.TestName("invalid test")

	s := New(nil, nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, nil)

	_, err := s.GetDigestDetails(context.Background(), testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestDigestDetails_NewTestOnChangelist_Success(t *testing.T) {
	unittest.SmallTest(t)

	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = types.TestName("added_new_test")
	const latestPSID = "latestps"
	const testCLID = "abc12345"
	const testCRS = "gerritHub"

	mes := &mock_expectations.Store{}
	mis := &mock_index.IndexSource{}
	mcs := &mock_clstore.Store{}
	mts := &mock_tjstore.Store{}

	empty := expectations.Expectations{}
	mes.On("Get", testutils.AnyContext).Return(&empty, nil)
	var ie expectations.Expectations
	ie.Set(testWeWantDetailsAbout, digestWeWantDetailsAbout, expectations.Positive)
	addChangelistExpectations(mes, testCRS, testCLID, &ie)

	// This index emulates the fact that master branch does not have the newly added test.
	mis.On("GetIndex").Return(makeThreeDevicesIndex())
	mis.On("GetIndexForCL", testCRS, testCLID).Return(&indexer.ChangelistIndex{
		ParamSet: paramtools.ParamSet{
			// The index is used to verify the test exists before searching through all the TryJob
			// results for a given CL.
			types.PrimaryKeyField: []string{string(data.AlphaTest), string(data.BetaTest), string(testWeWantDetailsAbout)},
		},
	})

	mcs.On("GetPatchsets", testutils.AnyContext, testCLID).Return([]code_review.Patchset{
		{
			SystemID:     latestPSID,
			ChangelistID: testCLID,
			// Only the ID is used.
		},
	}, nil)

	// Return 4 results, 2 that match on digest and test, 1 that matches only on digest and 1 that
	// matches only on test.
	mts.On("GetResults", testutils.AnyContext, tjstore.CombinedPSID{CRS: testCRS, CL: testCLID, PS: latestPSID}, anyTime).Return([]tjstore.TryJobResult{
		{
			Digest:       digestWeWantDetailsAbout,
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(testWeWantDetailsAbout)},
			GroupParams:  paramtools.Params{"os": "Android"},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
		{
			Digest:       digestWeWantDetailsAbout,
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(testWeWantDetailsAbout)},
			GroupParams:  paramtools.Params{"os": "iOS"},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
		{
			Digest:       digestWeWantDetailsAbout,
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(data.BetaTest)},
			GroupParams:  paramtools.Params{"os": "Android"},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
		{
			Digest:       "some other digest",
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(testWeWantDetailsAbout)},
			GroupParams:  paramtools.Params{"os": "Android"},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
	}, nil)

	reviewSystems := []clstore.ReviewSystem{
		{
			ID:          testCRS,
			Store:       mcs,
			URLTemplate: "https://skia-review.googlesource.com/%s",
			// Client and URLTemplate are unused here
		},
	}

	s := New(mes, nil, mis, reviewSystems, mts, nil, everythingPublic, nothingFlaky, nil)

	rv, err := s.GetDigestDetails(context.Background(), testWeWantDetailsAbout, digestWeWantDetailsAbout, testCLID, testCRS)
	require.NoError(t, err)
	rv.Result.ParamSet.Normalize() // sort keys for determinism
	assert.Equal(t, &frontend.DigestDetails{
		Result: frontend.SearchResult{
			Test:          testWeWantDetailsAbout,
			Digest:        digestWeWantDetailsAbout,
			Status:        "positive",
			TriageHistory: nil, // TODO(skbug.com/10097)
			ParamSet: paramtools.ParamSet{
				types.PrimaryKeyField: []string{string(testWeWantDetailsAbout)},
				"os":                  []string{"Android", "iOS"},
				"ext":                 []string{data.PNGExtension},
			},
		},
	}, rv)
}

func TestDigestDetails_NewTestOnChangelist_WithPublicParams_Success(t *testing.T) {
	unittest.SmallTest(t)

	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = types.TestName("added_new_test")
	const latestPSID = "latestps"
	const testCLID = "abc12345"
	const testCRS = "gerritHub"

	mes := &mock_expectations.Store{}
	mis := &mock_index.IndexSource{}
	mcs := &mock_clstore.Store{}
	mts := &mock_tjstore.Store{}

	empty := expectations.Expectations{}
	mes.On("Get", testutils.AnyContext).Return(&empty, nil)
	var ie expectations.Expectations
	ie.Set(testWeWantDetailsAbout, digestWeWantDetailsAbout, expectations.Positive)
	addChangelistExpectations(mes, testCRS, testCLID, &ie)

	// This index emulates the fact that master branch does not have the newly added test.
	mis.On("GetIndex").Return(makeThreeDevicesIndex())
	mis.On("GetIndexForCL", testCRS, testCLID).Return(&indexer.ChangelistIndex{
		ParamSet: paramtools.ParamSet{
			// The index is used to verify the test exists before searching through all the TryJob
			// results for a given CL.
			types.PrimaryKeyField: []string{string(data.AlphaTest), string(data.BetaTest), string(testWeWantDetailsAbout)},
		},
	})

	mcs.On("GetPatchsets", testutils.AnyContext, testCLID).Return([]code_review.Patchset{
		{
			SystemID:     latestPSID,
			ChangelistID: testCLID,
			// Only the ID is used.
		},
	}, nil)

	// Return 2 results that match on digest and test, 1 of which has an OS that is not on the
	// publicly viewable list and should be filtered.
	mts.On("GetResults", testutils.AnyContext, tjstore.CombinedPSID{CRS: testCRS, CL: testCLID, PS: latestPSID}, anyTime).Return([]tjstore.TryJobResult{
		{
			Digest:       digestWeWantDetailsAbout,
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(testWeWantDetailsAbout)},
			GroupParams:  paramtools.Params{"os": "Android", types.CorpusField: data.GMCorpus},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
		{
			Digest:       digestWeWantDetailsAbout,
			ResultParams: paramtools.Params{types.PrimaryKeyField: string(testWeWantDetailsAbout)},
			GroupParams:  paramtools.Params{"os": "super_secret_device_do_not_leak", types.CorpusField: data.GMCorpus},
			Options:      paramtools.Params{"ext": data.PNGExtension},
		},
	}, nil)

	publicMatcher, err := publicparams.MatcherFromRules(publicparams.MatchingRules{
		"gm": {
			"os": {"Android"},
		},
	})
	require.NoError(t, err)

	reviewSystems := []clstore.ReviewSystem{
		{
			ID:          testCRS,
			Store:       mcs,
			URLTemplate: "https://skia-review.googlesource.com/%s",
			// Client and URLTemplate are unused here
		},
	}
	s := New(mes, nil, mis, reviewSystems, mts, nil, publicMatcher, nothingFlaky, nil)

	rv, err := s.GetDigestDetails(context.Background(), testWeWantDetailsAbout, digestWeWantDetailsAbout, testCLID, testCRS)
	require.NoError(t, err)
	rv.Result.ParamSet.Normalize() // sort keys for determinism
	assert.Equal(t, &frontend.DigestDetails{
		Result: frontend.SearchResult{
			Test:          testWeWantDetailsAbout,
			Digest:        digestWeWantDetailsAbout,
			Status:        "positive",
			TriageHistory: nil, // TODO(skbug.com/10097)
			ParamSet: paramtools.ParamSet{
				types.PrimaryKeyField: []string{string(testWeWantDetailsAbout)},
				types.CorpusField:     []string{data.GMCorpus},
				"os":                  []string{"Android"},
				"ext":                 []string{data.PNGExtension},
			},
		},
	}, rv)
}

func TestDigestDetails_BadTestAndDigest_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	const digestWeWantDetailsAbout = types.Digest("invalid digest")
	const testWeWantDetailsAbout = types.TestName("invalid test")

	s := New(nil, nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, nil)

	_, err := s.GetDigestDetails(context.Background(), testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestDigestDetails_TestIgnored_DetailsContainResults_Success(t *testing.T) {
	unittest.LargeTest(t)

	const digestWeWantDetailsAbout = data.AlphaPositiveDigest
	const testWeWantDetailsAbout = data.AlphaTest

	mi := makeIndexWithIgnoreRules(t, "name=test_alpha")

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, digestWeWantDetailsAbout, data.AlphaNegativeDigest, makeBigDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(makeThreeDevicesExpectationStore(), nil, mi, nil, nil, nil, everythingPublic, nothingFlaky, db)

	result, err := s.GetDigestDetails(ctx, testWeWantDetailsAbout, digestWeWantDetailsAbout, "", "")
	require.NoError(t, err)
	assert.Equal(t, paramtools.ParamSet{
		"device":              {data.AnglerDevice, data.CrosshatchDevice},
		types.PrimaryKeyField: {string(data.AlphaTest)},
		types.CorpusField:     {"gm"},
		"ext":                 {data.PNGExtension},
	}, result.Result.ParamSet)
	assert.Equal(t, map[common.RefClosest]*frontend.SRDiffDigest{
		common.NegativeRef: {
			// Big Diff
			NumDiffPixels:    88812,
			PixelDiffPercent: 98.68,
			MaxRGBADiffs:     [4]int{102, 51, 13, 0},
			DimDiffer:        true,
			CombinedMetric:   4.7,
			QueryMetric:      4.7,
			Digest:           data.AlphaNegativeDigest,
			Status:           expectations.Negative,
			ParamSet: paramtools.ParamSet{
				"device":              {data.AnglerDevice, data.BullheadDevice, data.CrosshatchDevice},
				types.PrimaryKeyField: {string(data.AlphaTest)},
				types.CorpusField:     {"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
		common.PositiveRef: nil,
	}, result.Result.RefDiffs)
}

func TestDiffDigestsSunnyDay(t *testing.T) {
	unittest.LargeTest(t)

	const testWeWantDetailsAbout = data.AlphaTest
	const leftDigest = data.AlphaUntriagedDigest
	const rightDigest = data.AlphaPositiveDigest

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, leftDigest, rightDigest, makeSmallDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, db)

	cd, err := s.DiffDigests(ctx, testWeWantDetailsAbout, leftDigest, rightDigest, "", "")
	require.NoError(t, err)
	assert.Equal(t, &frontend.DigestComparison{
		Left: frontend.SearchResult{
			Test:   testWeWantDetailsAbout,
			Digest: leftDigest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				"device":              []string{data.BullheadDevice},
				types.PrimaryKeyField: []string{string(data.AlphaTest)},
				types.CorpusField:     []string{"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
		Right: &frontend.SRDiffDigest{
			Digest: rightDigest,
			Status: expectations.Positive,
			// Small diff
			NumDiffPixels:    8,
			PixelDiffPercent: 0.02,
			MaxRGBADiffs:     [4]int{0, 48, 12, 0},
			DimDiffer:        false,
			CombinedMetric:   0.0005,
			ParamSet: paramtools.ParamSet{
				"device":              []string{data.AnglerDevice, data.CrosshatchDevice},
				types.PrimaryKeyField: []string{string(data.AlphaTest)},
				types.CorpusField:     []string{"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
	}, cd)
}

func TestDiffDigests_UseSQLDiffMetrics_DiffExists_Success(t *testing.T) {
	unittest.LargeTest(t)

	const testWeWantDetailsAbout = data.AlphaTest
	const leftDigest = data.AlphaUntriagedDigest
	const rightDigest = data.AlphaPositiveDigest
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{DiffMetrics: []schema.DiffMetricRow{{
		LeftDigest:        d(t, leftDigest),
		RightDigest:       d(t, rightDigest),
		NumPixelsDiff:     13,
		PercentPixelsDiff: 0.5,
		MaxRGBADiffs:      [4]int{8, 9, 10, 11},
		MaxChannelDiff:    12,
		CombinedMetric:    4.2,
		DimensionsDiffer:  true,
		Timestamp:         time.Date(2021, time.February, 4, 3, 2, 1, 0, time.UTC),
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(makeThreeDevicesExpectationStore(), nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, db)

	cd, err := s.DiffDigests(ctx, testWeWantDetailsAbout, leftDigest, rightDigest, "", "")
	require.NoError(t, err)
	assert.Equal(t, &frontend.DigestComparison{
		Left: frontend.SearchResult{
			Test:   testWeWantDetailsAbout,
			Digest: leftDigest,
			Status: expectations.Untriaged,
			ParamSet: paramtools.ParamSet{
				"device":              []string{data.BullheadDevice},
				types.PrimaryKeyField: []string{string(data.AlphaTest)},
				types.CorpusField:     []string{"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
		Right: &frontend.SRDiffDigest{
			Digest: rightDigest,
			Status: expectations.Positive,
			// From data injected into SQL data
			NumDiffPixels:    13,
			PixelDiffPercent: 0.5,
			MaxRGBADiffs:     [4]int{8, 9, 10, 11},
			DimDiffer:        true,
			CombinedMetric:   4.2,
			ParamSet: paramtools.ParamSet{
				"device":              []string{data.AnglerDevice, data.CrosshatchDevice},
				types.PrimaryKeyField: []string{string(data.AlphaTest)},
				types.CorpusField:     []string{"gm"},
				"ext":                 {data.PNGExtension},
			},
		},
	}, cd)
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

func d(t *testing.T, digest types.Digest) schema.DigestBytes {
	db, err := sql.DigestToBytes(digest)
	require.NoError(t, err)
	return db
}

func TestDiffDigests_UseSQLDiffMetrics_DiffDoesNotExists_ReturnsError(t *testing.T) {
	unittest.LargeTest(t)

	const testWeWantDetailsAbout = data.AlphaTest
	const leftDigest = data.AlphaUntriagedDigest
	const rightDigest = data.AlphaPositiveDigest
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	s := New(nil, nil, nil, nil, nil, nil, everythingPublic, nothingFlaky, db)

	_, err := s.DiffDigests(ctx, testWeWantDetailsAbout, leftDigest, rightDigest, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "diff not found for 222208f09d37b73795649038408b5f33-000075b9c0f1b6a831c399e269772661")
}

func TestDiffDigestsChangelist(t *testing.T) {
	unittest.LargeTest(t)

	const testWeWantDetailsAbout = data.AlphaTest
	const leftDigest = data.AlphaUntriagedDigest
	const rightDigest = data.AlphaPositiveDigest
	const clID = "abc12354"
	const crs = "gerritHub"

	mes := makeThreeDevicesExpectationStore()
	var ie expectations.Expectations
	ie.Set(data.AlphaTest, leftDigest, expectations.Negative)
	issueStore := addChangelistExpectations(mes, crs, clID, &ie)
	issueStore.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, nil)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{}
	addDiffData(t, &existingData, leftDigest, rightDigest, makeSmallDiffMetric())
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	s := New(mes, nil, makeThreeDevicesIndexer(), nil, nil, nil, everythingPublic, nothingFlaky, db)

	cd, err := s.DiffDigests(context.Background(), testWeWantDetailsAbout, leftDigest, rightDigest, clID, crs)
	require.NoError(t, err)
	assert.Equal(t, cd.Left.Status, expectations.Negative)
}

// TestUntriagedUnignoredTryJobExclusiveDigests_NoIndexBuilt_Error is the case where an index was
// not built. As such, we do not recompute this (it is too expensive), so we return an error.
func TestUntriagedUnignoredTryJobExclusiveDigests_NoIndexBuilt_Error(t *testing.T) {
	unittest.SmallTest(t)

	const clID = "44474"
	const crs = "github"
	expectedID := tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  "abcdef",
	}

	mi := &mock_index.IndexSource{}
	mi.On("GetIndexForCL", crs, clID).Return(nil)

	s := New(nil, nil, mi, nil, nil, nil, everythingPublic, nothingFlaky, nil)

	_, err := s.UntriagedUnignoredTryJobExclusiveDigests(context.Background(), expectedID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not indexed")
}

// This test shows the case where a "flaky" trace is ignored because it (and only it) has a number
// of unique digests above the provided threshold.
func TestUntriagedUnignoredTryJobExclusiveDigests_LowFlakyTraceThreshold_FlakyTracesFilteredOut(t *testing.T) {
	unittest.SmallTest(t)

	const clID = "44474"
	const crs = "github"
	expectedID := tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  "abcdef",
	}
	indexedTS := time.Date(2020, time.September, 10, 8, 6, 4, 0, time.UTC)

	const alphaUntriagedTryJobDigest = types.Digest("aaaa65e567de97c8a62918401731c7ec")
	const betaUntriagedTryJobDigest = types.Digest("bbbb34f7c915a1ac3a5ba524c741946c")

	mi := &mock_index.IndexSource{}

	mes := makeThreeDevicesExpectationStore()
	var ie expectations.Expectations
	addChangelistExpectations(mes, crs, clID, &ie)

	anglerGroup := map[string]string{
		"device": data.AnglerDevice,
	}
	options := map[string]string{
		"ext": data.PNGExtension,
	}

	fis := makeThreeDevicesIndex()
	mi.On("GetIndex").Return(fis)
	mi.On("GetIndexForCL", crs, clID).Return(&indexer.ChangelistIndex{
		LatestPatchset: expectedID,
		ParamSet:       nil, // not used here
		ComputedTS:     indexedTS,
		UntriagedResults: []tjstore.TryJobResult{
			{
				// This trace is "flaky", so should not be reported. Specifically, AnglerAlphaTraceID
				// has 2 digests in it on master, AlphaNegativeDigest and AlphaPositiveDigest.
				GroupParams: anglerGroup,
				Options:     options,
				Digest:      alphaUntriagedTryJobDigest,
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				// This trace is not flaky and should be reported because there is only one digest seen
				// on the master branch BetaPositiveDigest.
				GroupParams: anglerGroup,
				Options:     options,
				Digest:      betaUntriagedTryJobDigest,
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     "gm",
				},
			},
		},
	})

	s := New(mes, nil, mi, nil, nil, nil, everythingPublic, 1, nil)

	dl, err := s.UntriagedUnignoredTryJobExclusiveDigests(context.Background(), expectedID)
	require.NoError(t, err)
	assert.Equal(t, []string{"gm"}, dl.Corpora)
	assert.Equal(t, []types.Digest{betaUntriagedTryJobDigest}, dl.Digests)
	// TS should be very recent, since the results were freshly computed.
	assert.Equal(t, indexedTS, dl.TS)
}

// TestUntriagedUnignoredTryJobExclusiveDigests_UsesIndex_Success is the same as the previous test,
// except it uses the pre-built index, which only returns the untriaged digests (that then need
// to be filtered to check for ignore rules and those seen on master).
func TestUntriagedUnignoredTryJobExclusiveDigests_UsesIndex_Success(t *testing.T) {
	unittest.SmallTest(t)

	const clID = "44474"
	const crs = "github"
	expectedID := tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  "abcdef",
	}

	const alphaUntriagedTryJobDigest = types.Digest("aaaa65e567de97c8a62918401731c7ec")
	const betaUntriagedTryJobDigest = types.Digest("bbbb34f7c915a1ac3a5ba524c741946c")
	const gammaNegativeTryJobDigest = types.Digest("cccc41bf4584e51be99e423707157277")
	const deltaIgnoredTryJobDigest = types.Digest("dddd84e51be99e42370715727765e563")

	mi := &mock_index.IndexSource{}

	// Set up the expectations such that for this CL, we have one extra expectation - marking
	// gammaNegativeTryJobDigest negative (it would be untriaged on master).
	mes := makeThreeDevicesExpectationStore()
	var ie expectations.Expectations
	ie.Set(data.AlphaTest, gammaNegativeTryJobDigest, expectations.Negative)
	addChangelistExpectations(mes, crs, clID, &ie)

	cpxTile := tiling.NewComplexTile(data.MakeTestTile())
	reduced := data.MakeTestTile()
	delete(reduced.Traces, data.BullheadBetaTraceID)
	// The following rule exclusively matches BullheadBetaTraceID, for which the tryjob produced
	// deltaIgnoredTryJobDigest
	cpxTile.SetIgnoreRules(reduced, paramtools.ParamMatcher{
		{
			"device":              []string{data.BullheadDevice},
			types.PrimaryKeyField: []string{string(data.BetaTest)},
		},
	})
	dc := digest_counter.New(data.MakeTestTile())
	fis, err := indexer.SearchIndexForTesting(cpxTile, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{}, mes, nil)
	require.NoError(t, err)
	mi.On("GetIndex").Return(fis)

	anglerGroup := map[string]string{
		"device": data.AnglerDevice,
	}
	bullheadGroup := map[string]string{
		"device": data.BullheadDevice,
	}
	crosshatchGroup := map[string]string{
		"device": data.CrosshatchDevice,
	}
	options := map[string]string{
		"ext": data.PNGExtension,
	}
	indexTS := time.Date(2020, time.May, 1, 2, 3, 4, 0, time.UTC)
	mi.On("GetIndexForCL", crs, clID).Return(&indexer.ChangelistIndex{
		ComputedTS:     indexTS,
		LatestPatchset: expectedID,
		UntriagedResults: []tjstore.TryJobResult{
			{
				GroupParams: anglerGroup,
				Options:     options,
				Digest:      betaUntriagedTryJobDigest, // should be reported.
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				GroupParams: bullheadGroup,
				Options:     options,
				Digest:      data.AlphaUntriagedDigest, // already seen on master as untriaged.
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				GroupParams: anglerGroup,
				Options:     options,
				Digest:      alphaUntriagedTryJobDigest, // should be reported.
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				GroupParams: bullheadGroup,
				Options:     options,
				Digest:      deltaIgnoredTryJobDigest, // matches an ignore rule; should be filtered.
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.BetaTest),
					types.CorpusField:     "gm",
				},
			},
			{
				GroupParams: crosshatchGroup,
				Options:     options,
				Digest:      gammaNegativeTryJobDigest, // already triaged as negative; should be filtered.
				ResultParams: map[string]string{
					types.PrimaryKeyField: string(data.AlphaTest),
					types.CorpusField:     "gm",
				},
			},
		},
	})

	s := New(mes, nil, mi, nil, nil, nil, everythingPublic, nothingFlaky, nil)

	dl, err := s.UntriagedUnignoredTryJobExclusiveDigests(context.Background(), expectedID)
	require.NoError(t, err)
	assert.Equal(t, &frontend.UntriagedDigestList{
		Digests: []types.Digest{alphaUntriagedTryJobDigest, betaUntriagedTryJobDigest},
		Corpora: []string{"gm"},
		TS:      indexTS,
	}, dl)
}

// TestFillInFrontEndTraceData_SingleTrace_DigestIndicesAreCorrect tests that we generate the output
// required to draw the trace graphs correctly, especially when dealing with many digests or
// missing digests.
func TestFillInFrontEndTraceData_SingleTrace_DigestIndicesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	// Add some shorthand aliases for easier-to-read test inputs.
	const mm = tiling.MissingDigest
	const mdi = MissingDigestIndex
	// These constants are not actual md5 digests, but that's ok for the purposes of this test -
	// any string constants will do.
	const d0, d1, d2, d3, d4 = types.Digest("d0"), types.Digest("d1"), types.Digest("d2"), types.Digest("d3"), types.Digest("d4")

	test := func(desc string, inputDigests []types.Digest, expectedData []int) {
		require.Equal(t, len(inputDigests), len(expectedData))
		// stubClassifier returns Positive for everything. For the purposes of drawing traces,
		// don't actually care about the expectations.
		stubClassifier := &mock_expectations.Classifier{}
		stubClassifier.On("Classification", mock.Anything, mock.Anything).Return(expectations.Positive)
		t.Run(desc, func(t *testing.T) {
			traces := []frontend.Trace{
				{
					ID: "not-a-real-trace-id-and-that's-ok",
					RawTrace: &tiling.Trace{
						Digests: inputDigests,
						// Keys can be omitted because they are not read here.
					},
					// Other fields don't matter for this test.
				},
			}
			tg := frontend.TraceGroup{Traces: traces}

			fillInFrontEndTraceData(&tg, "whatever", d0, stubClassifier, nil, false /* = appendPrimaryDigest*/)
			require.Len(t, tg.Traces, 1)
			assert.Equal(t, expectedData, tg.Traces[0].DigestIndices)
		})
	}

	test("several distinct digests",
		[]types.Digest{d4, d3, d2, d1, d0},
		[]int{4, 3, 2, 1, 0})
	// index 1 represents the first digest, starting at head, that doesn't match the "digest of
	// focus", which for these tests is d0. For convenience, in all the other sub-tests, the index
	// on the constants matches the expected index.
	test("several distinct digests, ordered by proximity to head",
		[]types.Digest{d1, d2, d3, d4, d0},
		[]int{4, 3, 2, 1, 0})
	test("missing digests",
		[]types.Digest{mm, d1, mm, d0, mm},
		[]int{mdi, 1, mdi, 0, mdi})
	test("multiple missing digest in a row",
		[]types.Digest{mm, mm, mm, d1, d1, mm, mm, mm, d0, mm, mm},
		[]int{mdi, mdi, mdi, 1, 1, mdi, mdi, mdi, 0, mdi, mdi})
	test("all the same",
		[]types.Digest{d0, d0, d0, d0, d0, d0, d0},
		[]int{0, 0, 0, 0, 0, 0, 0})
	test("d0 not at head",
		[]types.Digest{d0, d0, d0, d1, d2, d1},
		[]int{0, 0, 0, 1, 2, 1})
	// At a certain point, we lump distinct digests together. Currently the 10th distinct digests
	// and beyond are lumped in with the 9th (starting at head).
	test("too many distinct digests",
		[]types.Digest{"dA", "d9", "d8", "d7", "d6", "d5", d4, d3, d2, d1, d0},
		[]int{8, 8, 8, 7, 6, 5, 4, 3, 2, 1, 0})
	test("Exactly MaxDistinctDigestsToPresent digests",
		[]types.Digest{"d8", "d7", "d6", "d5", d4, d3, d2, d1, d0},
		[]int{8, 7, 6, 5, 4, 3, 2, 1, 0})

	// index 0 is always the primary digest. Indices [1, 3] are for the digests seen closest to head.
	// Indices [4, 7] are the 4 most common digests (ties are broken by closest to head).
	// Index 8 is everything else.
	test("prioritizes most used digests",
		[]types.Digest{"dC", "dC", "dC", "dB", "dB", "dA", "d9", "d8", "d7", "d6", "d6", "d5", d4, d3, d2, d1, d0},
		[]int{4, 4, 4, 6, 6, 8, 8, 8, 8, 5, 5, 8, 7, 3, 2, 1, 0})
}

func TestFillInFrontEndTraceData_MultipleTraces_DigestIndicesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	// Add some shorthand aliases for easier-to-read test inputs.
	const mm = tiling.MissingDigest
	const mdi = MissingDigestIndex

	test := func(desc string, traceOneDigests, traceTwoDigests []types.Digest, traceOneIndices, traceTwoIndices []int) {
		require.Equal(t, len(traceTwoDigests), len(traceOneDigests))
		require.Equal(t, len(traceOneIndices), len(traceTwoIndices))
		require.Equal(t, len(traceOneDigests), len(traceOneIndices))
		// stubClassifier returns Positive for everything. For the purposes of drawing traces,
		// don't actually care about the expectations.
		stubClassifier := &mock_expectations.Classifier{}
		stubClassifier.On("Classification", mock.Anything, mock.Anything).Return(expectations.Positive)
		t.Run(desc, func(t *testing.T) {
			traces := []frontend.Trace{
				{
					ID: "not-a-real-trace-id-and-that's-ok",
					RawTrace: &tiling.Trace{
						Digests: traceOneDigests,
						// Keys can be omitted because they are not read here.
					},
					// Other fields don't matter for this test.
				},
				{
					ID: "still-not-a-real-trace-id-and-that's-ok",
					RawTrace: &tiling.Trace{
						Digests: traceTwoDigests,
						// Keys can be omitted because they are not read here.
					},
					// Other fields don't matter for this test.
				},
			}
			tg := frontend.TraceGroup{Traces: traces}

			fillInFrontEndTraceData(&tg, "whatever", "d0", stubClassifier, nil, false /* = appendPrimaryDigest*/)
			require.Len(t, tg.Traces, 2)
			assert.Equal(t, traceOneIndices, tg.Traces[0].DigestIndices, "traceOne")
			assert.Equal(t, traceTwoIndices, tg.Traces[1].DigestIndices, "traceTwo")
		})
	}

	// Of note, "d0" is the primary digest for all of these tests.

	test("several distinct digests",
		[]types.Digest{mm, "d3", "d1", "d1", "d0"},
		[]types.Digest{"d4", mm, "d2", "d2", "d0"},
		[]int{mdi, 3, 1, 1, 0},
		[]int{4, mdi, 2, 2, 0},
	)

	test("was stable, but bug made tests go flaky",
		[]types.Digest{"dS", "dS", mm, "dA", "d8", "d6", "d3", "d2", "d1", "d0"},
		[]types.Digest{"dS", "dS", "dB", mm, "d7", "d5", "d4", "d1", "d2", "d0"},
		[]int{4, 4, -1, 8, 8, 7, 3, 2, 1, 0},
		[]int{4, 4, 8, -1, 8, 6, 5, 1, 2, 0},
	)

	test("primary digest is stable, but gets flaky",
		[]types.Digest{"d0", "d0", mm, "dA", "d8", "d6", "d5", "d2", "d1", "dF"},
		[]types.Digest{"d0", "d0", "dB", mm, "d7", "d5", "d4", "d1", "d0", "dE"},
		[]int{0, 0, -1, 8, 8, 7, 4, 5, 3, 2},
		[]int{0, 0, 8, -1, 8, 4, 6, 3, 0, 1},
	)
}

// TestFillInFrontEndTraceData_AppendPrimaryDigest_DigestIndicesAreCorrect tests that we generate
// the output required to draw the trace graphs correctly, specifically the case for Changelist
// results (that is, when appendPrimaryDigest is true).
func TestFillInFrontEndTraceData_AppendPrimaryDigest_DigestIndicesAreCorrect(t *testing.T) {
	unittest.SmallTest(t)
	// Add some shorthand aliases for easier-to-read test inputs.
	const mm = tiling.MissingDigest
	const mdi = MissingDigestIndex
	// These constants are not actual md5 digests, but that's ok for the purposes of this test -
	// any string constants will do.
	const d0, d1, d2, d3, d4 = types.Digest("d0"), types.Digest("d1"), types.Digest("d2"), types.Digest("d3"), types.Digest("d4")

	test := func(desc string, inputDigests []types.Digest, expectedData []int) {
		// stubClassifier returns Positive for everything. For the purposes of drawing traces,
		// don't actually care about the expectations.
		stubClassifier := &mock_expectations.Classifier{}
		stubClassifier.On("Classification", mock.Anything, mock.Anything).Return(expectations.Positive)
		t.Run(desc, func(t *testing.T) {
			traces := []frontend.Trace{
				{
					ID: "not-a-real-trace-id-and-that's-ok",
					RawTrace: &tiling.Trace{
						Digests: inputDigests,
						// Keys can be omitted because they are not read here.
					},
					// Other fields don't matter for this test.
				},
			}
			tg := frontend.TraceGroup{Traces: traces}

			fillInFrontEndTraceData(&tg, "whatever", d0, stubClassifier, nil, true /* = appendPrimaryDigest*/)
			require.Len(t, tg.Traces, 1)
			assert.Equal(t, expectedData, tg.Traces[0].DigestIndices)
		})
	}

	test("several distinct digests",
		[]types.Digest{d4, d3, d2, d1, d1},
		[]int{4, 3, 2, 1, 1, 0})
	// index 1 represents the first digest, starting at head, that doesn't match the "digest of
	// focus", which for these tests is d0. For convenience, in all the other sub-tests, the index
	// on the constants matches the expected index.
	test("several distinct digests, ordered by proximity to head",
		[]types.Digest{d1, d2, d3, d4, d4},
		[]int{4, 3, 2, 1, 1, 0})
	test("missing digests",
		[]types.Digest{mm, d1, mm, d1, mm},
		[]int{mdi, 1, mdi, 1, mdi, 0})
}

// TestFillInFrontEndTraceData_TotalDigestsCorrect tests that we count unique digests for a
// TraceGroup correctly, even when there are multiple traces or the number of digests is bigger than
// MaxDistinctDigestsToPresent.
func TestFillInFrontEndTraceData_TotalDigestsCorrect(t *testing.T) {
	unittest.SmallTest(t)
	// Add some shorthand aliases for easier-to-read test inputs.
	const md = tiling.MissingDigest
	// This constant is not an actual md5 digest, but that's ok for the purposes of this test -
	// any string constants will do.
	const d0 = types.Digest("d0")

	test := func(desc string, totalUniqueDigests int, inputTraceDigests ...[]types.Digest) {
		// stubClassifier returns Positive for everything. For the purposes of counting digests,
		// don't actually care about the expectations.
		stubClassifier := &mock_expectations.Classifier{}
		stubClassifier.On("Classification", mock.Anything, mock.Anything).Return(expectations.Positive)
		t.Run(desc, func(t *testing.T) {
			traces := make([]frontend.Trace, 0, len(inputTraceDigests))
			for i, digests := range inputTraceDigests {
				id := tiling.TraceID(fmt.Sprintf("trace-%d", i))
				traces = append(traces, frontend.Trace{
					ID: id,
					RawTrace: &tiling.Trace{
						Digests: digests,
						// Keys can be omitted because they are not read here.
					},
					// Other fields don't matter for this test.
				})
			}
			tg := frontend.TraceGroup{Traces: traces}
			fillInFrontEndTraceData(&tg, "whatever", d0, stubClassifier, nil, false /* = appendPrimaryDigest*/)
			require.Len(t, tg.Traces, len(inputTraceDigests))
			assert.Equal(t, totalUniqueDigests, tg.TotalDigests)
		})
	}
	test("one distinct digest, repeated multiple times",
		1,
		[]types.Digest{d0, d0, d0})
	test("several distinct digests",
		5,
		[]types.Digest{"d4", "d3", "d2", "d1", d0})
	test("does not count missing digests",
		5,
		[]types.Digest{"d4", "d3", "d2", "d1", md, d0},
		[]types.Digest{md, md, md, md, d0, md})
	test("accounts for distinct digests across traces",
		13,
		[]types.Digest{"d6", "d5", "d4", "d3", "d2", "d1", d0},
		[]types.Digest{"dF", "dE", "dD", "dC", "dB", "dA", d0})
	test("one long trace",
		11,
		[]types.Digest{"dA", "d9", "d8", "d7", "d6", "d5", "d4", "d3", "d2", "d1", d0})
}

func TestAddExpectations_Success(t *testing.T) {
	unittest.SmallTest(t)

	results := []*frontend.SearchResult{
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaPositiveDigest,
		},
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaNegativeDigest,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaPositiveDigest,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaUntriagedDigest,
		},
	}
	addExpectations(results, data.MakeTestExpectations())

	assert.ElementsMatch(t, []*frontend.SearchResult{
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaPositiveDigest,
			Status: expectations.Positive,
		},
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaNegativeDigest,
			Status: expectations.Negative,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaPositiveDigest,
			Status: expectations.Positive,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaUntriagedDigest,
			Status: expectations.Untriaged,
		},
	}, results)
}

func TestAddTriageHistory_HistoryExistsForAllEntries_Success(t *testing.T) {
	unittest.SmallTest(t)
	mes := makeThreeDevicesExpectationStore()
	s := New(nil, nil, nil, nil, nil, nil, nil, nothingFlaky, nil)

	input := []*frontend.SearchResult{
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaPositiveDigest,
			// The rest of the fields don't matter for this test.
		},
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaNegativeDigest,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaPositiveDigest,
		},
	}
	s.addTriageHistory(context.Background(), mes, input)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, input[0].TriageHistory)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaNegativeTriageTS,
		},
	}, input[1].TriageHistory)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   betaPositiveTriageTS,
		},
	}, input[2].TriageHistory)
}

func TestAddTriageHistory_EmptyTriageHistory_Success(t *testing.T) {
	unittest.SmallTest(t)
	mes := &mock_expectations.Store{}
	mes.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, nil)
	s := New(nil, nil, nil, nil, nil, nil, nil, nothingFlaky, nil)

	input := []*frontend.SearchResult{
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaPositiveDigest,
			// The rest of the fields don't matter for this test.
		},
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaNegativeDigest,
		},
		{
			Test:   data.BetaTest,
			Digest: data.BetaPositiveDigest,
		},
	}
	s.addTriageHistory(context.Background(), mes, input)
	assert.Nil(t, input[0].TriageHistory)
	assert.Nil(t, input[1].TriageHistory)
	assert.Nil(t, input[2].TriageHistory)
}

func TestAddTriageHistory_ExpectationStoreError_ReturnedTriageHistoryIsEmpty(t *testing.T) {
	unittest.SmallTest(t)
	mes := &mock_expectations.Store{}
	mes.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("kaboom"))
	s := New(nil, nil, nil, nil, nil, nil, nil, nothingFlaky, nil)

	input := []*frontend.SearchResult{
		{
			Test:   data.AlphaTest,
			Digest: data.AlphaPositiveDigest,
			// The rest of the fields don't matter for this test.
		},
	}
	s.addTriageHistory(context.Background(), mes, input)
	assert.Nil(t, input[0].TriageHistory)
}

func TestGetTriageHistory_CachesResults_CallsGetTriageHistoryOncePerEntry(t *testing.T) {
	unittest.SmallTest(t)
	mes := &mock_expectations.Store{}
	mes.On("GetTriageHistory", testutils.AnyContext, data.AlphaTest, data.AlphaPositiveDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, nil).Once()
	mes.On("GetTriageHistory", testutils.AnyContext, data.BetaTest, data.BetaPositiveDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   betaPositiveTriageTS,
		},
	}, nil).Once()

	s := New(nil, nil, nil, nil, nil, nil, nil, nothingFlaky, nil)
	trBeta := s.getTriageHistory(context.Background(), mes, data.BetaTest, data.BetaPositiveDigest)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   betaPositiveTriageTS,
		},
	}, trBeta)
	trAlpha := s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, trAlpha)

	// This should be from the cache - the .Once() on the mocks ensures that.
	tr := s.getTriageHistory(context.Background(), mes, data.BetaTest, data.BetaPositiveDigest)
	assert.Equal(t, trBeta, tr)
	tr = s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Equal(t, trAlpha, tr)
}

func TestGetTriageHistory_CacheClearedWhenNotified(t *testing.T) {
	unittest.SmallTest(t)
	notifier := expectations.NewEventDispatcherForTesting()
	mes := &mock_expectations.Store{}
	mes.On("GetTriageHistory", testutils.AnyContext, data.AlphaTest, data.AlphaPositiveDigest).Return(nil, nil).Once()

	s := New(nil, notifier, nil, nil, nil, nil, nil, nothingFlaky, nil)

	// The first call to history is empty.
	tr := s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Empty(t, tr)
	// Notify something that does not match our entry.
	notifier.NotifyChange(expectations.ID{
		Grouping: data.AlphaTest,
		Digest:   "for a completely different digest",
	})

	// The empty value should still be cached.
	tr = s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Empty(t, tr)

	mes.On("GetTriageHistory", testutils.AnyContext, data.AlphaTest, data.AlphaPositiveDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, nil).Once()
	notifier.NotifyChange(expectations.ID{
		Grouping: data.AlphaTest,
		Digest:   data.AlphaPositiveDigest,
	})

	// Cache should be cleared for our entry, so we refetch and get the new results.
	trAlpha := s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Equal(t, []frontend.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, trAlpha)

	// This should now be from the cache.
	tr = s.getTriageHistory(context.Background(), mes, data.AlphaTest, data.AlphaPositiveDigest)
	assert.Equal(t, trAlpha, tr)
}

func TestCollectDigestsForBulkTriage_Success(t *testing.T) {
	unittest.SmallTest(t)

	results := []*frontend.SearchResult{
		{
			Test:       "apple",
			Digest:     "grannysmith",
			ClosestRef: common.PositiveRef,
		},
		{
			Test:       "apple",
			Digest:     "honeycrisp",
			ClosestRef: common.NoRef, // Nothing compares to a Honeycrisp Apple. They are the best kind.
		},
		{
			Test:       "grapefruit",
			Digest:     "oro_blanco",
			ClosestRef: common.NegativeRef, // Who likes grapefruit???
		},
	}

	bulkTriageData := collectDigestsForBulkTriage(results)
	assert.Equal(t, web_frontend.TriageRequestData{
		"apple": {
			"grannysmith": "positive",
			"honeycrisp":  "",
		},
		"grapefruit": {
			"oro_blanco": "negative",
		},
	}, bulkTriageData)
}

var everythingPublic publicparams.Matcher = nil

// By setting nothingFlaky to a high number, we make sure no trace is considered flaky because
// this threshold will always be higher than the number of unique digests for a trace.
const nothingFlaky = 10000

// makeThreeDevicesIndexer returns an IndexSource that returns the result of makeThreeDevicesIndex.
func makeThreeDevicesIndexer() indexer.IndexSource {
	mi := &mock_index.IndexSource{}

	fis := makeThreeDevicesIndex()
	mi.On("GetIndex").Return(fis)
	mi.On("GetIndexForCL", mock.Anything, mock.Anything).Return(nil)
	return mi
}

// makeThreeDevicesIndex returns a search index corresponding to the three_devices_data
// (which currently has nothing ignored).
func makeThreeDevicesIndex() *indexer.SearchIndex {
	cpxTile := tiling.NewComplexTile(data.MakeTestTile())
	dc := digest_counter.New(data.MakeTestTile())
	ps := paramsets.NewParamSummary(data.MakeTestTile(), dc)
	si, err := indexer.SearchIndexForTesting(cpxTile, [2]digest_counter.DigestCounter{dc, dc}, [2]paramsets.ParamSummary{ps, ps}, nil, nil)
	if err != nil {
		// Something is horribly broken with our test data/setup
		panic(err.Error())
	}
	return si
}

const userWhoTriaged = "test@example.com"

var (
	alphaPositiveTriageTS = time.Date(2020, time.March, 1, 2, 3, 4, 0, time.UTC)
	alphaNegativeTriageTS = time.Date(2020, time.March, 4, 2, 3, 4, 0, time.UTC)
	betaPositiveTriageTS  = time.Date(2020, time.March, 7, 2, 3, 4, 0, time.UTC)

	anyTime = mock.MatchedBy(func(time.Time) bool { return true })
)

func makeThreeDevicesExpectationStore() *mock_expectations.Store {
	mes := &mock_expectations.Store{}
	mes.On("Get", testutils.AnyContext).Return(data.MakeTestExpectations(), nil)

	mes.On("GetTriageHistory", testutils.AnyContext, data.AlphaTest, data.AlphaPositiveDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaPositiveTriageTS,
		},
	}, nil)
	mes.On("GetTriageHistory", testutils.AnyContext, data.AlphaTest, data.AlphaNegativeDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   alphaNegativeTriageTS,
		},
	}, nil)
	mes.On("GetTriageHistory", testutils.AnyContext, data.BetaTest, data.BetaPositiveDigest).Return([]expectations.TriageHistory{
		{
			User: userWhoTriaged,
			TS:   betaPositiveTriageTS,
		},
	}, nil)
	// Catch-all for the untriaged entries
	mes.On("GetTriageHistory", testutils.AnyContext, mock.Anything, mock.Anything).Return(nil, nil)

	return mes
}

func addChangelistExpectations(mes *mock_expectations.Store, crs string, clID string, issueExp *expectations.Expectations) *mock_expectations.Store {
	issueStore := &mock_expectations.Store{}
	mes.On("ForChangelist", clID, crs).Return(issueStore, nil)
	issueStore.On("Get", testutils.AnyContext).Return(issueExp, nil)
	return issueStore
}

func addDiffData(t *testing.T, data *schema.Tables, left types.Digest, right types.Digest, metric *diff.DiffMetrics) {
	// Arbitrary timestamp.
	ts := time.Date(2021, time.February, 15, 16, 17, 18, 0, time.UTC)
	// Add two rows, one for left-right and one for right-left, just like production data is
	// calculated and stored.
	data.DiffMetrics = append(data.DiffMetrics, schema.DiffMetricRow{
		LeftDigest:        d(t, left),
		RightDigest:       d(t, right),
		NumPixelsDiff:     metric.NumDiffPixels,
		PercentPixelsDiff: metric.PixelDiffPercent,
		MaxRGBADiffs:      metric.MaxRGBADiffs,
		MaxChannelDiff:    max(metric.MaxRGBADiffs),
		CombinedMetric:    metric.CombinedMetric,
		DimensionsDiffer:  metric.DimDiffer,
		Timestamp:         ts,
	})
	data.DiffMetrics = append(data.DiffMetrics, schema.DiffMetricRow{
		LeftDigest:        d(t, right),
		RightDigest:       d(t, left),
		NumPixelsDiff:     metric.NumDiffPixels,
		PercentPixelsDiff: metric.PixelDiffPercent,
		MaxRGBADiffs:      metric.MaxRGBADiffs,
		MaxChannelDiff:    max(metric.MaxRGBADiffs),
		CombinedMetric:    metric.CombinedMetric,
		DimensionsDiffer:  metric.DimDiffer,
		Timestamp:         ts,
	})
}

func max(nums [4]int) int {
	n := nums[0]
	for _, m := range nums {
		if m > n {
			n = m
		}
	}
	return n
}

// This is arbitrary data.
func makeSmallDiffMetric() *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    8,
		PixelDiffPercent: 0.02,
		MaxRGBADiffs:     [4]int{0, 48, 12, 0},
		DimDiffer:        false,
		CombinedMetric:   0.0005,
	}
}

func makeBigDiffMetric() *diff.DiffMetrics {
	return &diff.DiffMetrics{
		NumDiffPixels:    88812,
		PixelDiffPercent: 98.68,
		MaxRGBADiffs:     [4]int{102, 51, 13, 0},
		DimDiffer:        true,
		CombinedMetric:   4.7,
	}
}

func emptyCommentStore() comment.Store {
	mcs := &mock_comment.Store{}
	mcs.On("ListComments", testutils.AnyContext).Return(nil, nil)
	return mcs
}

func makeIndexWithIgnoreRules(t *testing.T, ignoreQueries ...string) *mock_index.IndexSource {
	fullTile := data.MakeTestTile()
	cpxTile := tiling.NewComplexTile(fullTile)
	var ignoreRules []ignore.Rule
	for _, q := range ignoreQueries {
		ignoreRules = append(ignoreRules, ignore.Rule{
			Query: q,
		})
	}
	filteredTile, pm, err := ignore.FilterIgnored(fullTile, ignoreRules)
	require.NoError(t, err)
	cpxTile.SetIgnoreRules(filteredTile, pm)
	fullTileCount := digest_counter.New(fullTile)
	fullTileSummary := paramsets.NewParamSummary(fullTile, fullTileCount)
	filteredTileCount := digest_counter.New(filteredTile)
	filteredTileSummary := paramsets.NewParamSummary(filteredTile, filteredTileCount)
	si, err := indexer.SearchIndexForTesting(cpxTile,
		[2]digest_counter.DigestCounter{filteredTileCount, fullTileCount},
		[2]paramsets.ParamSummary{filteredTileSummary, fullTileSummary}, nil, nil)
	require.NoError(t, err)
	mi := &mock_index.IndexSource{}
	mi.On("GetIndex").Return(si)
	return mi
}
