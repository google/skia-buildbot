// Package builders builds objects from config.InstanceConfig objects.
//
// These are functions separate from config.InstanceConfig so that we don't end
// up with cyclical import issues.
package builders

import (
	"context"
	"io/fs"
	"sync"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib" // pgx Go sql
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/alerts/sqlalertstore"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/file/dirsource"
	"go.skia.org/infra/perf/go/file/gcssource"
	"go.skia.org/infra/perf/go/filestore/gcs"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/sqlshortcutstore"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
)

// pgxLogAdaptor allows bubbling pgx logs up into our application.
type pgxLogAdaptor struct{}

// Log a message at the given level with data key/value pairs. data may be nil.
func (pgxLogAdaptor) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	switch level {
	case pgx.LogLevelTrace:
	case pgx.LogLevelDebug:
	case pgx.LogLevelInfo:
	case pgx.LogLevelWarn:
		sklog.Warningf("pgx - %s %v", msg, data)
	case pgx.LogLevelError:
		sklog.Errorf("pgx - %s %v", msg, data)
	case pgx.LogLevelNone:
	}
}

// maxPoolConnections is the MaxConns our pgxPool will maintain.
//
// TODO(jcgregorio) This is a guess of a good number, once later CLs land I can
// experiment with how this affects performance.
const maxPoolConnections = 300

// singletonPool is the one and only instance of *pgxpool.Pool that an
// application should have, used in newCockroachDBFromConfig.
var singletonPool *pgxpool.Pool

// singletonPoolMutex is used to enforce the singleton nature of singletonPool,
// used in newCockroachDBFromConfig
var singletonPoolMutex sync.Mutex

// newCockroachDBFromConfig opens an existing CockroachDB database.
//
// No migrations are applied automatically, they must be applied by the
// 'migrate' command line application. See COCKROACHDB.md for more details.
func newCockroachDBFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (*pgxpool.Pool, error) {
	singletonPoolMutex.Lock()
	defer singletonPoolMutex.Unlock()

	if singletonPool != nil {
		return singletonPool, nil
	}

	cfg, err := pgxpool.ParseConfig(instanceConfig.DataStoreConfig.ConnectionString)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse database config: %q", instanceConfig.DataStoreConfig.ConnectionString)
	}

	sklog.Infof("%#v", *cfg)
	cfg.MaxConns = maxPoolConnections
	cfg.ConnConfig.Logger = pgxLogAdaptor{}
	singletonPool, err = pgxpool.ConnectConfig(ctx, cfg)
	return singletonPool, err
}

// NewPerfGitFromConfig return a new perfgit.Git for the given instanceConfig.
//
// The instance created does not poll by default, callers need to call
// StartBackgroundPolling().
func NewPerfGitFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (*perfgit.Git, error) {
	if instanceConfig.DataStoreConfig.ConnectionString == "" {
		return nil, skerr.Fmt("A connection_string must always be supplied.")
	}

	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
	default:
		return nil, skerr.Fmt("Unknown datastore_type: %q", instanceConfig.DataStoreConfig.DataStoreType)
	}

	// Now create the appropriate db.
	db, err := newCockroachDBFromConfig(ctx, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	g, err := perfgit.New(ctx, local, db, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return g, nil
}

// NewTraceStoreFromConfig creates a new TraceStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewTraceStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (tracestore.TraceStore, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := newCockroachDBFromConfig(ctx, instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqltracestore.New(db, instanceConfig.DataStoreConfig)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewAlertStoreFromConfig creates a new alerts.Store from the InstanceConfig.
func NewAlertStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (alerts.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := newCockroachDBFromConfig(ctx, instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqlalertstore.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewRegressionStoreFromConfig creates a new regression.RegressionStore from
// the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewRegressionStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (regression.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := newCockroachDBFromConfig(ctx, instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqlregressionstore.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewShortcutStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (shortcut.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := newCockroachDBFromConfig(ctx, instanceConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqlshortcutstore.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
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

// NewIngestedFSFromConfig creates a new fs.FS from the InstanceConfig which
// provides access to ingested files.
//
// If local is true then we aren't running in production.
func NewIngestedFSFromConfig(ctx context.Context, _ *config.InstanceConfig, local bool) (fs.FS, error) {
	// We currently default to Google Cloud Storage, but Config options could be
	// added to use other systems, such as S3.
	return gcs.New(ctx, local)
}
