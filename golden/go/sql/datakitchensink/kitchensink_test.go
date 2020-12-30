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

	_, err := db.Exec(ctx, schema.Schema)
	require.NoError(t, err)

	data := datakitchensink.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, data))

	// Spot check the data.
	row := db.QueryRow(ctx, "SELECT count(*) from TraceValues")
	count := 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)

	row = db.QueryRow(ctx, "SELECT count(*) from Traces WHERE corpus = $1", "corpus_one")
	count = 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 3, count)
}
