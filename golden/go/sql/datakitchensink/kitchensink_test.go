package datakitchensink_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestBuild_DataIsValidAndMatchesSchema(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)

	data := datakitchensink.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, data))

	// Spot check the data.
	row := db.QueryRow(ctx, "SELECT count(*) from TraceValues")
	count := 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 180, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE corpus = $1", "round")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 15, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule = $1", true)
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule = $1", false)
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 39, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE matches_any_ignore_rule IS NULL")
	count = -1
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelPositive))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 9, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelNegative))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 4, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Expectations WHERE label = $1", string(schema.LabelUntriaged))
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 7, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Changelists")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Patchsets")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 3, count)
	row = db.QueryRow(ctx, "SELECT count(*) from Tryjobs")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 6, count)
}
