// Package maintenance runs long running processes for a single perf instance.
package maintenance

import (
	"context"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dfbuilder"
	"go.skia.org/infra/perf/go/maintenance/deletion"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/regression/migration"
	sheriffconfig "go.skia.org/infra/perf/go/sheriffconfig/service"
	"go.skia.org/infra/perf/go/sql/expectedschema"
	"go.skia.org/infra/perf/go/trace_visibility/checker"
	"go.skia.org/infra/perf/go/trace_visibility/promoter"
	"go.skia.org/infra/perf/go/trace_visibility/provider"
	"go.skia.org/infra/perf/go/trace_visibility/provider/chrome"
	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore"
	"go.skia.org/infra/perf/go/tracing"
	"golang.org/x/oauth2/google"
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

	// How often to run the trace visibility checker.
	visibilityCheckPeriod = time.Hour

	// How often to run the trace visibility promoter.
	traceVisibilityPromotionPeriod = time.Hour * 12

	// Timeout for a single run of the trace visibility checker.
	visibilityCheckTimeout = time.Minute * 5

	// Initial delay before starting the background visibility tasks.
	visibilityTaskInitialDelay = time.Minute * 15
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

	startVisibilityChecker(ctx, instanceConfig, db)
	startVisibilityPromoter(ctx, instanceConfig, db)

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

		configProvider, err := sheriffconfig.CreateConfigProvider(
			ctx,
			flags.Local,
			instanceConfig.MaintenanceConfig.GitilesRepoUrl,
			instanceConfig.MaintenanceConfig.SheriffConfigPath,
			instanceConfig.MaintenanceConfig.FallbackToLucicfg,
		)

		if err != nil {
			sklog.Errorf("No valid config provider could be initialized for Sheriff Configs: %s", err)
		} else if configProvider != nil {
			sheriffConfig, err := sheriffconfig.New(ctx, db, subscriptionStore, alertStore, configProvider, instanceConfig.InstanceName)
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

func startVisibilityChecker(ctx context.Context, instanceConfig *config.InstanceConfig, db pool.Pool) {
	if instanceConfig.VisibilityConfig == nil {
		sklog.Info("Skipping visibility check: config missing.")
		return
	}

	var err error
	client := httputils.NewTimeoutClient()
	ts, tokenErr := google.DefaultTokenSource(ctx, auth.ScopeGerrit)
	if tokenErr == nil {
		client = httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	} else {
		sklog.Warningf("Failed to create authenticated token source for visibility checker: %s. Using unauthenticated client.", tokenErr)
	}

	var visibilityProvider provider.Provider
	switch instanceConfig.VisibilityConfig.ProviderName {
	case "chrome":
		visibilityProvider, err = chrome.ChromeProvider(*instanceConfig.VisibilityConfig, client)
	default:
		sklog.Errorf("Unknown visibility provider: %q", instanceConfig.VisibilityConfig.ProviderName)
		return
	}

	if err != nil {
		sklog.Errorf("Failed to initialize %q provider: %s", instanceConfig.VisibilityConfig.ProviderName, err)
		return
	}

	visibilityChecker := checker.NewChecker(sqlconfigstore.New(db), visibilityProvider)

	go func() {
		sklog.Infof("Waiting %v before starting trace visibility checker...", visibilityTaskInitialDelay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(visibilityTaskInitialDelay):
		}
		util.RepeatCtx(ctx, visibilityCheckPeriod, func(ctx context.Context) {
			checkCtx, cancel := context.WithTimeout(ctx, visibilityCheckTimeout)
			defer cancel()
			added, removed, err := visibilityChecker.Check(checkCtx)
			if err != nil {
				sklog.Errorf("Failed to run visibility checker: %s", err)
			}
			metrics2.GetCounter("perf_visibility_checker_added_count", nil).Inc(int64(added))
			metrics2.GetCounter("perf_visibility_checker_removed_count", nil).Inc(int64(removed))
		})
	}()
}

func startVisibilityPromoter(ctx context.Context, instanceConfig *config.InstanceConfig, db pool.Pool) {
	if instanceConfig.VisibilityConfig == nil {
		return
	}
	sklog.Info("Starting background trace visibility promoter...")
	configStore := sqlconfigstore.New(db)
	backgroundPromoter := promoter.New(db, configStore)
	go func() {
		sklog.Infof("Waiting %v before starting background trace visibility promoter loop...", visibilityTaskInitialDelay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(visibilityTaskInitialDelay):
		}
		sklog.Infof("Starting background trace visibility promoter loop (interval: %v)...", traceVisibilityPromotionPeriod)
		util.RepeatCtx(ctx, traceVisibilityPromotionPeriod, func(ctx context.Context) {
			start := time.Now()
			promoted, err := backgroundPromoter.Promote(ctx)
			if err != nil {
				sklog.Errorf("Failed background trace promotion loop: %s", err)
			}
			metrics2.GetCounter("perf_visibility_traces_promoted_count", nil).Inc(int64(promoted))
			metrics2.GetFloat64SummaryMetric("perf_visibility_promotion_time_s").Observe(time.Since(start).Seconds())
		})
	}()
}
