package expectedschema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/expectedschema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

/*
// TODO(pasthana): Uncomment once https://skia-review.googlesource.com/c/buildbot/+/992260
// is merged, and schema.jsons have been validated to be equal to current
// prod db
func Test_NoMigrationNeeded(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewCockroachDBForTests(ctx, t)

	// Newly created schema should already be up to date, so no error should pop up.
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.CockroachDB)
	require.NoError(t, err)
}
*/

const CreateInvalidTable = `
DROP TABLE IF EXISTS Changelists;
CREATE TABLE IF NOT EXISTS Changelists (
	alert TEXT
  );
`

func Test_InvalidSchema(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)

	_, err := db.Exec(ctx, CreateInvalidTable)
	require.NoError(t, err)

	// Live schema doesn't match next or prev schema versions. This shouldn't happen.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.CockroachDB)

	require.Error(t, err)
}

/*
// TODO(pasthana): Uncomment once https://skia-review.googlesource.com/c/buildbot/+/992260
// is merged, and schema.jsons have been validated to be equal to current
// prod db
func Test_MigrationNeeded(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)

	next, err := expectedschema.Load("cockroachdb")
	require.NoError(t, err)
	prev, err := expectedschema.LoadPrev("cockroachdb")
	require.NoError(t, err)

	_, err = db.Exec(ctx, expectedschema.FromNextToLive)
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, golden_schema.Tables{}, string(config.CockroachDB))
	require.NoError(t, err)
	// Current schema should now match prev.
	assertdeep.Equal(t, prev, *actual)

	// Since live matches the prev schema, it should get migrated to next.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db, config.CockroachDB)
	require.NoError(t, err)

	actual, err = schema.GetDescription(ctx, db, golden_schema.Tables{}, string(config.CockroachDB))
	require.NoError(t, err)
	assertdeep.Equal(t, next, *actual)
}
*/
