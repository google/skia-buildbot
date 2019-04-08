/*
	Pulls data from multiple sources and funnels into metrics.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/datahopper/go/bot_metrics"
	"go.skia.org/infra/datahopper/go/swarming_metrics"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/taskname"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/perfclient"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/db/pubsub"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"google.golang.org/api/option"
)

// flags
var (
	// TODO(borenet): Combine btInstance, firestoreInstance, and
	// pubsubTopicSet.
	btInstance        = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject         = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories to query for status.")
	workdir           = flag.String("workdir", ".", "Working directory used by data processors.")

	perfBucket     = flag.String("perf_bucket", "skia-perf", "The GCS bucket that should be used for writing into perf")
	perfPrefix     = flag.String("perf_duration_prefix", "task-duration", "The folder name in the bucket that task duration metric shoudl be written.")
	pubsubProject  = flag.String("pubsub_project", "", "GCE project to use for PubSub.")
	pubsubTopicSet = flag.String("pubsub_topic_set", "", fmt.Sprintf("Pubsub topic set; one of: %v", pubsub.VALID_TOPIC_SETS))
)

var (
	// Regexp matching non-alphanumeric characters.
	re = regexp.MustCompile("[^A-Za-z0-9]+")

	BUILDSLAVE_OFFLINE_BLACKLIST = []string{
		"build3-a3",
		"build4-a3",
		"vm255-m3",
	}
)

func main() {
	common.InitWithMust(
		"datahopper",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	ctx := context.Background()

	// Absolutify the workdir.
	w, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(w)
	}
	sklog.Infof("Workdir is %s", w)

	// OAuth2.0 TokenSource.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, pubsub.AUTH_SCOPE, bigtable.Scope, datastore.ScopeDatastore, swarming.AUTH_SCOPE, auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Various API clients.
	swarm, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmInternal, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		sklog.Fatal(err)
	}
	gsClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Fatal(err)
	}
	storageClient := gcs.NewGCSClient(gsClient, *perfBucket)
	pc := perfclient.New(*perfPrefix, storageClient)

	tnp := taskname.DefaultTaskNameParser()

	// Shared repo objects.
	if *repoUrls == nil {
		sklog.Fatal("At least one --repo is required.")
	}
	btConf := &gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
	}
	repos, err := repograph.NewBTGitStoreMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	lvRepos := metrics2.NewLiveness("datahopper_repo_update")
	go util.RepeatCtx(time.Minute, ctx, func() {
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
	go util.RepeatCtx(30*time.Minute, ctx, func() {
		if err := tcc.Cleanup(ctx, OVERDUE_JOB_METRICS_PERIOD); err != nil {
			sklog.Errorf("Failed to cleanup TaskCfgCache: %s", err)
		}
	})

	// Data generation goroutines.

	// Swarming bots.
	swarmingClients := map[string]swarming.ApiClient{
		swarming.SWARMING_SERVER:         swarm,
		swarming.SWARMING_SERVER_PRIVATE: swarmInternal,
	}
	swarmingPools := map[string][]string{
		swarming.SWARMING_SERVER:         swarming.POOLS_PUBLIC,
		swarming.SWARMING_SERVER_PRIVATE: swarming.POOLS_PRIVATE,
	}
	swarming_metrics.StartSwarmingBotMetrics(swarmingClients, swarmingPools, metrics2.GetDefaultClient())

	// Swarming tasks.
	if err := swarming_metrics.StartSwarmingTaskMetrics(ctx, *btProject, *btInstance, swarm, pc, tnp, ts); err != nil {
		sklog.Fatal(err)
	}

	// Number of commits in the repo.
	go func() {
		skiaGauge := metrics2.GetInt64Metric("repo_commits", map[string]string{"repo": "skia"})
		infraGauge := metrics2.GetInt64Metric("repo_commits", map[string]string{"repo": "infra"})
		for range time.Tick(5 * time.Minute) {
			nSkia := repos[common.REPO_SKIA].Len()
			skiaGauge.Update(int64(nSkia))
			nInfra := repos[common.REPO_SKIA_INFRA].Len()
			infraGauge.Update(int64(nInfra))
		}
	}()

	// Tasks metrics.
	// TODO(borenet): We should include metrics from all three (production,
	// internal, staging) instances.
	label := "datahopper"
	mod, err := pubsub.NewModifiedData(*pubsubProject, *pubsubTopicSet, label, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	d, err := firestore.NewDB(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts, mod)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	if err := StartTaskMetrics(ctx, d); err != nil {
		sklog.Fatal(err)
	}

	// Jobs metrics.
	if err := StartJobMetrics(ctx, d, repos, tcc); err != nil {
		sklog.Fatal(err)
	}

	// Generate "time to X% bot coverage" metrics.
	if err := bot_metrics.Start(ctx, d, repos, tcc, *btProject, *btInstance, *workdir, ts); err != nil {
		sklog.Fatal(err)
	}

	if err := StartFirestoreBackupMetrics(ctx, ts); err != nil {
		sklog.Fatal(err)
	}

	// Wait while the above goroutines generate data.
	select {}
}
