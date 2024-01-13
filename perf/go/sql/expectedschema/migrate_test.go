package expectedschema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func Test_NoMigrationNeeded(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewCockroachDBForTests(t, "desc")

	// Newly created schema should already be up to date, so no error should pop up.
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
}

const CreateInvalidTable = `
DROP TABLE IF EXISTS Alerts;
CREATE TABLE IF NOT EXISTS Alerts (
	alert TEXT
  );
`

func Test_InvalidSchema(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(t, "desc")

	_, err := db.Exec(ctx, CreateInvalidTable)
	require.NoError(t, err)

	// Live schema doesn't match next or prev schema versions. This shouldn't happen.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)

	require.Error(t, err)
}

func Test_MigrationNeeded(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(t, "desc")

	next, err := expectedschema.Load()
	require.NoError(t, err)
	prev, err := expectedschema.LoadPrev()
	require.NoError(t, err)

	_, err = db.Exec(ctx, expectedschema.FromNextToLive)
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, sql.Tables{})
	require.NoError(t, err)
	// Current schema should now match prev.
	assertdeep.Equal(t, prev, *actual)

	// Since live matches the prev schema, it should get migrated to next.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)

	actual, err = schema.GetDescription(ctx, db, sql.Tables{})
	require.NoError(t, err)
	assertdeep.Equal(t, next, *actual)
}
