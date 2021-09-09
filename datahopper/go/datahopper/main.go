/*
	Pulls data from multiple sources and funnels into metrics.
*/

package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/datahopper/go/bot_metrics"
	"go.skia.org/infra/datahopper/go/supported_branches"
	"go.skia.org/infra/datahopper/go/swarming_metrics"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/perfclient"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/window"
	"google.golang.org/api/option"
)

const appName = "datahopper"

// flags
var (
	// TODO(borenet): Combine btInstance and firestoreInstance.
	btInstance        = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject         = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	perfBucket        = flag.String("perf_bucket", "skia-perf", "The GCS bucket that should be used for writing into perf")
	perfPrefix        = flag.String("perf_duration_prefix", "task-duration", "The folder name in the bucket that task duration metric should be written.")
	port              = flag.String("port", ":8000", "HTTP service port for the health check server (e.g., ':8000')")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories to query for status.")
	swarmingServer    = flag.String("swarming_server", "", "Host name of the Swarming server.")
	swarmingPools     = common.NewMultiStringFlag("swarming_pool", nil, "Swarming pools to use.")
)

func main() {
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	ctx := context.Background()

	// OAuth2.0 TokenSource.
	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail, pubsub.ScopePubSub, bigtable.Scope, datastore.ScopeDatastore, swarming.AUTH_SCOPE, auth.ScopeReadWrite, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Various API clients.
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Fatal(err)
	}
	storageClient := gcsclient.New(gsClient, *perfBucket)
	pc := perfclient.New(*perfPrefix, storageClient)

	tnp := taskname.DefaultTaskNameParser()

	// Shared repo objects.
	if *repoUrls == nil {
		sklog.Fatal("At least one --repo is required.")
	}
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: appName,
	}
	repos, err := bt_gitstore.NewBTGitStoreMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	lvRepos := metrics2.NewLiveness("datahopper_repo_update")
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		if err := repos.Update(ctx); err != nil {
			sklog.Errorf("Failed to update repos: %s", err)
		} else {
			lvRepos.Reset()
		}
	})

	// TaskCfgCache.
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCfgCache: %s", err)
	}
	go util.RepeatCtx(ctx, 30*time.Minute, func(ctx context.Context) {
		if err := tcc.Cleanup(ctx, OVERDUE_JOB_METRICS_PERIOD); err != nil {
			sklog.Errorf("Failed to cleanup TaskCfgCache: %s", err)
		}
	})

	// Data generation goroutines.

	// Swarming bots.
	swarmClient, err := swarming.NewApiClient(httpClient, *swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}
	swarming_metrics.StartSwarmingBotMetrics(ctx, *swarmingServer, *swarmingPools, swarmClient, metrics2.GetDefaultClient())

	// Swarming tasks.
	if err := swarming_metrics.StartSwarmingTaskMetrics(ctx, *btProject, *btInstance, swarmClient, *swarmingPools, pc, tnp, ts); err != nil {
		sklog.Fatal(err)
	}

	// Number of commits in the repo.
	go func() {
		for range time.Tick(5 * time.Minute) {
			for repoUrl, repo := range repos {
				normUrl, err := git.NormalizeURL(repoUrl)
				if err != nil {
					sklog.Fatal(err)
				}
				tags := map[string]string{"repo": normUrl}
				metrics2.GetInt64Metric("repo_commits", tags).Update(int64(repo.Len()))
			}
		}
	}()

	// Task and Job DB and shared caches.
	d, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	period := TIME_PERIODS[len(TIME_PERIODS)-1]
	if OVERDUE_JOB_METRICS_PERIOD > period {
		period = OVERDUE_JOB_METRICS_PERIOD
	}
	if bot_metrics.MAX_TIME_PERIOD > period {
		period = bot_metrics.MAX_TIME_PERIOD
	}
	w, err := window.New(period, OVERDUE_JOB_METRICS_NUM_COMMITS, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	tCache, err := cache.NewTaskCache(ctx, d, w, nil)
	if err != nil {
		sklog.Fatalf("Failed to create task cache: %s", err)
	}
	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	if err != nil {
		sklog.Fatalf("Failed to create job cache: %s", err)
	}

	// Task metrics.
	if err := StartTaskMetrics(ctx, tCache, *firestoreInstance); err != nil {
		sklog.Fatal(err)
	}

	// Jobs metrics.
	if err := StartJobMetrics(ctx, jCache, w, *firestoreInstance, repos, tcc); err != nil {
		sklog.Fatal(err)
	}

	// Generate "time to X% bot coverage" metrics.
	if err := bot_metrics.Start(ctx, tCache, repos, tcc, *btProject, *btInstance, ts); err != nil {
		sklog.Fatal(err)
	}

	if err := StartFirestoreBackupMetrics(ctx, ts); err != nil {
		sklog.Fatal(err)
	}

	// Collect metrics for supported branches.
	supported_branches.Start(ctx, *repoUrls, httpClient, swarmClient, *swarmingPools)

	// Metrics for last modification timestamp of go.mod.
	goModRepos := map[string][]string{}
	for _, repo := range *repoUrls {
		if util.In(repo, []string{common.REPO_SKIA, common.REPO_SKIA_INFRA}) {
			goModRepos[repo] = []string{"go.mod"}
		}
	}
	StartLastModifiedMetrics(ctx, httpClient, goModRepos)

	// Wait while the above goroutines generate data.
	httputils.RunHealthCheckServer(*port)
}
