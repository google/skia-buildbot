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
	localfilestore "go.skia.org/infra/perf/go/filestore/local"
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
	"go.skia.org/infra/perf/go/userissue"
	userissue_store "go.skia.org/infra/perf/go/userissue/sqluserissuestore"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	"go.skia.org/infra/go/cache/local"
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
// application should have, used in NewDBPoolFromConfig.
var singletonPool pool.Pool

// singletonPoolMutex is used to enforce the singleton nature of singletonPool,
// used in NewDBPoolFromConfig
var singletonPoolMutex sync.Mutex

// NewDBPoolFromConfig opens an existing database.
func NewDBPoolFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig, checkSchema bool) (pool.Pool, error) {
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
	if instanceConfig.DataStoreConfig.MinimumConnectionsInDBPool == 0 {
		// Default to at least 2 connections.
		instanceConfig.DataStoreConfig.MinimumConnectionsInDBPool = 2
	}
	cfg.MinConns = instanceConfig.DataStoreConfig.MinimumConnectionsInDBPool
	sklog.Infof("Specified min db connections: %d", cfg.MinConns)
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

		actual, err := schema.GetDescription(ctx, singletonPool, sql.Tables{}, string(instanceConfig.DataStoreConfig.DataStoreType))
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
func NewPerfGitFromConfig(ctx context.Context, localToProd bool, instanceConfig *config.InstanceConfig) (perfgit.Git, error) {
	if instanceConfig.DataStoreConfig.ConnectionString == "" {
		return nil, skerr.Fmt("A connection_string must always be supplied.")
	}

	// Now create the appropriate db.
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	g, err := perfgit.New(ctx, localToProd, db, instanceConfig)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return g, nil
}

// NewTraceStoreFromConfig creates a new TraceStore from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewTraceStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (tracestore.TraceStore, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	traceParamStore, err := NewTraceParamStore(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	var inMemoryTraceParams *sqltracestore.InMemoryTraceParams = nil
	inMemoryTraceParams, err = sqltracestore.NewInMemoryTraceParams(ctx, db, 12*60*60 /*12hr*/)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return sqltracestore.New(db, instanceConfig.DataStoreConfig, traceParamStore, inMemoryTraceParams)
}

// NewMetadataStoreFromConfig creates a new MetadataStore from the InstanceConfig.
func NewMetadataStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (tracestore.MetadataStore, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}

	return sqltracestore.NewSQLMetadataStore(db), nil
}

// NewTraceParamStore returns a new TraceParamStore from the instance config.
func NewTraceParamStore(ctx context.Context, instanceConfig *config.InstanceConfig) (tracestore.TraceParamStore, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}

	return sqltracestore.NewTraceParamStore(db), nil
}

// NewAlertStoreFromConfig creates a new alerts.Store from the InstanceConfig.
func NewAlertStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (alerts.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return sqlalertstore.New(db)
}

// NewRegressionStoreFromConfig creates a new regression.RegressionStore from
// the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewRegressionStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig, alertsConfigProvider alerts.ConfigProvider) (regression.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}

	if instanceConfig.UseRegression2 {
		return sqlregression2store.New(db, alertsConfigProvider, instanceConfig)
	} else {
		return sqlregressionstore.New(db)
	}
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewShortcutStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (shortcut.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return sqlshortcutstore.New(db)
}

// NewShortcutStoreFromConfig creates a new shortcut.Store from the
// InstanceConfig.
func NewGraphsShortcutStoreFromConfig(ctx context.Context, localToProd bool, instanceConfig *config.InstanceConfig) (graphsshortcut.Store, error) {
	if localToProd {
		cache, err := local.New(100)
		if err != nil {
			return nil, err
		}
		return graphsshortcutstore.NewCacheGraphsShortcutStore(cache), nil
	}
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return graphsshortcutstore.New(db)
}

// NewSourceFromConfig creates a new file.Source from the InstanceConfig.
//
// If local is true then we aren't running in production.
func NewSourceFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (file.Source, error) {
	switch instanceConfig.IngestionConfig.SourceConfig.SourceType {
	case config.GCSSourceType:
		return gcssource.New(ctx, instanceConfig)
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
func NewIngestedFSFromConfig(ctx context.Context, cfg *config.InstanceConfig) (fs.FS, error) {
	switch cfg.IngestionConfig.SourceConfig.SourceType {
	case config.GCSSourceType:
		return gcs.New(ctx)
	case config.DirSourceType:
		return localfilestore.New(cfg.IngestionConfig.SourceConfig.Sources[0])
	}
	// We currently default to Google Cloud Storage, but Config options could be
	// added to use other systems, such as S3.
	return gcs.New(ctx)
}

// NewAnomalyGroupStoreFromConfig creates a new anomalygroup.Store from the
// InstanceConfig which provides access to the anomalygroup data.
func NewAnomalyGroupStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (anomalygroup.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return ag_store.New(db)
}

// NewCulpritStoreFromConfig creates a new culprit.Store from the
// InstanceConfig which provides access to the culprit data.
func NewCulpritStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (culprit.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return culprit_store.New(db)
}

// NewSubscriptionStoreFromConfig creates a new subscription.Store from the
// InstanceConfig which provides access to the subscription data.
func NewSubscriptionStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (subscription.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return subscription_store.New(db)
}

// NewFavoriteStoreFromConfig creates a new favorites.Store from the
// InstanceConfig which provides access to the favorite data.
func NewFavoriteStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (favorites.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return favorite_store.New(db), nil
}

// NewUserIssueStoreFromConfig creates a new userissue.Store from the
// InstanceConfig which provides access to the userissue data.
func NewUserIssueStoreFromConfig(ctx context.Context, instanceConfig *config.InstanceConfig) (userissue.Store, error) {
	db, err := getDBPool(ctx, instanceConfig)
	if err != nil {
		return nil, err
	}
	return userissue_store.New(db), nil
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

// getDBPool returns a pool.Pool object based on the target database configured.
func getDBPool(ctx context.Context, instanceConfig *config.InstanceConfig) (pool.Pool, error) {
	db, err := NewDBPoolFromConfig(ctx, instanceConfig, true)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return db, nil
}
