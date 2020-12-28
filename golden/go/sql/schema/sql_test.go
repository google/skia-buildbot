package schema_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestSchema_LoadIntoCockroachDB_Success(t *testing.T) {
	unittest.LargeTest(t)

	dbUrl := sqltest.MakeLocalCockroachDBForTesting(t, true /*=cleanup*/)
	ctx := context.Background()
	conf, err := pgx.ParseConfig(dbUrl)
	require.NoError(t, err)
	db, err := pgx.ConnectConfig(ctx, conf)
	require.NoError(t, err)
	_, err = db.Exec(ctx, schema.Schema)
	require.NoError(t, err)
}
