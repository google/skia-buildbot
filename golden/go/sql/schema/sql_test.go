package schema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

// This test makes sure the schema defined in this package can be processed by cockroachDB
// without error.
func TestSchema_LoadIntoCockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)

	_, err := db.Exec(ctx, schema.Schema)
	require.NoError(t, err)
}
