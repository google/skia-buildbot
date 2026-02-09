package expectedschema_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{})
	require.NoError(t, err)
}

const CreateInvalidTableSpanner = `
DROP INDEX IF EXISTS idx_alerts_subname;
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
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.Error(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{})
	require.NoError(t, err)
}

func Test_MigrationNeeded_Spanner(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "desc")

	next, err := expectedschema.Load()
	require.NoError(t, err)
	prev, err := expectedschema.LoadPrev()
	require.NoError(t, err)

	_, err = db.Exec(ctx, expectedschema.FromNextToLiveSpanner)
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	// Current schema should now match prev.
	assertdeep.Equal(t, prev, *actual)

	// Since live matches the prev schema, it should get migrated to next.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{})
	require.NoError(t, err)

	actual, err = schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assertdeep.Equal(t, next, *actual)
}

func Test_TraceParamsAddColAndIdx_Spanner(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewSpannerDBForTests(t, "desc")

	// Insert a tilenumber and paramset to generate "bot" col from:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'bot', 'win-10-perf' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	require.NoError(t, err)

	// Specify that we should index traceparams on "bot":
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{"bot"})
	require.NoError(t, err)

	actualCols, actualIdxs, err := expectedschema.GetTraceParamsGeneratedColsAndIdxs(ctx, db, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assert.Equal(t, 1, len(actualCols))
	assert.Equal(t, "bot", actualCols[0])
	assert.Equal(t, 1, len(actualIdxs))
	assert.Equal(t, "idx_traceparams_bot", actualIdxs[0])
}

func Test_TraceParamsRemoveColAndIdx_Spanner(t *testing.T) {
	ctx := context.Background()
	// Load DB loaded with schema from schema.go
	db := sqltest.NewSpannerDBForTests(t, "desc")

	// Insert a tilenumber and paramset to generate "bot" col from:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'bot', 'win-10-perf' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	require.NoError(t, err)

	// Specify that we should index traceparams on "bot":
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{"bot"})
	require.NoError(t, err)

	// Remove all paramsets so no columns are generated for traceparams:
	dropFromParamsets := `DELETE FROM ParamSets`
	_, err = db.Exec(ctx, dropFromParamsets)
	require.NoError(t, err)
	// Specify that we should NO LONGER index traceparams on "bot":
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)
	err = expectedschema.UpdateTraceParamsSchema(ctx, db, config.SpannerDataStoreType, []string{})
	require.NoError(t, err)

	actualCols, actualIdxs, err := expectedschema.GetTraceParamsGeneratedColsAndIdxs(ctx, db, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assert.Equal(t, 0, len(actualCols))
	assert.Equal(t, 0, len(actualIdxs))
}
