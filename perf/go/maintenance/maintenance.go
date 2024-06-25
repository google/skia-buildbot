// Package maintenance runs long running processes for a single perf instance.
package maintenance

import (
	"context"
	"time"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/builders"
	redis_cache "go.skia.org/infra/perf/go/cache/redis"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression/migration"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/tracing"
)

const (
	// How often to update the git repo from origin.
	gitRepoUpdatePeriod = time.Minute

	// How often to migrate a batch of regressions to the new table.
	regressionMigratePeriod = time.Minute

	// Size of the batch of regressions to migrate.
	regressionMigrationBatchSize = 50

	redisCacheRefreshPeriod = time.Minute
)

// Start all the long running processes. This function does not return if all
// the processes started correctly.
func Start(ctx context.Context, flags config.MaintenanceFlags, instanceConfig *config.InstanceConfig) error {
	if err := tracing.Init(flags.Local, instanceConfig); err != nil {
		return skerr.Wrapf(err, "Start tracing.")
	}

	// Migrate schema if needed.
	db, err := builders.NewCockroachDBFromConfig(ctx, instanceConfig, false)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create CockroachDB instance.")
	}
	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	if err != nil {
		return skerr.Wrapf(err, "Failed to migrate schema.")
	}

	// New perfgit.Git.
	g, err := builders.NewPerfGitFromConfig(ctx, flags.Local, instanceConfig)
	if err != nil {
		return skerr.Wrapf(err, "Build perfGit instance.")
	}
	// Start a background process that periodically adds new commits to the
	// database.
	g.StartBackgroundPolling(ctx, gitRepoUpdatePeriod)

	// Migrate regression schema if specified.
	if flags.MigrateRegressions {
		migrator, err := migration.New(ctx, db)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build regression schema migrator.")
		}
		migrator.RunPeriodicMigration(regressionMigratePeriod, regressionMigrationBatchSize)
	}

	if flags.RefreshQueryCache {
		sklog.Info("Creating Redis Client.")
		gcpClient, err := gcp_redis.NewCloudRedisClient(ctx)
		if err != nil {
			return skerr.Wrapf(err, "Cannot create Redis client for Google Cloud.")
		}
		traceStore, err := builders.NewTraceStoreFromConfig(ctx, flags.Local, instanceConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build TraceStore.")
		}

		dfBuilder := dfbuilder.NewDataFrameBuilderFromTraceStore(
			g,
			traceStore,
			2,
			dfbuilder.Filtering(instanceConfig.FilterParentTraces))
		psRefresher := psrefresh.NewParamSetRefresher(traceStore, 2, dfBuilder, instanceConfig.QueryConfig)
		err = psRefresher.Start(time.Hour)
		if err != nil {
			return skerr.Wrapf(err, "Error starting paramset refreshser.")
		}

		cache, err := redis_cache.NewRedisCache(ctx, gcpClient, &instanceConfig.QueryConfig.RedisConfig, &traceStore, flags.TilesForQueryCache)
		if err != nil {
			return skerr.Wrapf(err, "Error creating new redis cache instance")
		}
		cacheParamSetRefresher := psrefresh.NewCachedParamSetRefresher(psRefresher, cache)
		cacheParamSetRefresher.StartRefreshRoutine(ctx, redisCacheRefreshPeriod)
	}

	select {}
}
