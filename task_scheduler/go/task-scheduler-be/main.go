package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/tracing"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	swarming_task_execution "go.skia.org/infra/task_scheduler/go/task_execution/swarming"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task-scheduler-be"

	PUBSUB_SUBSCRIBER_TASK_SCHEDULER          = "task-scheduler"
	PUBSUB_SUBSCRIBER_TASK_SCHEDULER_INTERNAL = "task-scheduler-internal"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = APP_NAME
)

var (
	// Flags.
	btInstance        = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject         = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	cdPool            = flag.String("cd_pool", "", "Swarming pool used only for continuous deployment tasks.")
	port              = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable     = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local             = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	rbeInstance       = flag.String("rbe_instance", "projects/chromium-swarm/instances/default_instance", "CAS instance to use")
	repoUrls          = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	scoreDecay24Hr    = flag.Float64("scoreDecay24Hr", 0.9, "Task candidate scores are penalized using linear time decay. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	swarmingPools     = common.NewMultiStringFlag("pool", nil, "Which Swarming pools to use.")
	swarmingServer    = flag.String("swarming_server", swarming.SWARMING_SERVER, "Which Swarming server to use.")
	timePeriod        = flag.String("timeWindow", "4d", "Time period to use.")
	commitWindow      = flag.Int("commitWindow", 10, "Minimum number of recent commits to keep in the timeWindow.")
	diagnosticsBucket = flag.String("diagnostics_bucket", "skia-task-scheduler-diagnostics", "Name of Google Cloud Storage bucket to use for diagnostics data.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	pubsubTopicName      = flag.String("pubsub_topic", swarming.PUBSUB_TOPIC_SWARMING_TASKS, "Pub/Sub topic to use for Swarming tasks.")
	pubsubSubscriberName = flag.String("pubsub_subscriber", PUBSUB_SUBSCRIBER_TASK_SCHEDULER, "Pub/Sub subscriber name.")
)

func main() {

	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	if err := tracing.Initialize(0.1, *btProject, nil); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}
	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Set up token source and authenticated API clients.
	gitcookiesPath := "/tmp/.gitcookies"
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, auth.ScopeReadWrite, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE, compute.CloudPlatformScope /* TODO(borenet): No! */)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}
	if _, err := gitauth.New(tokenSource, gitcookiesPath, true, ""); err != nil {
		sklog.Fatalf("Failed to create git cookie updater: %s", err)
	}
	cas, err := rbe.NewClient(ctx, *rbeInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create RBE-CAS client: %s", err)
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()

	// Initialize the database.
	tsDb, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	cleanup.AtExit(func() {
		util.Close(tsDb)
	})

	// Skip tasks DB.
	skipTasks, err := skip_tasks.NewWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatal(err)
	}

	// Git repos.
	if *repoUrls == nil {
		sklog.Fatal("--repo is required.")
	}
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: "task-scheduler",
	}
	autoUpdateRepos, err := gs_pubsub.NewAutoUpdateMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	repos := autoUpdateRepos.Map

	// Initialize Swarming client.
	cfg := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithDialTimeout(time.Minute).With2xxOnly()
	cfg.RequestTimeout = time.Minute
	swarm, err := swarming.NewApiClient(cfg.Client(), *swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}

	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		sklog.Fatal(err)
	}
	diagClient := gcsclient.New(storageClient, *diagnosticsBucket)
	diagInstance := *firestoreInstance

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create caches.
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, *btProject, *btInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCfgCache: %s", err)
	}

	// Create and start the task scheduler.
	sklog.Infof("Creating task scheduler.")
	taskExec := swarming_task_execution.NewSwarmingTaskExecutor(swarm, *rbeInstance, *pubsubTopicName)
	ts, err := scheduling.NewTaskScheduler(ctx, tsDb, skipTasks, period, *commitWindow, repos, cas, *rbeInstance, taskExec, httpClient, *scoreDecay24Hr, *swarmingPools, *cdPool, *pubsubTopicName, taskCfgCache, tokenSource, diagClient, diagInstance)
	if err != nil {
		sklog.Fatal(err)
	}
	cleanup.AtExit(func() {
		util.LogErr(ts.Close())
	})
	if err := swarming.InitPubSub(*pubsubTopicName, *pubsubSubscriberName, ts.HandleSwarmingPubSub); err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Created task scheduler. Starting loop.")
	ts.Start(ctx)
	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, tokenSource, 5*time.Minute, func(ctx context.Context, repo string, graph *repograph.Graph, ack, nack func()) error {
		ack()
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}

	// Run the health check server.
	httputils.RunHealthCheckServer(*port)
}
