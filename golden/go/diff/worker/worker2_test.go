package worker

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/golden/go/sql/databuilder"

	"go.skia.org/infra/go/repo_root"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff/mocks"
	"go.skia.org/infra/golden/go/sql"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestWorkerImpl_CalculateDiffs_NoExistingData_Success(t *testing.T) {

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
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor))

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

func TestWorkerImpl_CalculateDiffs_ReadFromPrimaryBranch_Success(t *testing.T) {

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
	require.NoError(t, w.CalculateDiffs(ctx, grouping, nil))

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

func TestWorkerImpl_CalculateDiffs_DiffSubset_Success(t *testing.T) {

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
	require.NoError(t, w.CalculateDiffs(ctx, grouping, extraDigests))

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

func TestWorkerImpl_CalculateDiffs_ReadFromPrimaryBranch_SparseData_Success(t *testing.T) {

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), now.ContextKey, fakeNow)
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	// Only C03, C04, C05 will be in the last 3 commits for this data.
	sparseData := makeSparseData()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, sparseData))
	waitForSystemTime()
	w := New(db, &fsImageSource{root: kitchenSinkRoot(t)}, 3)

	grouping := paramtools.Params{
		types.CorpusField:     dks.RoundCorpus,
		types.PrimaryKeyField: dks.CircleTest,
	}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, []types.Digest{dks.DigestC06Pos_CL}))

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

func TestWorkerImpl_CalculateDiffs_ImageNotFound_PartialData(t *testing.T) {

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

	w := New(db, mis, 2)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToCalculateDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor))

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

func TestWorkerImpl_CalculateDiffs_CorruptedImage_PartialData(t *testing.T) {

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

	w := New(db, mis, 2)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToCalculateDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt}
	require.NoError(t, w.CalculateDiffs(ctx, grouping, imagesToCalculateDiffsFor))

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

func TestWorkerImpl_GetTriagedDigests_Success(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	w := New(db, nil, 100)

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

func TestWorkerImpl_GetCommonAndRecentDigests_SmokeTest(t *testing.T) {

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	waitForSystemTime()

	w := New(db, nil, 100)

	squareGrouping := paramtools.Params{types.CorpusField: dks.CornersCorpus, types.PrimaryKeyField: dks.SquareTest}

	actualSquare, err := w.getCommonAndRecentDigests(ctx, squareGrouping)
	require.NoError(t, err)
	assert.ElementsMatch(t, []schema.DigestBytes{
		d(dks.DigestA01Pos), d(dks.DigestA02Pos), d(dks.DigestA03Pos), d(dks.DigestA04Unt),
		d(dks.DigestA05Unt), d(dks.DigestA06Unt), d(dks.DigestA07Pos), d(dks.DigestA08Pos),
		d(dks.DigestA09Neg),
	}, actualSquare)
}

func newWorker2UsingImagesFromKitchenSink(t *testing.T, db *pgxpool.Pool) *WorkerImpl {
	return New(db, &fsImageSource{root: kitchenSinkRoot(t)}, 200)
}

var kitchenSinkData = dks.Build()

// expectedFromKS returns the computed diff metric from the kitchen sink data. It replaces the
// default timestamp with the provided timestamp.
func expectedFromKS(t *testing.T, left types.Digest, right types.Digest, ts time.Time) schema.DiffMetricRow {
	leftB := d(left)
	rightB := d(right)
	for _, row := range kitchenSinkData.DiffMetrics {
		if bytes.Equal(leftB, row.LeftDigest) && bytes.Equal(rightB, row.RightDigest) {
			row.Timestamp = ts
			return row
		}
	}
	require.Fail(t, "Could not find diff metrics for %s-%s", left, right)
	return schema.DiffMetricRow{}
}

func makeSparseData() schema.Tables {
	b := databuilder.TablesBuilder{TileWidth: 1}
	// Make a few commits, each on their own tile
	b.CommitsWithData().
		Insert("337", "whomever", "commit 337", "2020-12-01T00:00:01Z").
		Insert("437", "whomever", "commit 437", "2020-12-01T00:00:02Z").
		Insert("537", "whomever", "commit 537", "2020-12-01T00:00:03Z").
		Insert("637", "whomever", "commit 637", "2020-12-01T00:00:04Z").
		Insert("687", "whomever", "commit 687", "2020-12-01T00:00:05Z")

	b.SetDigests(map[rune]types.Digest{
		// All of these will be untriaged because diffs don't care about triage status
		'a': dks.DigestC01Pos,
		'b': dks.DigestC02Pos,
		'c': dks.DigestC03Unt,
		'd': dks.DigestC04Unt,
		'e': dks.DigestC05Unt,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)

	b.AddTracesWithCommonKeys(paramtools.Params{
		types.CorpusField: dks.RoundCorpus,
	}).History("abcde").Keys([]paramtools.Params{{types.PrimaryKeyField: dks.CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"file1", "file2", "file3", "file4", "file5"}, // not used in this test
			[]string{"2020-12-01T00:00:05Z", "2020-12-01T00:00:05Z", "2020-12-01T00:00:05Z", "2020-12-01T00:00:05Z", "2020-12-01T00:00:05Z"})

	b.AddIgnoreRule("userOne", "userOne", "2020-12-02T0:00:00Z", "nop ignore",
		paramtools.ParamSet{"matches": []string{"nothing"}})

	rv := b.Build()
	rv.DiffMetrics = nil
	return rv
}

func d(d types.Digest) schema.DigestBytes {
	b, err := sql.DigestToBytes(d)
	if err != nil {
		panic(err)
	}
	return b
}

func getAllDiffMetricRows(t *testing.T, db *pgxpool.Pool) []schema.DiffMetricRow {
	rows := sqltest.GetAllRows(context.Background(), t, db, "DiffMetrics", &schema.DiffMetricRow{}).([]schema.DiffMetricRow)
	for _, r := range rows {
		// spot check that we handle arrays correctly.
		assert.NotEqual(t, [4]int{0, 0, 0, 0}, r.MaxRGBADiffs)
	}
	return rows
}

func getAllProblemImageRows(t *testing.T, db *pgxpool.Pool) []schema.ProblemImageRow {
	return sqltest.GetAllRows(context.Background(), t, db, "ProblemImages", &schema.ProblemImageRow{}).([]schema.ProblemImageRow)
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

// fsImageSource returns an image from the local file system, looking in a given root directory.
type fsImageSource struct {
	root string
}

func (f fsImageSource) GetImage(_ context.Context, digest types.Digest) ([]byte, error) {
	p := filepath.Join(f.root, string(digest)+".png")
	return ioutil.ReadFile(p)
}

func kitchenSinkRoot(t *testing.T) string {
	root, err := repo_root.Get()
	if err != nil {
		require.NoError(t, err)
	}
	return filepath.Join(root, "golden", "go", "sql", "datakitchensink", "img")
}
