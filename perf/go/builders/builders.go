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
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/pool/wrapper/timeout"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/alerts/sqlalertstore"
	"go.skia.org/infra/perf/go/anomalygroup"
	ag_store "go.skia.org/infra/perf/go/anomalygroup/sqlanomalygroupstore"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit"
	culprit_store "go.skia.org/infra/perf/go/culprit/sqlculpritstore"
	"go.skia.org/infra/perf/go/favorites"
	favorite_store "go.skia.org/infra/perf/go/favorites/sqlfavoritestore"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/file/dirsource"
	"go.skia.org/infra/perf/go/file/gcssource"
	"go.skia.org/infra/perf/go/filestore/gcs"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/graphsshortcut/graphsshortcutstore"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/sqlregression2store"
	"go.skia.org/infra/perf/go/regression/sqlregressionstore"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/shortcut/sqlshortcutstore"
	"go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/subscription"
	subscription_store "go.skia.org/infra/perf/go/subscription/sqlsubscriptionstore"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	localCache "go.skia.org/infra/go/cache/local"
	redisCache "go.skia.org/infra/go/cache/redis"
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
		sklog.Warningf("pgx - %s %v", msg, data)
	case pgx.LogLevelNone:
	}
}

// maxPoolConnections is the MaxConns our pgxPool will maintain.
//
// TODO(jcgregorio) This is a guess of a good number, once later CLs land I can
// experiment with how this affects performance.
const maxPoolConnections = 300

// singletonPool is the one and only instance of pool.Pool that an
// application should have, used in NewCockroachDBFromConfig.
var singletonPool pool.Pool

// singletonPoolMutex is used to enforce the singleton nature of singletonPool,
// used in NewCockroachDBFromConfig
var singletonPoolMutex sync.Mutex

// NewCockroachDBFromConfig opens an existing CockroachDB database.
//
// No migrations are applied automatically, they must be applied by the
// 'migrate' command line application. See COCKROACHDB.md for more details.
func NewCockroachDBFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig, checkSchema bool) (pool.Pool, error) {
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
	rawPool, err := pgxpool.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Wrap the db pool in a ContentTimeout which checks that every context has
	// a timeout.
	singletonPool = timeout.New(rawPool)

	if checkSchema {
		// Confirm the database has the right schema.
		expectedSchema, err := expectedschema.Load()
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		actual, err := schema.GetDescription(ctx, singletonPool, sql.Tables{})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if diff := assertdeep.Diff(expectedSchema, *actual); diff != "" {
			return nil, skerr.Fmt("Schema needs to be updated: %s.", diff)
		}
	}

	return singletonPool, err
}

// NewPerfGitFromConfig return a new perfgit.Git for the given instanceConfig.
//
// The instance created does not poll by default, callers need to call
// StartBackgroundPolling().
func NewPerfGitFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (perfgit.Git, error) {
	if instanceConfig.DataStoreConfig.ConnectionString == "" {
		return nil, skerr.Fmt("A connection_string must always be supplied.")
	}

	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
	default:
		return nil, skerr.Fmt("Unknown datastore_type: %q", instanceConfig.DataStoreConfig.DataStoreType)
	}

	// Now create the appropriate db.
	db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
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
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
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
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
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
func NewRegressionStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig, alertsConfigProvider alerts.ConfigProvider) (regression.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if instanceConfig.UseRegression2 {
			return sqlregression2store.New(db, alertsConfigProvider)
		} else {
			return sqlregressionstore.New(db)
		}
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewShortcutStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (shortcut.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return sqlshortcutstore.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewGraphsShortcutStoreFromConfig(ctx context.Context, local bool, instanceConfig *config.InstanceConfig) (graphsshortcut.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return graphsshortcutstore.New(db)
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

// NewAnomalyGroupStoreFromConfig creates a new anomalygroup.Store from the
// InstanceConfig which provides access to the anomalygroup data.
func NewAnomalyGroupStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (anomalygroup.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return ag_store.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewCulpritStoreFromConfig creates a new culprit.Store from the
// InstanceConfig which provides access to the culprit data.
func NewCulpritStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (culprit.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return culprit_store.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewSubscriptionStoreFromConfig creates a new subscription.Store from the
// InstanceConfig which provides access to the subscription data.
func NewSubscriptionStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (subscription.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return subscription_store.New(db)
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// NewFavoriteStoreFromConfig creates a new favorites.Store from the
// InstanceConfig which provides access to the favorite data.
func NewFavoriteStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (favorites.Store, error) {
	switch instanceConfig.DataStoreConfig.DataStoreType {
	case config.CockroachDBDataStoreType:
		db, err := NewCockroachDBFromConfig(ctx, instanceConfig, true)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return favorite_store.New(db), nil
	}
	return nil, skerr.Fmt("Unknown datastore type: %q", instanceConfig.DataStoreConfig.DataStoreType)
}

// GetCacheFromConfig returns a cache.Cache instance based on the given configuration.
func GetCacheFromConfig(ctx context.Context, instanceConfig config.InstanceConfig) (cache.Cache, error) {
	var cache cache.Cache
	var err error
	switch instanceConfig.QueryConfig.CacheConfig.Type {
	case config.RedisCache:
		redisConfig := instanceConfig.QueryConfig.RedisConfig
		gcpClient, err := gcp_redis.NewCloudRedisClient(ctx)
		if err != nil {
			sklog.Fatalf("Cannot create Redis client for Google Cloud: %v", err)
		}
		cache, err = redisCache.NewRedisCache(ctx, gcpClient, &redisConfig)
		if err != nil {
			sklog.Fatalf("Error creating new redis cache: %v", err)
		}
	case config.LocalCache:
		cache, err = localCache.New(100)
		if err != nil {
			sklog.Fatalf("Error creating new local cache: %v", err)
		}
	default:
		sklog.Fatalf("Invalid cache type %s specified in config", instanceConfig.QueryConfig.CacheConfig.Type)
	}

	return cache, err
}
