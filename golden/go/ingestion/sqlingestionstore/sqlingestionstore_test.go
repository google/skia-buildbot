package sqlingestionstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestSetIngested_WritesToDeprecatedIngestedFiles(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.SetIngested(ctx, "gcs://my-bucket/myfile.json", time.Date(2021, time.January, 7, 10, 40, 0, 0, time.UTC)))
	require.NoError(t, store.SetIngested(ctx, "s3://my-bucket/myotherfile.json", time.Date(2021, time.January, 8, 9, 10, 11, 0, time.UTC)))

	rows, err := db.Query(ctx, `SELECT * FROM DeprecatedIngestedFiles`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows []schema.DeprecatedIngestedFileRow
	for rows.Next() {
		var r schema.DeprecatedIngestedFileRow
		assert.NoError(t, rows.Scan(&r.SourceFileID, &r.SourceFile, &r.LastIngested))
		r.LastIngested = r.LastIngested.UTC() // Timezone is not set to UTC upon scanning?
		actualRows = append(actualRows, r)
	}
	assert.Equal(t, []schema.DeprecatedIngestedFileRow{{
		SourceFileID: schema.SourceFileID{0x81, 0xb3, 0xcd, 0xa8, 0x7a, 0x82, 0x33, 0x5, 0x6d, 0x3c, 0x28, 0x29, 0x23, 0x9b, 0x25, 0xfc},
		SourceFile:   "gcs://my-bucket/myfile.json",
		LastIngested: time.Date(2021, time.January, 7, 10, 40, 0, 0, time.UTC),
	}, {
		SourceFileID: schema.SourceFileID{0xa0, 0x35, 0xf3, 0x47, 0x77, 0x88, 0x4b, 0x3a, 0xfb, 0x5a, 0xa9, 0x4c, 0x54, 0xdb, 0x6d, 0xd},
		SourceFile:   "s3://my-bucket/myotherfile.json",
		LastIngested: time.Date(2021, time.January, 8, 9, 10, 11, 0, time.UTC),
	}}, actualRows)
}

func TestWasIngested_AlreadyStored_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	const testFile = "gcs://my-bucket/myfile.json"
	require.NoError(t, store.SetIngested(ctx, testFile, time.Date(2021, time.January, 7, 10, 40, 0, 0, time.UTC)))

	ok, err := store.WasIngested(ctx, testFile)
	require.NoError(t, err)
	assert.True(t, ok)

	// Should be deterministic
	ok, err = store.WasIngested(ctx, testFile)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestWasIngested_StoredLater_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	const testFile = "gcs://my-bucket/myfile.json"
	ok, err := store.WasIngested(ctx, testFile)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, store.SetIngested(ctx, testFile, time.Date(2021, time.January, 7, 10, 40, 0, 0, time.UTC)))

	ok, err = store.WasIngested(ctx, testFile)
	require.NoError(t, err)
	assert.True(t, ok)
}
