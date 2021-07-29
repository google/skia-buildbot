package worker

import (
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff/mocks"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestWorkerImpl2_CalculateDiffs_NoExistingData_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	waitForSystemTime()
	w := newWorker2UsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToCalculateDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt, dks.DigestA05Unt}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor, imagesToCalculateDiffsFor))

	actualMetrics := getAllDiffMetricRows(t, db)
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA04Unt, fakeNow),
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA05Unt, fakeNow),

		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA01Pos, fakeNow),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA04Unt, fakeNow),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA05Unt, fakeNow),

		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA01Pos, fakeNow),
		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA05Unt, fakeNow),

		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA01Pos, fakeNow),
		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA04Unt, fakeNow),
	}, actualMetrics)
	assert.Empty(t, getAllProblemImageRows(t, db))
}

func TestWorkerImpl2_CalculateDiffs_ReadFromPrimaryBranch_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Remove existing diffs, so the ones for triangle test can be recomputed.
	existingData.DiffMetrics = nil
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	w := newWorker2UsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     dks.CornersCorpus,
		types.PrimaryKeyField: dks.TriangleTest,
	}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, nil, nil))

	actualMetrics := getAllDiffMetricRows(t, db)
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestBlank, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB01Pos, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB02Pos, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB03Neg, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB04Neg, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB03Neg, fakeNow),
	}, actualMetrics)
	assert.Empty(t, getAllProblemImageRows(t, db))
}

func TestWorkerImpl2_CalculateDiffs_DiffSubset_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	// Remove existing diffs, so the ones for triangle test can be recomputed.
	existingData.DiffMetrics = nil
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	w := newWorker2UsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     dks.CornersCorpus,
		types.PrimaryKeyField: dks.TriangleTest,
	}
	// By adding in extra digests, we make sure to hit the calculateDiffSubset branch
	var extraDigests []types.Digest
	for i := 0; i < computeTotalGridCutoff+1; i++ {
		extraDigests = append(extraDigests, dks.DigestBlank)
	}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, extraDigests, nil))

	actualMetrics := getAllDiffMetricRows(t, db)
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestBlank, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestBlank, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB01Pos, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestB01Pos, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB02Pos, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB03Neg, fakeNow),
		expectedFromKS(t, dks.DigestB02Pos, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB03Neg, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB03Neg, dks.DigestB04Neg, fakeNow),

		expectedFromKS(t, dks.DigestB04Neg, dks.DigestBlank, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB01Pos, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB02Pos, fakeNow),
		expectedFromKS(t, dks.DigestB04Neg, dks.DigestB03Neg, fakeNow),
	}, actualMetrics)
	assert.Empty(t, getAllProblemImageRows(t, db))
}

func TestWorkerImpl2_CalculateDiffs_ReadFromPrimaryBranch_SparseData_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	// Only C03, C04, C05 will be in the last 3 commits for this data.
	sparseData := makeSparseData()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, sparseData))
	waitForSystemTime()
	w := NewV2(db, &fsImageSource{root: kitchenSinkRoot(t)}, 3)

	grouping := paramtools.Params{
		types.CorpusField:     dks.RoundCorpus,
		types.PrimaryKeyField: dks.CircleTest,
	}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, []types.Digest{dks.DigestC06Pos_CL}, []types.Digest{dks.DigestC06Pos_CL}))

	actualMetrics := getAllDiffMetricRows(t, db)
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestC03Unt, dks.DigestC04Unt, fakeNow),
		expectedFromKS(t, dks.DigestC03Unt, dks.DigestC05Unt, fakeNow),
		expectedFromKS(t, dks.DigestC03Unt, dks.DigestC06Pos_CL, fakeNow),

		expectedFromKS(t, dks.DigestC04Unt, dks.DigestC03Unt, fakeNow),
		expectedFromKS(t, dks.DigestC04Unt, dks.DigestC05Unt, fakeNow),
		expectedFromKS(t, dks.DigestC04Unt, dks.DigestC06Pos_CL, fakeNow),

		expectedFromKS(t, dks.DigestC05Unt, dks.DigestC03Unt, fakeNow),
		expectedFromKS(t, dks.DigestC05Unt, dks.DigestC04Unt, fakeNow),
		expectedFromKS(t, dks.DigestC05Unt, dks.DigestC06Pos_CL, fakeNow),

		expectedFromKS(t, dks.DigestC06Pos_CL, dks.DigestC03Unt, fakeNow),
		expectedFromKS(t, dks.DigestC06Pos_CL, dks.DigestC04Unt, fakeNow),
		expectedFromKS(t, dks.DigestC06Pos_CL, dks.DigestC05Unt, fakeNow),
	}, actualMetrics)
	assert.Empty(t, getAllProblemImageRows(t, db))
}

func TestWorkerImpl2_CalculateDiffs_ImageNotFound_PartialData(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	waitForSystemTime()
	// Set up an image source that can download A01 and A02, but returns error on A04
	b01, err := ioutil.ReadFile(filepath.Join(kitchenSinkRoot(t), string(dks.DigestA01Pos)+".png"))
	require.NoError(t, err)
	b02, err := ioutil.ReadFile(filepath.Join(kitchenSinkRoot(t), string(dks.DigestA02Pos)+".png"))
	require.NoError(t, err)
	mis := &mocks.ImageSource{}
	mis.On("GetImage", testutils.AnyContext, dks.DigestA01Pos).Return(b01, nil)
	mis.On("GetImage", testutils.AnyContext, dks.DigestA02Pos).Return(b02, nil)
	mis.On("GetImage", testutils.AnyContext, dks.DigestA04Unt).Return(nil, errors.New("not found"))

	w := NewV2(db, mis, 2)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToCalculateDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor, imagesToCalculateDiffsFor))

	actualMetrics := getAllDiffMetricRows(t, db)
	// We should see partial success
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA01Pos, fakeNow),
	}, actualMetrics)

	actualProblemImageRows := getAllProblemImageRows(t, db)
	require.Len(t, actualProblemImageRows, 1)
	problem := actualProblemImageRows[0]
	assert.Equal(t, string(dks.DigestA04Unt), problem.Digest)
	// could be more than 1 because of multiple goroutines running
	assert.True(t, problem.NumErrors >= 1)
	assert.Contains(t, problem.LatestError, "not found")
	assert.Equal(t, fakeNow, problem.ErrorTS)
}

func TestWorkerImpl2_CalculateDiffs_CorruptedImage_PartialData(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 2, 2, 2, 2, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	// Write a sentinel error for A04. We expect this to be updated.
	existingProblem := schema.Tables{ProblemImages: []schema.ProblemImageRow{{
		Digest:      string(dks.DigestA04Unt),
		NumErrors:   100,
		LatestError: "not found",
		ErrorTS:     time.Date(2020, time.January, 1, 1, 1, 1, 0, time.UTC),
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingProblem))
	waitForSystemTime()

	// Set up an image source that can download A01 and A02, but invalid PNG data for A04
	b01, err := ioutil.ReadFile(filepath.Join(kitchenSinkRoot(t), string(dks.DigestA01Pos)+".png"))
	require.NoError(t, err)
	b02, err := ioutil.ReadFile(filepath.Join(kitchenSinkRoot(t), string(dks.DigestA02Pos)+".png"))
	require.NoError(t, err)
	mis := &mocks.ImageSource{}
	mis.On("GetImage", testutils.AnyContext, dks.DigestA01Pos).Return(b01, nil)
	mis.On("GetImage", testutils.AnyContext, dks.DigestA02Pos).Return(b02, nil)
	mis.On("GetImage", testutils.AnyContext, dks.DigestA04Unt).Return([]byte(`not a png`), nil)

	w := NewV2(db, mis, 2)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToCalculateDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor, imagesToCalculateDiffsFor))

	actualMetrics := getAllDiffMetricRows(t, db)
	// We should see partial success
	assert.Equal(t, []schema.DiffMetricRow{
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA01Pos, fakeNow),
	}, actualMetrics)

	actualProblemImageRows := getAllProblemImageRows(t, db)
	require.Len(t, actualProblemImageRows, 1)
	problem := actualProblemImageRows[0]
	assert.Equal(t, string(dks.DigestA04Unt), problem.Digest)
	// The sentinel value is 100. This should be greater than that because of the new error.
	assert.True(t, problem.NumErrors >= 101)
	assert.Contains(t, problem.LatestError, "png: invalid format: not a PNG file")
	assert.Equal(t, fakeNow, problem.ErrorTS)
}

func TestWorkerImpl2_GetTriagedDigests_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	w := NewV2(db, nil, 100)

	squareGrouping := paramtools.Params{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest}
	triangleGrouping := paramtools.Params{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.TriangleTest}
	circleGrouping := paramtools.Params{types.CorpusField: dks.RoundCorpus, types.PrimaryKeyField: dks.CircleTest}

	actualSquare, err := w.getTriagedDigests(ctx, squareGrouping, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []schema.DigestBytes{
		d(dks.DigestA01Pos), d(dks.DigestA02Pos), d(dks.DigestA03Pos), d(dks.DigestA07Pos),
		d(dks.DigestA08Pos), d(dks.DigestA09Neg),
	}, actualSquare)

	actualTriangle, err := w.getTriagedDigests(ctx, triangleGrouping, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []schema.DigestBytes{
		d(dks.DigestB01Pos), d(dks.DigestB02Pos), d(dks.DigestB03Neg), d(dks.DigestB04Neg),
	}, actualTriangle)

	actualCircle, err := w.getTriagedDigests(ctx, circleGrouping, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []schema.DigestBytes{
		d(dks.DigestC01Pos), d(dks.DigestC02Pos),
	}, actualCircle)
}

func TestWorkerImpl2_GetCommonAndRecentDigests_SmokeTest(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	w := NewV2(db, nil, 100)

	squareGrouping := paramtools.Params{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest}

	actualSquare, err := w.getCommonAndRecentDigests(ctx, squareGrouping)
	require.NoError(t, err)
	assert.ElementsMatch(t, []schema.DigestBytes{
		d(dks.DigestA01Pos), d(dks.DigestA02Pos), d(dks.DigestA03Pos), d(dks.DigestA04Unt),
		d(dks.DigestA05Unt), d(dks.DigestA06Unt), d(dks.DigestA07Pos), d(dks.DigestA08Pos),
		d(dks.DigestA09Neg),
	}, actualSquare)
}

func newWorker2UsingImagesFromKitchenSink(t *testing.T, db *pgxpool.Pool) *WorkerImpl2 {
	return NewV2(db, &fsImageSource{root: kitchenSinkRoot(t)}, 200)
}
