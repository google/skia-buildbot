// Package maintenance runs long running processes for a single perf instance.
package maintenance

import (
	"context"
	"time"

	"go.skia.org/infra/go/luciconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/maintenance/deletion"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression/migration"
	sheriffconfig "go.skia.org/infra/perf/go/sheriffconfig/service"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/tracing"
)

const (
	// How often to update the git repo from origin.
	gitRepoUpdatePeriod = time.Minute

	// How often to migrate a batch of regressions to the new table.
	regressionMigratePeriod = time.Minute

	// How often to poll LUCI Config for new config changes.
	configImportPeriod = time.Minute * 10

	// Size of the batch of regressions to migrate.
	regressionMigrationBatchSize = 50

	// Time interval for refreshing the redis cache.
	redisCacheRefreshPeriod = time.Hour * 4

	// How often to delete a batch of old shortcuts and regressions.
	deletionPeriod = time.Minute * 15

	// Size of the batch of shortcuts to delete.
	deletionBatchSize = 1000
)

// Start all the long running processes. This function does not return if all
// the processes started correctly.
func Start(ctx context.Context, flags config.MaintenanceFlags, instanceConfig *config.InstanceConfig) error {
	if err := tracing.Init(flags.Local, instanceConfig); err != nil {
		return skerr.Wrapf(err, "Start tracing.")
	}

	// Migrate schema if needed.
	db, err := builders.NewDBPoolFromConfig(ctx, instanceConfig, false)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create Spanner instance.")
	}

	err = expectedschema.ValidateAndMigrateNewSchema(ctx, db)
	if err != nil {
		return skerr.Wrapf(err, "Failed to migrate schema.")
	}

	if flags.GenerateTraceParamsAdditions {
		var traceParamsIndexes []string
		if instanceConfig != nil {
			traceParamsIndexes = instanceConfig.DataStoreConfig.TraceParamsParamIndexes
		}
		err = expectedschema.UpdateTraceParamsSchema(ctx, db, instanceConfig.DataStoreConfig.DataStoreType, traceParamsIndexes)
		if err != nil {
			return skerr.Wrapf(err, "Failed to update traceparams schema.")
		}
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
		migrator, err := migration.New(ctx, db, instanceConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build regression schema migrator.")
		}
		migrator.RunPeriodicMigration(regressionMigratePeriod, regressionMigrationBatchSize)
	}

	if instanceConfig.EnableSheriffConfig && instanceConfig.InstanceName != "" {

		alertStore, err := builders.NewAlertStoreFromConfig(ctx, instanceConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build AlertStore.")
		}
		subscriptionStore, err := builders.NewSubscriptionStoreFromConfig(ctx, instanceConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build SubscriptionStore.")
		}
		luciConfig, err := luciconfig.NewApiClient(ctx, false)
		if err != nil {
			sklog.Errorf("Failed to build LUCI Config client: %s", err)
			// TODO(eduardoyap): Move this out of the else block. For now it's just to prevent the
			// service from crashing if we're unable to connect.
		} else {
			sheriffConfig, err := sheriffconfig.New(ctx, db, subscriptionStore, alertStore, luciConfig, instanceConfig.InstanceName)
			if err != nil {
				return skerr.Wrapf(err, "Error starting sheriff config service.")
			}
			sheriffConfig.StartImportRoutine(configImportPeriod)
		}

	}

	if flags.RefreshQueryCache {
		sklog.Info("Creating Redis Client.")
		traceStore, err := builders.NewTraceStoreFromConfig(ctx, instanceConfig)
		if err != nil {
			return skerr.Wrapf(err, "Failed to build TraceStore.")
		}

		dfBuilder := dfbuilder.NewDataFrameBuilderFromTraceStore(
			g,
			traceStore,
			nil, // setting trace cache to nil so that we don't cache it in maintenance service.
			2,
			dfbuilder.Filtering(instanceConfig.FilterParentTraces),
			instanceConfig.QueryConfig.CommitChunkSize,
			instanceConfig.QueryConfig.MaxEmptyTilesForQuery,
			instanceConfig.Experiments.PreflightSubqueriesForExistingKeys,
			[]string{})
		psRefresher := psrefresh.NewDefaultParamSetRefresher(traceStore, 2, dfBuilder, instanceConfig.QueryConfig, instanceConfig.Experiments)
		err = psRefresher.Start(time.Hour)
		if err != nil {
			return skerr.Wrapf(err, "Error starting paramset refreshser.")
		}

		cache, err := builders.GetCacheFromConfig(ctx, *instanceConfig)

		if err != nil {
			return skerr.Wrapf(err, "Error creating new cache instance")
		}
		cacheParamSetRefresher := psrefresh.NewCachedParamSetRefresher(psRefresher, cache)
		cacheParamSetRefresher.StartRefreshRoutine(redisCacheRefreshPeriod)
	}

	if flags.DeleteShortcutsAndRegressions {
		deleter, err := deletion.New(db)
		if err != nil {
			return skerr.Wrapf(err, "Error creating new Deleter")
		}
		deleter.RunPeriodicDeletion(deletionPeriod, deletionBatchSize)
	}

	select {}
}
