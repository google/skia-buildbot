package expectedschema_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/sql/pool"
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

const dropAllTablesAndIndices = `
  DROP INDEX IF EXISTS by_revision;
  DROP INDEX IF EXISTS by_user_id;
  DROP INDEX IF EXISTS by_tile_number;
  DROP INDEX IF EXISTS by_trace_id;
  DROP INDEX IF EXISTS by_trace_id2;
  DROP INDEX IF EXISTS by_key_value;
  DROP INDEX IF EXISTS by_alert_id;
  DROP INDEX IF EXISTS by_commit_alert;
  DROP INDEX IF EXISTS by_source_file;
  DROP INDEX IF EXISTS by_source_file_id;
  DROP INDEX IF EXISTS by_trace_id_tv2;
  DROP INDEX IF EXISTS by_commit_and_prev_commit;
  DROP INDEX IF EXISTS by_trace_id_and_commit;
  DROP INDEX IF EXISTS idx_alerts_subname;
  DROP INDEX IF EXISTS by_sub_name_creation_time;
  DROP INDEX IF EXISTS by_sub_name_triage_status_creation_time_asc;
  DROP INDEX IF EXISTS by_legacy_key;

  DROP TABLE IF EXISTS Alerts;
  DROP TABLE IF EXISTS AnomalyGroups;
  DROP TABLE IF EXISTS Autobisections;
  DROP TABLE IF EXISTS Commits;
  DROP TABLE IF EXISTS Culprits;
  DROP TABLE IF EXISTS Favorites;
  DROP TABLE IF EXISTS GraphsShortcuts;
  DROP TABLE IF EXISTS PublicTraceRules;
  DROP TABLE IF EXISTS Metadata;
  DROP TABLE IF EXISTS ParamSets;
  DROP TABLE IF EXISTS Postings;
  DROP TABLE IF EXISTS Regressions;
  DROP TABLE IF EXISTS Regressions2;
  DROP TABLE IF EXISTS RegressionsShortcuts;
  DROP TABLE IF EXISTS ReverseKeyMap;
  DROP TABLE IF EXISTS Shortcuts;
  DROP TABLE IF EXISTS SourceFiles;
  DROP TABLE IF EXISTS Subscriptions;
  DROP TABLE IF EXISTS TraceParams;
  DROP TABLE IF EXISTS TraceValues;
  DROP TABLE IF EXISTS TraceValues2;
  DROP TABLE IF EXISTS UserIssues;
  DROP TABLE IF EXISTS schema_migrations;
`

func cleanDatabase(t *testing.T, db pool.Pool) {
	ctx := context.Background()
	_, err := db.Exec(ctx, dropAllTablesAndIndices)
	require.NoError(t, err)
}

func getAppliedVersions(t *testing.T, db pool.Pool) []int {
	ctx := context.Background()
	rows, err := db.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version ASC")
	require.NoError(t, err)
	var versions []int
	for rows.Next() {
		var v int
		err := rows.Scan(&v)
		require.NoError(t, err)
		versions = append(versions, v)
	}
	return versions
}

func getExpectedVersions(t *testing.T) []int {
	entries, err := expectedschema.MigrationsFS.ReadDir("migrations")
	require.NoError(t, err)

	var versions []int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".sql")
		verStr, _, found := strings.Cut(name, "_")
		require.True(t, found, "invalid filename: %s", entry.Name())
		v, err := strconv.Atoi(verStr)
		require.NoError(t, err)
		versions = append(versions, v)
	}
	slices.Sort(versions)
	return versions
}

func Test_VersionedMigrations_FromScratch(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "v_scratch")
	cleanDatabase(t, db)

	// Since database is completely empty, it should apply all migrations (1 and 2)
	err := expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)

	applied := getAppliedVersions(t, db)
	assert.Equal(t, getExpectedVersions(t), applied)

	// Verify that the final schema matches what we expect from load.
	expectedSchema, err := expectedschema.Load()
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assertdeep.Equal(t, expectedSchema, *actual)
}

func Test_VersionedMigrations_BootstrapExisting(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "v_bootstrap")
	cleanDatabase(t, db)

	// 1. Manually apply the entire version 1 baseline schema (representing fully up-to-date pre-tracked state)
	baselineSQL, err := expectedschema.MigrationsFS.ReadFile("migrations/0001_init.sql")
	require.NoError(t, err)
	_, err = db.Exec(ctx, string(baselineSQL))
	require.NoError(t, err)

	// 2. Validate and migrate. It should:
	//    - Detect that autobisections exists (initialization completed)
	//    - Bootstrap version 1 in schema_migrations
	//    - Run migration 2 to reach target.
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)

	applied := getAppliedVersions(t, db)
	assert.Equal(t, getExpectedVersions(t), applied)

	expectedSchema, err := expectedschema.Load()
	require.NoError(t, err)

	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, string(config.SpannerDataStoreType))
	require.NoError(t, err)
	assertdeep.Equal(t, expectedSchema, *actual)
}

func Test_VerifySchemaVersion(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "verify_ver")
	cleanDatabase(t, db)

	// Initially, database is completely empty, VerifySchemaVersion should report error
	err := expectedschema.VerifySchemaVersion(ctx, db)
	require.Error(t, err)

	// After validating and migrating, it should pass
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	require.NoError(t, err)

	err = expectedschema.VerifySchemaVersion(ctx, db)
	require.NoError(t, err)
}

func TestMigrations_NoDuplicateVersions(t *testing.T) {
	// 1. Verify that the production checked-in migrations don't have duplicates.
	migrations, err := expectedschema.GetMigrations()
	require.NoError(t, err)
	require.NotEmpty(t, migrations)

	// 2. Verify that mock migrations with duplicate versions return a duplicate migration error.
	oldFS := expectedschema.DefaultMigrationsFS
	defer func() { expectedschema.DefaultMigrationsFS = oldFS }()

	expectedschema.DefaultMigrationsFS = fstest.MapFS{
		"migrations/0001_init.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"migrations/0002_test.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"migrations/0002_dup.sql":  &fstest.MapFile{Data: []byte("SELECT 2 dup;")},
	}

	_, err = expectedschema.GetMigrations()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate migration version 2 found")
}
