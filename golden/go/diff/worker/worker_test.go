package worker

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/mocks"
	"go.skia.org/infra/golden/go/sql"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestComputeDiffs_NoExistingData_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), NowSourceKey, mockTime(fakeNow))
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	waitForSystemTime()
	w := newWorkerUsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToComputeDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt, dks.DigestA05Unt}
	require.NoError(t, w.ComputeDiffs(ctx, grouping, imagesToComputeDiffsFor))

	rows, err := db.Query(ctx, `SELECT * FROM DiffMetrics ORDER BY left_digest, right_digest`)
	require.NoError(t, err)
	defer rows.Close()
	var actualMetrics []schema.DiffMetricRow
	for rows.Next() {
		var m schema.DiffMetricRow
		require.NoError(t, rows.Scan(&m.LeftDigest, &m.RightDigest, &m.NumPixelsDiff, &m.PercentPixelsDiff,
			&m.MaxRGBADiffs, &m.MaxChannelDiff, &m.CombinedMetric, &m.DimensionsDiffer, &m.Timestamp))
		m.Timestamp = m.Timestamp.UTC()
		actualMetrics = append(actualMetrics, m)
		// spot check that we handle arrays correctly.
		assert.NotEqual(t, [4]int{0, 0, 0, 0}, m.MaxRGBADiffs)
	}
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
}

func TestComputeDiffs_MultipleBatches_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), NowSourceKey, mockTime(fakeNow))
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	waitForSystemTime()
	// all images loaded will be the same, so diff values are all zeros.
	w := newWorkerUsingBlankImages(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	var imagesToComputeDiffsFor []types.Digest
	// 16 digests should result in 32 * 32 - 32 diffs (a 32 by 32 square with the diagonal removed)
	// This is more than the batchSize in writeMetrics
	for i := 0; i < 32; i++ {
		d := fmt.Sprintf("%032d", i)
		imagesToComputeDiffsFor = append(imagesToComputeDiffsFor, types.Digest(d))
	}
	require.NoError(t, w.ComputeDiffs(ctx, grouping, imagesToComputeDiffsFor))

	row := db.QueryRow(ctx, `SELECT count(*) FROM DiffMetrics`)
	count := 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 32*31, count)
}

func TestComputeDiffs_ExistingMetrics_NoExistingTiledTraces_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), NowSourceKey, mockTime(fakeNow))
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{DiffMetrics: []schema.DiffMetricRow{
		sentinelMetricRow(dks.DigestA01Pos, dks.DigestA02Pos), // should not be recomputed
		sentinelMetricRow(dks.DigestA02Pos, dks.DigestA01Pos), // should not be recomputed
		sentinelMetricRow(dks.DigestBlank, dks.DigestA04Unt),  // should not impact computation
		sentinelMetricRow(dks.DigestA04Unt, dks.DigestBlank),  // should not impact computation
	}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	w := newWorkerUsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToComputeDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos, dks.DigestA04Unt, dks.DigestA05Unt}
	require.NoError(t, w.ComputeDiffs(ctx, grouping, imagesToComputeDiffsFor))

	rows, err := db.Query(ctx, `SELECT * FROM DiffMetrics ORDER BY left_digest, right_digest`)
	require.NoError(t, err)
	defer rows.Close()
	var actualMetrics []schema.DiffMetricRow
	for rows.Next() {
		var m schema.DiffMetricRow
		require.NoError(t, rows.Scan(&m.LeftDigest, &m.RightDigest, &m.NumPixelsDiff, &m.PercentPixelsDiff,
			&m.MaxRGBADiffs, &m.MaxChannelDiff, &m.CombinedMetric, &m.DimensionsDiffer, &m.Timestamp))
		m.Timestamp = m.Timestamp.UTC()
		actualMetrics = append(actualMetrics, m)
	}
	assert.Equal(t, []schema.DiffMetricRow{
		sentinelMetricRow(dks.DigestBlank, dks.DigestA04Unt),

		sentinelMetricRow(dks.DigestA01Pos, dks.DigestA02Pos),
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA04Unt, fakeNow),
		expectedFromKS(t, dks.DigestA01Pos, dks.DigestA05Unt, fakeNow),

		sentinelMetricRow(dks.DigestA02Pos, dks.DigestA01Pos),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA04Unt, fakeNow),
		expectedFromKS(t, dks.DigestA02Pos, dks.DigestA05Unt, fakeNow),

		sentinelMetricRow(dks.DigestA04Unt, dks.DigestBlank),
		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA01Pos, fakeNow),
		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA04Unt, dks.DigestA05Unt, fakeNow),

		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA01Pos, fakeNow),
		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA02Pos, fakeNow),
		expectedFromKS(t, dks.DigestA05Unt, dks.DigestA04Unt, fakeNow),
	}, actualMetrics)
}

func TestComputeDiffs_NoNewMetrics_Success(t *testing.T) {
	unittest.LargeTest(t)

	fakeNow := time.Date(2021, time.February, 1, 1, 1, 1, 0, time.UTC)
	ctx := context.WithValue(context.Background(), NowSourceKey, mockTime(fakeNow))
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := schema.Tables{DiffMetrics: []schema.DiffMetricRow{
		sentinelMetricRow(dks.DigestA01Pos, dks.DigestA02Pos), // should not be recomputed
		sentinelMetricRow(dks.DigestA02Pos, dks.DigestA01Pos), // should not be recomputed
	}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	waitForSystemTime()
	w := newWorkerUsingImagesFromKitchenSink(t, db)

	grouping := paramtools.Params{
		types.CorpusField:     "not used",
		types.PrimaryKeyField: "not used",
	}
	imagesToComputeDiffsFor := []types.Digest{dks.DigestA01Pos, dks.DigestA02Pos}
	require.NoError(t, w.ComputeDiffs(ctx, grouping, imagesToComputeDiffsFor))

	rows, err := db.Query(ctx, `SELECT * FROM DiffMetrics ORDER BY left_digest, right_digest`)
	require.NoError(t, err)
	defer rows.Close()
	var actualMetrics []schema.DiffMetricRow
	for rows.Next() {
		var m schema.DiffMetricRow
		require.NoError(t, rows.Scan(&m.LeftDigest, &m.RightDigest, &m.NumPixelsDiff, &m.PercentPixelsDiff,
			&m.MaxRGBADiffs, &m.MaxChannelDiff, &m.CombinedMetric, &m.DimensionsDiffer, &m.Timestamp))
		m.Timestamp = m.Timestamp.UTC()
		actualMetrics = append(actualMetrics, m)
	}
	assert.Equal(t, []schema.DiffMetricRow{
		sentinelMetricRow(dks.DigestA01Pos, dks.DigestA02Pos),
		sentinelMetricRow(dks.DigestA02Pos, dks.DigestA01Pos),
	}, actualMetrics)
}

// sentinelMetricRow returns a DiffMetricRow with arbitrary data that should not change as a result
// of the test using it (DiffMetrics are immutable).
func sentinelMetricRow(left types.Digest, right types.Digest) schema.DiffMetricRow {
	leftB, err := sql.DigestToBytes(left)
	if err != nil {
		panic(err)
	}
	rightB, err := sql.DigestToBytes(right)
	if err != nil {
		panic(err)
	}
	return schema.DiffMetricRow{
		LeftDigest:        leftB,
		RightDigest:       rightB,
		NumPixelsDiff:     -1,
		PercentPixelsDiff: -1,
		MaxRGBADiffs:      [4]int{-1, -1, -1, -1},
		MaxChannelDiff:    -1,
		CombinedMetric:    -1,
		Timestamp:         time.Date(2020, time.March, 14, 15, 9, 26, 0, time.UTC),
	}
}

func newWorkerUsingImagesFromKitchenSink(t *testing.T, db *pgxpool.Pool) *WorkerImpl {
	infraRoot, err := repo_root.Get()
	require.NoError(t, err)
	kitchenSinkPath := filepath.Join(infraRoot, "golden", "go", "sql", "datakitchensink", "img")
	return New(db, &fsImageSource{root: kitchenSinkPath})
}

func newWorkerUsingBlankImages(t *testing.T, db *pgxpool.Pool) *WorkerImpl {
	infraRoot, err := repo_root.Get()
	require.NoError(t, err)
	blankImagePath := filepath.Join(infraRoot, "golden", "go", "sql", "datakitchensink", "img", string(dks.DigestBlank+".png"))
	return New(db, &fixedImageSource{img: blankImagePath})
}

var kitchenSinkData = dks.Build()

// expectedFromKS returns the computed diff metric from the kitchen sink data. It replaces the
// default timestamp with the provided timestamp.
func expectedFromKS(t *testing.T, left types.Digest, right types.Digest, ts time.Time) schema.DiffMetricRow {
	leftB, err := sql.DigestToBytes(left)
	require.NoError(t, err)
	rightB, err := sql.DigestToBytes(right)
	require.NoError(t, err)
	for _, row := range kitchenSinkData.DiffMetrics {
		if bytes.Equal(leftB, row.LeftDigest) && bytes.Equal(rightB, row.RightDigest) {
			row.Timestamp = ts
			return row
		}
	}
	require.Fail(t, "Could not find diff metrics for %s-%s", left, right)
	return schema.DiffMetricRow{}
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

func mockTime(ts time.Time) NowSource {
	mt := mocks.NowSource{}
	mt.On("Now").Return(ts)
	return &mt
}

// fsImageSource returns an image from the local file system, looking in a given root directory.
type fsImageSource struct {
	root string
}

func (f fsImageSource) GetImage(_ context.Context, digest types.Digest) ([]byte, error) {
	p := filepath.Join(f.root, string(digest)+".png")
	return ioutil.ReadFile(p)
}

// fixedImageSource returns the same bytes for every image
type fixedImageSource struct {
	img string
}

func (f fixedImageSource) GetImage(_ context.Context, _ types.Digest) ([]byte, error) {
	return ioutil.ReadFile(f.img)
}
