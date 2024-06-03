// Package maintenance runs long running processes for a single perf instance.
package maintenance

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/redis"
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

	redisCacheRefreshPeriod = time.Minute * 30
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
		redisClient, err := redis.NewRedisClient(ctx)
		if err != nil {
			return skerr.Wrapf(err, "Failed to create Redis client.")
		}
		err = redisClient.StartRefreshRoutine(ctx, redisCacheRefreshPeriod, &instanceConfig.QueryConfig.RedisConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to execute the Redis refresh routine.")
		}
	}

	select {}
}
