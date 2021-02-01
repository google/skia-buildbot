package worker

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"go.skia.org/infra/golden/go/sql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/mocks"
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
	infraRoot, err := repo_root.Get()
	require.NoError(t, err)
	kitchenSinkPath := filepath.Join(infraRoot, "golden", "go", "sql", "datakitchensink", "img")

	w := New(db, &fsImageSource{root: kitchenSinkPath})

	grouping := paramtools.Params{
		types.CorpusField:     "my-corpus",
		types.PrimaryKeyField: "test-one",
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
	assert.NotEmpty(t, actualMetrics)
}

var kitchenSinkData = dks.Build()

// expectedFromKS returns the computed diff metric from the kitchen sink data. It replaces the
// default timestamp with the provided timestamp.
func expectedFromKS(t *testing.T, left types.Digest, right types.Digest, ts time.Time) schema.DiffMetricRow {
	leftB, err := sql.DigestToBytes(left)
	require.NoError(t, err)
	rightB, err := sql.DigestToBytes(right)
	for _, row := range kitchenSinkData.DiffMetrics {
		if bytes.Equal(leftB, row.LeftDigest) && bytes.Equal(rightB, row.RightDigest) {
			row.Timestamp = ts
			return row
		}
	}
	require.Fail(t, "Could not find diff metrics for %s-%s", left, right)
	return schema.DiffMetricRow{}
}

func mockTime(ts time.Time) NowSource {
	mt := mocks.NowSource{}
	mt.On("Now").Return(ts)
	return &mt
}

type fsImageSource struct {
	root string
}

func (f fsImageSource) GetImage(_ context.Context, digest types.Digest) ([]byte, error) {
	p := filepath.Join(f.root, string(digest)+".png")
	return ioutil.ReadFile(p)
}
