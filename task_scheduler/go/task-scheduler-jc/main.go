package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	gs_pubsub "go.skia.org/infra/go/gitstore/pubsub"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/periodic"
	"go.skia.org/infra/go/pubsub"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tracing"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/job_creation"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "task-scheduler-jc"

	// PubSub subscriber ID used for GitStore.
	GITSTORE_SUBSCRIBER_ID = APP_NAME
)

var (
	// Flags.
	btInstance               = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject                = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	buildbucketBucket        = flag.String("tryjob_bucket", tryjobs.BUCKET_PRIMARY, "Which Buildbucket bucket to use for try jobs.")
	buildbucketProject       = flag.String("buildbucket_project", "skia", "luci-config project in which the buildbucket bucket is defined.")
	buildbucketTarget        = flag.String("buildbucket_target", "", "Buildbucket backend target name used to address this scheduler.")
	buildbucketPubSubProject = flag.String("buildbucket_pubsub_project", "", "Pub/sub project used for sending messages to Buildbucket.")
	host                     = flag.String("host", "localhost", "HTTP service host")
	port                     = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	disableTryjobs           = flag.Bool("disable_try_jobs", false, "If set, no try jobs will be picked up.")
	firestoreInstance        = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	gitstoreTable            = flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	local                    = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	rbeInstance              = flag.String("rbe_instance", "projects/chromium-swarm/instances/default_instance", "CAS instance to use")
	repoUrls                 = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")
	timePeriod               = flag.String("timeWindow", "4d", "Time period to use.")
	tracingProject           = flag.String("tracing_project", "", "GCP project where traces should be uploaded.")
	commitWindow             = flag.Int("commitWindow", 10, "Minimum number of recent commits to keep in the timeWindow.")
	workdir                  = flag.String("workdir", "workdir", "Working directory to use.")
	promPort                 = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	numSyncWorkers           = flag.Int("sync_workers", syncer.DefaultNumWorkers, "Number of sync worker goroutines to use.")
)

func main() {

	// Global init.
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
	)
	defer common.Defer()

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	if *tracingProject != "" {
		if err := tracing.Initialize(0.1, *tracingProject, nil); err != nil {
			sklog.Fatalf("Could not set up tracing: %s", err)
		}
	}
	ctx, cancelFn := context.WithCancel(context.Background())
	cleanup.AtExit(cancelFn)

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// task-scheduler-jc uses a custom temporary dir, to ensure that it's on a
	// persistent disk. Create it if it does not exist.
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		if err := os.Mkdir(os.TempDir(), os.ModePerm); err != nil {
			sklog.Fatalf("Failed to create %s: %s", os.TempDir(), err)
		}
	}

	// Set up token source and authenticated API clients.
	var tokenSource oauth2.TokenSource
	tokenSource, err = google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, pubsub.ScopePubSub, datastore.ScopeDatastore, bigtable.Scope, compute.CloudPlatformScope /* TODO(borenet): No! */)
	if err != nil {
		sklog.Fatalf("Failed to create token source: %s", err)
	}
	cas, err := rbe.NewClient(ctx, *rbeInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create RBE-CAS client: %s", err)
	}
	if _, err := gitauth.New(ctx, tokenSource, types.GitCookiesPath, true, ""); err != nil {
		sklog.Fatalf("Failed to create git cookie updater: %s", err)
	}

	// Authenticated HTTP client.
	httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()

	// Gerrit API client.
	gerrit, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, httpClient)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize the database.
	tsDb, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, tokenSource)
	if err != nil {
		sklog.Fatalf("Failed to create Firestore DB client: %s", err)
	}
	cleanup.AtExit(func() {
		util.Close(tsDb)
	})

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

	// Find depot_tools.
	// TODO(borenet): Package depot_tools in the Docker image.
	depotTools, err := depot_tools.Sync(ctx, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

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

	// Create pubsub client.
	var pubsubClient pubsub.Client
	if *buildbucketPubSubProject != "" {
		pubsubClient, err = pubsub.NewClient(ctx, *buildbucketPubSubProject, option.WithTokenSource(tokenSource))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create and start the JobCreator.
	sklog.Infof("Creating JobCreator.")
	jc, err := job_creation.NewJobCreator(ctx, tsDb, period, *commitWindow, wdAbs, serverURL, repos, cas, httpClient, *buildbucketProject, *buildbucketTarget, *buildbucketBucket, common.PROJECT_REPO_MAPPING, depotTools, gerrit, taskCfgCache, pubsubClient, *numSyncWorkers)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Created JobCreator. Starting loop.")
	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, tokenSource, 5*time.Minute, jc.HandleRepoUpdate); err != nil {
		sklog.Fatal(err)
	}
	jc.Start(ctx, !*disableTryjobs)

	// Set up periodic triggers.
	if err := periodic.Listen(ctx, fmt.Sprintf("task-scheduler-%s", *firestoreInstance), tokenSource, func(ctx context.Context, name, id string) bool {
		if err := jc.MaybeTriggerPeriodicJobs(ctx, name); err != nil {
			sklog.Errorf("Failed to trigger periodic jobs; will retry later: %s", err)
			return false // We will retry later.
		}
		return true
	}); err != nil {
		sklog.Fatal(err)
	}

	// Run the health check server.
	httputils.RunHealthCheckServer(*port)
}
