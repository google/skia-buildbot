package ref_differ

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

func TestSQLRefDiffer_PositiveAndNegativeDigestsExist_CombinedMetric_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := generateSQLDiffs()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	es := makeClassifier([]types.Digest{posDigestOne, posDigestTwo, posDigestThree}, []types.Digest{negDigestSix, negDigestSeven})
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				posDigestTwo:   arbitraryParamSetForTest(),
				posDigestThree: arbitraryParamSetForTest(),
				negDigestSix:   arbitraryParamSetForTest(),
				negDigestSeven: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				posDigestTwo:   1,
				posDigestThree: 1,
				negDigestSix:   1,
				negDigestSeven: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.CombinedMetric
	matches := []string{types.PrimaryKeyField} // This is the default for several gold queries.
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.NegativeRef, input.ClosestRef)
	posRef := input.RefDiffs[frontend.PositiveRef]
	require.NotNil(t, posRef)
	assert.Equal(t, posDigestThree, posRef.Digest)
	assert.Equal(t, [4]int{9, 10, 11, 12}, posRef.MaxRGBADiffs)
	assert.Equal(t, float32(1.0), posRef.CombinedMetric)
	assert.Equal(t, 300, posRef.NumDiffPixels)
	assert.Equal(t, float32(0.3), posRef.PixelDiffPercent)
	assert.True(t, posRef.DimDiffer)
	assert.Equal(t, posRef.CombinedMetric, posRef.QueryMetric)

	negRef := input.RefDiffs[frontend.NegativeRef]
	require.NotNil(t, negRef)
	assert.Equal(t, negDigestSeven, negRef.Digest)
	assert.Equal(t, [4]int{17, 18, 19, 20}, negRef.MaxRGBADiffs)
	assert.Equal(t, float32(0.5), negRef.CombinedMetric)
	assert.Equal(t, 250, negRef.NumDiffPixels)
	assert.Equal(t, float32(0.25), negRef.PixelDiffPercent)
	assert.False(t, negRef.DimDiffer)
	assert.Equal(t, negRef.CombinedMetric, negRef.QueryMetric)
}

func TestSQLRefDiffer_PositiveAndNegativeDigestsExist_PercentPixels_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := generateSQLDiffs()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	es := makeClassifier([]types.Digest{posDigestOne, posDigestTwo, posDigestThree}, []types.Digest{negDigestSix, negDigestSeven})
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				posDigestTwo:   arbitraryParamSetForTest(),
				posDigestThree: arbitraryParamSetForTest(),
				negDigestSix:   arbitraryParamSetForTest(),
				negDigestSeven: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				posDigestTwo:   1,
				posDigestThree: 1,
				negDigestSix:   1,
				negDigestSeven: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.PercentMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.PositiveRef, input.ClosestRef)
	posRef := input.RefDiffs[frontend.PositiveRef]
	require.NotNil(t, posRef)
	assert.Equal(t, posDigestTwo, posRef.Digest)
	assert.Equal(t, float32(0.1), posRef.PixelDiffPercent)
	assert.Equal(t, posRef.PixelDiffPercent, posRef.QueryMetric)
	negRef := input.RefDiffs[frontend.NegativeRef]
	require.NotNil(t, negRef)
	assert.Equal(t, negDigestSix, negRef.Digest)
	assert.Equal(t, float32(0.15), negRef.PixelDiffPercent)
	assert.Equal(t, negRef.PixelDiffPercent, negRef.QueryMetric)
}

func TestSQLRefDiffer_PositiveAndNegativeDigestsExist_NumPixels_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := generateSQLDiffs()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	es := makeClassifier([]types.Digest{posDigestOne, posDigestTwo, posDigestThree}, []types.Digest{negDigestSix, negDigestSeven})
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				posDigestTwo:   arbitraryParamSetForTest(),
				posDigestThree: arbitraryParamSetForTest(),
				negDigestSix:   arbitraryParamSetForTest(),
				negDigestSeven: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				posDigestTwo:   1,
				posDigestThree: 1,
				negDigestSix:   1,
				negDigestSeven: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.PixelMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.PositiveRef, input.ClosestRef)
	posRef := input.RefDiffs[frontend.PositiveRef]
	require.NotNil(t, posRef)
	assert.Equal(t, posDigestOne, posRef.Digest)
	assert.Equal(t, 100, posRef.NumDiffPixels)
	assert.Equal(t, float32(posRef.NumDiffPixels), posRef.QueryMetric)
	negRef := input.RefDiffs[frontend.NegativeRef]
	require.NotNil(t, negRef)
	assert.Equal(t, negDigestSix, negRef.Digest)
	assert.Equal(t, 150, negRef.NumDiffPixels)
	assert.Equal(t, float32(negRef.NumDiffPixels), negRef.QueryMetric)
}

func TestSQLRefDiffer_NoNegativeDigests_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := generateSQLDiffs()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	es := makeClassifier([]types.Digest{posDigestOne, posDigestTwo, posDigestThree}, nil)
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				posDigestTwo:   arbitraryParamSetForTest(),
				posDigestThree: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				posDigestTwo:   1,
				posDigestThree: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.CombinedMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.PositiveRef, input.ClosestRef)
	posRef := input.RefDiffs[frontend.PositiveRef]
	require.NotNil(t, posRef)
	assert.Equal(t, posDigestThree, posRef.Digest)

	negRef := input.RefDiffs[frontend.NegativeRef]
	require.Nil(t, negRef)
}

func TestSQLRefDiffer_NoMatchingDigests_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := generateSQLDiffs()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()

	es := makeClassifier([]types.Digest{posDigestOne}, []types.Digest{negDigestSeven})
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				negDigestSeven: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				negDigestSeven: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.CombinedMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     "The wrong test name that nothing matches",
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.NoRef, input.ClosestRef)
	assert.Nil(t, input.RefDiffs[frontend.PositiveRef])
	assert.Nil(t, input.RefDiffs[frontend.NegativeRef])
}

func TestSQLRefDiffer_NoDiffMetrics_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	es := makeClassifier([]types.Digest{posDigestOne}, []types.Digest{negDigestSeven})
	mis := &mock_index.IndexSearcher{}
	mis.On("GetParamsetSummaryByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]map[types.Digest]paramtools.ParamSet{
			testName: {
				posDigestOne:   arbitraryParamSetForTest(),
				negDigestSeven: arbitraryParamSetForTest(),
			},
		},
	)
	mis.On("DigestCountsByTest", types.ExcludeIgnoredTraces).Return(
		map[types.TestName]digest_counter.DigestCount{
			testName: {
				posDigestOne:   1,
				negDigestSeven: 1,
			},
		},
	)

	rd := NewSQLImpl(db, es, mis)

	metric := query.CombinedMetric
	matches := []string{types.PrimaryKeyField}
	input := frontend.SearchResult{
		ParamSet: arbitraryParamSetForTest(),
		Digest:   untriagedDigestZero,
		Test:     testName,
	}
	err := rd.FillRefDiffs(context.Background(), &input, metric, matches, matchAll, types.ExcludeIgnoredTraces)

	require.NoError(t, err)
	assert.Equal(t, frontend.NoRef, input.ClosestRef)
	assert.Nil(t, input.RefDiffs[frontend.PositiveRef])
	assert.Nil(t, input.RefDiffs[frontend.NegativeRef])
}

func makeClassifier(positive []types.Digest, negative []types.Digest) expectations.Classifier {
	var exp expectations.Expectations
	for _, p := range positive {
		exp.Set(testName, p, expectations.Positive)
	}
	for _, n := range negative {
		exp.Set(testName, n, expectations.Negative)
	}
	return &exp
}

// The data generated here is nonsensical in the sense that the numbers could not correspond to
// real images. However, it is still useful because it has different orderings for different
// metric types.
func generateSQLDiffs() schema.Tables {
	now := time.Date(2021, time.February, 3, 4, 5, 6, 0, time.UTC)
	return schema.Tables{DiffMetrics: []schema.DiffMetricRow{{
		LeftDigest:        mustDigestToBytes(untriagedDigestZero),
		RightDigest:       mustDigestToBytes(posDigestOne),
		NumPixelsDiff:     100,
		PercentPixelsDiff: 0.2,
		MaxRGBADiffs:      [4]int{1, 2, 3, 4},
		MaxChannelDiff:    4,
		CombinedMetric:    3.0,
		DimensionsDiffer:  false,
		Timestamp:         now,
	}, {
		LeftDigest:        mustDigestToBytes(untriagedDigestZero),
		RightDigest:       mustDigestToBytes(posDigestTwo),
		NumPixelsDiff:     200,
		PercentPixelsDiff: 0.1,
		MaxRGBADiffs:      [4]int{5, 6, 7, 8},
		MaxChannelDiff:    8,
		CombinedMetric:    2.0,
		DimensionsDiffer:  false,
		Timestamp:         now,
	}, {
		LeftDigest:        mustDigestToBytes(untriagedDigestZero),
		RightDigest:       mustDigestToBytes(posDigestThree),
		NumPixelsDiff:     300,
		PercentPixelsDiff: 0.3,
		MaxRGBADiffs:      [4]int{9, 10, 11, 12},
		MaxChannelDiff:    12,
		CombinedMetric:    1.0,
		DimensionsDiffer:  true,
		Timestamp:         now,
	}, {
		LeftDigest:        mustDigestToBytes(untriagedDigestZero),
		RightDigest:       mustDigestToBytes(negDigestSix),
		NumPixelsDiff:     150,
		PercentPixelsDiff: .15,
		MaxRGBADiffs:      [4]int{13, 14, 15, 16},
		MaxChannelDiff:    16,
		CombinedMetric:    6,
		DimensionsDiffer:  true,
		Timestamp:         now,
	}, {
		LeftDigest:        mustDigestToBytes(untriagedDigestZero),
		RightDigest:       mustDigestToBytes(negDigestSeven),
		NumPixelsDiff:     250,
		PercentPixelsDiff: .250,
		MaxRGBADiffs:      [4]int{17, 18, 19, 20},
		MaxChannelDiff:    20,
		CombinedMetric:    0.5,
		DimensionsDiffer:  false,
		Timestamp:         now,
	}}}
}

const (
	untriagedDigestZero = types.Digest("00000000000000000000000000000000")
	posDigestOne        = types.Digest("11111111111111111111111111111111")
	posDigestTwo        = types.Digest("22222222222222222222222222222222")
	posDigestThree      = types.Digest("33333333333333333333333333333333")
	negDigestSix        = types.Digest("66666666666666666666666666666666")
	negDigestSeven      = types.Digest("77777777777777777777777777777777")
)

func arbitraryParamSetForTest() paramtools.ParamSet {
	return paramtools.ParamSet{
		types.PrimaryKeyField: []string{string(testName)},
		"arbitrary":           []string{"data"},
		"does":                []string{"not", "matter"},
	}
}

func mustDigestToBytes(d types.Digest) schema.DigestBytes {
	db, err := sql.DigestToBytes(d)
	if err != nil {
		panic(err)
	}
	return db
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

var matchAll = paramtools.ParamSet{}

const (
	testName = types.TestName("some_test")
)
