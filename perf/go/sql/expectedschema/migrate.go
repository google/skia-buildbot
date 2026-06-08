// This file contains useful logic for maintenance tasks to migrate new schema
// changes.
//
// Maintenance tasks (see //perf/go/maintenance/maintenance.go) use the same
// executable as the perf frontend and ingesters, so when Louhi does an update
// all of them will be deployed at the same time. Since frontend and ingesters
// check for a correct schema they will panic on startup, so the old
// instances of those apps will continue to run.
//
// The maintenance task will run and thus will upgrade the schema in the
// database. After that completes when frontend and ingesters are retried
// (k8s does that automatically), then they will start successfully. This
// means that any change in schema must be compatible with both the current
// and previous version of the perfserver executable.
package expectedschema

import (
	"bytes"
	"context"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql"
)

type migration struct {
	version int
	name    string
	sql     string
}

func getMigrations() ([]migration, error) {
	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read migrations directory")
	}

	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		// Expected file format: migrations/<version>_<name>.sql, e.g. migrations/0002_test.sql
		name := strings.TrimSuffix(entry.Name(), ".sql")
		verStr, _, found := strings.Cut(name, "_")
		if !found {
			return nil, skerr.Fmt("invalid migration filename format: %s", entry.Name())
		}
		version, err := strconv.Atoi(verStr)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse version from filename: %s", entry.Name())
		}
		content, err := MigrationsFS.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to read migration file: %s", entry.Name())
		}
		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(content),
		})
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	// Validate sequentiality starting from 1
	for i, m := range migrations {
		if m.version != i+1 {
			return nil, skerr.Fmt("migrations are not sequential: expected version %d, got %d", i+1, m.version)
		}
	}

	return migrations, nil
}

func tableExists(ctx context.Context, db pool.Pool, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
		);
	`
	err := db.QueryRow(ctx, query, tableName).Scan(&exists)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	return exists, nil
}

// isInitCompleted detects if the database initialization has already been
// completed before the schema_migrations tracking was introduced.
func isInitCompleted(ctx context.Context, db pool.Pool) (bool, error) {
	// If autobisections table exists, the database was already fully initialized
	// prior to version tracking.
	return tableExists(ctx, db, "autobisections")
}

// GetCurrentVersion returns the currently applied schema version.
// If the schema_migrations table is empty, it attempts bootstrapping.
func GetCurrentVersion(ctx context.Context, db pool.Pool) (int, error) {
	// 1. Query the maximum applied version from the table.
	var currentVersion int
	query := `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`
	row := db.QueryRow(ctx, query)
	if err := row.Scan(&currentVersion); err != nil {
		return 0, skerr.Wrapf(err, "failed to query current schema version")
	}

	// 2. If the table is empty, check if the database was already initialized
	// before version tracking was introduced, and bootstrap version 1.
	if currentVersion == 0 {
		sklog.Infof("Table 'schema_migrations' is empty. Checking if database is already initialized...")
		initCompleted, err := isInitCompleted(ctx, db)
		if err != nil {
			return 0, skerr.Wrapf(err, "failed to check if database is initialized")
		}
		if initCompleted {
			sklog.Infof("Database was already initialized. Bootstrapping version 1 in migrations history.")
			_, err = db.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT (version) DO NOTHING`)
			if err != nil {
				return 0, skerr.Wrapf(err, "failed to bootstrap migration version 1")
			}
			return 1, nil
		}
	}

	return currentVersion, nil
}

// ValidateAndMigrateNewSchema will check whether there's new schemas checked-in,
// and if so, migrate the schema in the given Spanner instance using versioned SQL scripts.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool) error {
	sklog.Infof("Starting ValidateAndMigrateNewSchema. Ready to inspect schema transitions.")

	err := func() error {
		conn, err := db.Acquire(ctx)
		if err != nil {
			return skerr.Wrapf(err, "failed to acquire database connection")
		}
		defer conn.Release()

		// Cloud Spanner running via PGAdapter requires autocommit mode for implicit DDL
		// transactions inside the migration sequence to avoid SQLSTATE 25000 errors.
		_, err = conn.Exec(ctx, "SET spanner.ddl_transaction_mode = 'AutocommitImplicitTransaction'")
		if err != nil {
			return skerr.Wrapf(err, "failed to set ddl_transaction_mode")
		}

		// 1. Ensure the schema_migrations table exists in the database.
		_, err = conn.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INT PRIMARY KEY,
				applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
			);
		`)
		if err != nil {
			return skerr.Wrapf(err, "failed to create schema_migrations table")
		}
		sklog.Infof("Successfully checked or created 'schema_migrations' logging table.")

		// 2. Retrieve the currently applied version from the database (supporting bootstrapping).
		currentVersion, err := GetCurrentVersion(ctx, db)
		if err != nil {
			return skerr.Wrapf(err, "failed to get current or legacy schema version")
		}

		// 3. Load, parse, sort, and validate all migration files from embed.FS.
		migrations, err := getMigrations()
		if err != nil {
			return skerr.Wrapf(err, "failed to load migrations")
		}
		sklog.Infof("Loaded %d total embedded migration scripts from migrations/ package path.", len(migrations))

		// 4. Run the pending migrations step-by-step.
		appliedAny := false
		for _, m := range migrations {
			if m.version > currentVersion {
				sklog.Infof("Applying pending database migration script: Version=%d, File=%s", m.version, m.name)
				sklog.Infof("Executing SQL DDL content for migration %s:\n%s", m.name, m.sql)
				_, err = conn.Exec(ctx, m.sql)
				if err != nil {
					return skerr.Wrapf(err, "failed to apply migration version %d (%s)", m.version, m.name)
				}
				_, err = conn.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version)
				if err != nil {
					return skerr.Wrapf(err, "failed to record migration version %d as applied", m.version)
				}
				currentVersion = m.version
				sklog.Infof("Successfully upgraded database schema to version %d.", currentVersion)
				appliedAny = true
			}
		}

		if !appliedAny {
			sklog.Infof("Database schema is already up to date at version %d. No pending migrations to execute.", currentVersion)
		}
		return nil
	}()
	if err != nil {
		return err
	}

	// 5. Perform final schema check to ensure the actual schema description matches the targets.
	sklog.Infof("Verifying final database schema description matches expected target 'schema_spanner.json'...")
	expectedSchema, err := Load()
	if err != nil {
		return skerr.Wrap(err)
	}
	actual, err := schema.GetDescription(ctx, db, sql.Tables{}, "spanner")
	if err != nil {
		return skerr.Wrap(err)
	}
	if diff := assertdeep.Diff(expectedSchema, *actual); diff != "" {
		return skerr.Fmt("after migration, schema description does not match schema_spanner.json: %s", diff)
	}
	sklog.Infof("Success! Verified that the live database catalog layout matches expected target.")

	return nil
}

func Difference(as []string, bs []string) ([]string, []string) {
	var aNotB, bNotA []string
	for _, a := range as {
		if !slices.Contains(bs, a) {
			aNotB = append(aNotB, a)
		}
	}
	for _, b := range bs {
		if !slices.Contains(as, b) {
			bNotA = append(bNotA, b)
		}
	}
	return aNotB, bNotA
}

type TraceParamsUpdateContext struct {
	AddCols  []string
	DropCols []string
	AddIdxs  []string
	DropIdxs []string
}

var traceParamsUpdateTemplate = `
{{ range $i, $idx := .DropIdxs -}}
DROP INDEX {{ $idx }};
{{ end -}}

{{ range $i, $col := .DropCols -}}
ALTER TABLE traceparams
	DROP COLUMN {{ $col }};
{{ end -}}

{{ range $i, $col := .AddCols -}}
ALTER TABLE traceparams
	ADD COLUMN {{ $col }} character varying GENERATED ALWAYS AS (((params ->> '{{ $col }}'::text))::character varying) VIRTUAL;
{{ end -}}

{{ range $i, $idx := .AddIdxs -}}
CREATE INDEX idx_traceparams_{{ $idx }} ON traceparams ({{ $idx }});
{{ end -}}
 `

// This function updates the traceparams table schema so that it has generated columns
// for each param key in the paramsets table (last 2 tiles) and indexes for each column
// named in the instance config, and will drop any surplus columns/indexes it finds in the
// schema.
func UpdateTraceParamsSchema(ctx context.Context, db pool.Pool, datastoreType config.DataStoreType, traceParamsIndexes []string) error {
	sklog.Debugf("Starting update traceparams schema. DatastoreType: %s", datastoreType)

	requiredCols, err := GetParams(ctx, db, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}
	requiredIdxs := traceParamsIndexes
	actualCols, actualIdxs, err := GetTraceParamsGeneratedColsAndIdxs(ctx, db, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}
	extraCols, missingCols := Difference(actualCols, requiredCols)
	extraIdxs, missingIdxs := Difference(actualIdxs, requiredIdxs)
	if len(extraCols) == 0 && len(missingCols) == 0 &&
		len(extraIdxs) == 0 && len(missingIdxs) == 0 {
		sklog.Debugf("No changes needed for traceparams generated columns/indexes.")
		return nil
	}
	context := TraceParamsUpdateContext{
		AddCols: missingCols, DropCols: extraCols,
		AddIdxs: missingIdxs, DropIdxs: extraIdxs}
	t, err := template.New("").Parse(traceParamsUpdateTemplate)
	if err != nil {
		return skerr.Wrapf(err, "parsing template %q", traceParamsUpdateTemplate)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, context); err != nil {
		sklog.Errorf("Failed to execute template=%v, context=%v.", t, context)
		return skerr.Wrapf(err, "Failed to migrate Schema")
	}
	traceParamsUpdateStatement := b.String()
	_, err = db.Exec(ctx, traceParamsUpdateStatement)
	if err != nil {
		sklog.Errorf("Failed to update traceparams schema (statement='%s')", traceParamsUpdateStatement)
		return skerr.Wrapf(err, "Failed to migrate Schema")
	}

	return nil
}

// VerifySchemaVersion checks if the database schema version matches the expected version.
func VerifySchemaVersion(ctx context.Context, db pool.Pool) error {
	// 1. Get maximum embedded version from migrations directory.
	migrations, err := getMigrations()
	if err != nil {
		return skerr.Wrapf(err, "failed to load migrations")
	}
	if len(migrations) == 0 {
		return skerr.Fmt("no embedded migration files found")
	}
	expectedVersion := migrations[len(migrations)-1].version

	// 2. Retrieve the currently applied version from the DB.
	var currentVersion int
	query := `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`
	row := db.QueryRow(ctx, query)
	if err := row.Scan(&currentVersion); err != nil {
		return skerr.Wrapf(err, "failed to query current schema version (is the database initialized?)")
	}

	// 3. Compare version numbers.
	if currentVersion < expectedVersion {
		sklog.Infof("Readiness Check: Waiting for database schema migration from version %d to %d...", currentVersion, expectedVersion)
		return skerr.Fmt("schema needs to be updated: current version %d is behind expected version %d", currentVersion, expectedVersion)
	}

	return nil
}
