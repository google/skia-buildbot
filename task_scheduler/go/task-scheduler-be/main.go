package main

import (
	"context"
	"flag"
	"fmt"
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
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/skip_tasks"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	swarming_task_execution_v2 "go.skia.org/infra/task_scheduler/go/task_execution/swarmingv2"
	"go.skia.org/infra/task_scheduler/go/types"
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
	btInstance           = flag.String("bigtable-instance", "", "BigTable instance to use.")
	btProject            = flag.String("bigtable-project", "", "GCE project to use for BigTable.")
	debugBusyBots        = flag.Bool("debug-busy-bots", false, "If set, dump debug information in the busy-bots module.")
	port                 = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	firestoreInstance    = flag.String("firestore-instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable        = flag.String("gitstore-bt-table", "git-repos2", "BigTable table used for GitStore.")
	local                = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	rbeInstance          = flag.String("rbe-instance", "projects/chromium-swarm/instances/default_instance", "CAS instance to use")
	repoUrls             = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	scoreDecay24Hr       = flag.Float64("score-decay-24hr", 0.9, "Task candidate scores are penalized using linear time decay. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	swarmingServers      = swarming_task_execution_v2.SwarmingServersFlag("swarming-server", fmt.Sprintf("Maps Swarming server to its associated realm and pools, eg. %q. The first is used as the default.", swarming_task_execution_v2.ExpectSwarmingServersFlagFormat))
	timePeriod           = flag.String("time-window", "4d", "Time period to use.")
	commitWindow         = flag.Int("commit-window", 10, "Minimum number of recent commits to keep in the timeWindow.")
	diagnosticsBucket    = flag.String("diagnostics-bucket", "skia-task-scheduler-diagnostics", "Name of Google Cloud Storage bucket to use for diagnostics data.")
	promPort             = flag.String("prom-port", ":20000", "Metrics service address (e.g., ':10110')")
	pubsubTopicName      = flag.String("pubsub-topic", swarming.PUBSUB_TOPIC_SWARMING_TASKS, "Pub/Sub topic to use for Swarming tasks.")
	pubsubSubscriberName = flag.String("pubsub-subscriber", PUBSUB_SUBSCRIBER_TASK_SCHEDULER, "Pub/Sub subscriber name.")
)

func main() {
	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		common.StructuredLogging(local),
	)
	defer common.Defer()

	if len(*repoUrls) == 0 {
		sklog.Fatal("At least one --repo is required.")
	}

	if len(*swarmingServers) == 0 {
		sklog.Fatal("At least one --swarming-server is required.")
	}

	// TODO(borenet): This is disabled because it causes errors to be logged
	// every 5 seconds. I've tried reducing the sample frequency significantly
	// and it hasn't helped.
	//if err := tracing.Initialize(0.01, *btProject, nil); err != nil {
	//	sklog.Fatalf("Could not set up tracing: %s", err)
	//}
	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Set up token source and authenticated API clients.
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, auth.ScopeReadWrite, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, swarming.AUTH_SCOPE, compute.CloudPlatformScope /* TODO(borenet): No! */)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
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

	// Initialize storage client.
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

	// Create the task executors.
	var taskExecs types.TaskExecutors
	for _, swarmingServer := range *swarmingServers {
		swarmClient := swarmingv2.NewDefaultClient(httpClient, swarmingServer.Name)
		swarmingTaskExec := swarming_task_execution_v2.NewSwarmingV2TaskExecutor(swarmClient, swarmingServer.Name, *rbeInstance, *pubsubTopicName, swarmingServer.Realm, swarmingServer.Pools)
		taskExecs = append(taskExecs, swarmingTaskExec)
	}

	// Create and start the task scheduler.
	sklog.Infof("Creating task scheduler.")
	ts, err := scheduling.NewTaskScheduler(ctx, tsDb, skipTasks, period, *commitWindow, repos, cas, *rbeInstance, taskExecs, httpClient, *scoreDecay24Hr, *pubsubTopicName, taskCfgCache, tokenSource, diagClient, diagInstance, scheduling.BusyBotsDebugLog(*debugBusyBots))
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
