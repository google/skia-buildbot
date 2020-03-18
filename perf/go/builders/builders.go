// Package builders builds objects from config.InstanceConfig objects.
//
// These are functions separate from config.InstanceConfig so that we don't end
// up with cyclical import issues.
package builders

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/alerts/dsalertstore"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/file/dirsource"
	"go.skia.org/infra/perf/go/file/gcssource"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/dsregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/dsshortcutstore"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
	"go.skia.org/infra/perf/go/sql/migrations/sqlite3"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/btts"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
)

// newSQLite3DBFromConfig opens an existing, or creates a new, sqlite3 database
// with all migrations applied.
func newSQLite3DBFromConfig(instanceConfig *config.InstanceConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", instanceConfig.DataStoreConfig.ConnectionString)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	migrationsConnection := fmt.Sprintf("sqlite3://%s", instanceConfig.DataStoreConfig.ConnectionString)
	sqlite3Migrations, err := sqlite3.New()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	err = migrations.Up(sqlite3Migrations, migrationsConnection)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return db, nil
}

// newCockroachDBFromConfig opens an existing CockroachDB database with all
// migrations applied.
func newCockroachDBFromConfig(instanceConfig *config.InstanceConfig) (*sql.DB, error) {
	// Note that the migrationsConnection is different from the sql.Open
	// connection string since migrations know about CockroachDB, but we use the
	// Postgres driver for the database/sql connection since there's no native
	// CockroachDB golang driver, and the suggested SQL drive for CockroachDB is
	// the Postgres driver since that's the underlying communication protocol it
	// uses.
	migrationsConnection := strings.Replace(instanceConfig.DataStoreConfig.ConnectionString, "postgresql://", "cockroach://", 1)

	db, err := sql.Open("postgres", instanceConfig.DataStoreConfig.ConnectionString)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cockroachdbMigrations, err := cockroachdb.New()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	err = migrations.Up(cockroachdbMigrations, migrationsConnection)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return db, nil
}

// NewTraceStoreFromConfig creates a new TraceStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewTraceStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (tracestore.TraceStore, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.GCPDataStoreType:
		ts, err := auth.NewDefaultTokenSource(local, bigtable.Scope)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		traceStore, err := btts.NewBigTableTraceStoreFromConfig(ctx, instanceConfig, ts, false)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to open BigTable trace store.")
		}
		return traceStore, nil
	case config.SQLite3DataStoreType:
		db, err := newSQLite3DBFromConfig(instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqltracestore.New(db, perfsql.SQLiteDialect, instanceConfig.DataStoreConfig.TileSize)
	case config.CockroachDBDataStoreType:
		db, err := newCockroachDBFromConfig(instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqltracestore.New(db, perfsql.CockroachDBDialect, instanceConfig.DataStoreConfig.TileSize)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewAlertStoreFromConfig creates a new alerts.Store from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewAlertStoreFromConfig(local bool, cfg *config.InstanceConfig) (alerts.Store, error) {
	if local {
		// Should we forcibly change the namespace?
	}
	return dsalertstore.New(), nil
}

// NewRegressionStoreFromConfig creates a new regression.RegressionStore from
// the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewRegressionStoreFromConfig(local bool, cidl *cid.CommitIDLookup, cfg *config.InstanceConfig) (regression.Store, error) {
	lookup := func(ctx context.Context, c *cid.CommitID) (*cid.CommitDetail, error) {
		details, err := cidl.Lookup(ctx, []*cid.CommitID{c})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return details[0], nil
	}
	return dsregressionstore.NewRegressionStoreDS(lookup), nil
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewShortcutStoreFromConfig(cfg *config.InstanceConfig) (shortcut.Store, error) {
	return dsshortcutstore.New(), nil
}

// NewSourceFromConfig creates a new file.Source from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewSourceFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig, local bool) (file.Source, error) {
	switch instanceConfig.IngestionConfig.SourceConfig.SourceType {
	case config.GCSSourceType:
		return gcssource.New(ctx, instanceConfig, local)
	case config.DirSourceType:
		n := len(instanceConfig.IngestionConfig.SourceConfig.Sources)
		if n != 1 {
			return nil, skerr.Fmt("For a source_type of 'dir' there must be a single entry for 'sources', found %d.", n)
		}
		return dirsource.New(instanceConfig.IngestionConfig.SourceConfig.Sources[0])
	default:
		return nil, skerr.Fmt("Unknown source_type: %q", instanceConfig.IngestionConfig.SourceConfig.SourceType)
	}
}
