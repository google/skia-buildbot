package expectedschema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/expectedschema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func Test_NoMigrationNeeded_Spanner(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewSpannerDBForTests(ctx, t)

	// Newly created schema should already be up to date, so no error should pop up.
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.Spanner)
	require.NoError(t, err)
}

const CreateInvalidTableSpanner = `
DROP TABLE IF EXISTS Changelists;
CREATE TABLE IF NOT EXISTS Changelists (
	alert TEXT PRIMARY KEY
  );
`

func Test_InvalidSchema_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)

	_, err := db.Exec(ctx, CreateInvalidTableSpanner)
	require.NoError(t, err)

	// Live schema doesn't match next or prev schema versions. This shouldn't happen.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.Spanner)

	require.Error(t, err)
}

/*
// TODO(pasthana): Uncomment once https://skia-review.googlesource.com/c/buildbot/+/992260
// is merged, and schema.jsons have been validated to be equal to current
// prod db
func Test_MigrationNeeded_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(ctx, t)

	next, err := expectedschema.Load(config.Spanner)
	require.NoError(t, err)
	prev, err := expectedschema.LoadPrev(config.Spanner)
	require.NoError(t, err)

	_, err = db.Exec(ctx, expectedschema.FromNextToLiveSpanner)
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, golden_schema.Tables{}, string(config.Spanner))
	require.NoError(t, err)
	// Current schema should now match prev.
	assertdeep.Equal(t, prev, *actual)

	// Since live matches the prev schema, it should get migrated to next.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.Spanner)
	require.NoError(t, err)

	actual, err = schema.GetDescription(ctx, db, golden_schema.Tables{}, string(config.Spanner))
	require.NoError(t, err)
	assertdeep.Equal(t, next, *actual)
}
*/
