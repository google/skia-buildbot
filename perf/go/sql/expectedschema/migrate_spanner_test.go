package expectedschema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func Test_NoMigrationNeeded_Spanner(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewSpannerDBForTests(t, "desc")

	// Newly created schema should already be up to date, so no error should pop up.
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.SpannerDataStoreType)
	require.NoError(t, err)
}

const CreateInvalidTableSpanner = `
DROP TABLE IF EXISTS Alerts;
CREATE TABLE IF NOT EXISTS Alerts (
	alert TEXT PRIMARY KEY
  );
`

func Test_InvalidSchema_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "desc")

	_, err := db.Exec(ctx, CreateInvalidTableSpanner)
	require.NoError(t, err)

	// Live schema doesn't match next or prev schema versions. This shouldn't happen.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.SpannerDataStoreType)

	require.Error(t, err)
}

func Test_MigrationNeeded_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "desc")

	next, err := expectedschema.Load(config.SpannerDataStoreType)
	require.NoError(t, err)
	prev, err := expectedschema.LoadPrev(config.SpannerDataStoreType)
	require.NoError(t, err)

	_, err = db.Exec(ctx, expectedschema.FromNextToLive)
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	// Current schema should now match prev.
	assertdeep.Equal(t, prev, *actual)

	// Since live matches the prev schema, it should get migrated to next.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.SpannerDataStoreType)
	require.NoError(t, err)

	actual, err = schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assertdeep.Equal(t, next, *actual)
}
