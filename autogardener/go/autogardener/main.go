package main

import (
	"context"
	"flag"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/ingester"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	ts_firestore "go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Flags
	btInstance        = flag.String("bigtable-instance", "", "BigTable instance to use.")
	btProject         = flag.String("bigtable-project", "", "GCE project to use for BigTable.")
	gitstoreTable     = flag.String("gitstore-bt-table", "git-repos2", "BigTable table used for GitStore.")
	port              = flag.String("port", ":8000", "HTTP service port.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	gcpProject        = flag.String("project", "skia-infra-public", "GCP project to use for Gemini API billing")
	location          = flag.String("location", "us-central1", "GCP location to use for Gemini API")
	cheapModel        = flag.String("cheap-model", "gemini-2.5-flash-lite", "Gemini model name to use for less-intensive tasks")
	cheapTPM          = flag.Int("cheap-tpm", 4000000, "Maximum tokens per minute for the cheap model")
	cheapRPM          = flag.Int("cheap-rpm", 1000, "Maximum requests per minute for the cheap model")
	expensiveModel    = flag.String("expensive-model", "gemini-flash-latest", "Gemini model name to use for more-intensive tasks")
	expensiveTPM      = flag.Int("expensive-tpm", 1000000, "Maximum tokens per minute for the expensive model")
	expensiveRPM      = flag.Int("expensive-rpm", 1000, "Maximum requests per minute for the expensive model")
	mcpServer         = flag.String("mcp-server", "https://mcp-skia.luci.app/sse", "MCP server to use.")
	firestoreProject  = flag.String("firestore-project", firestore.FIRESTORE_PROJECT, "Project to use for firestore.")
	firestoreInstance = flag.String("firestore-instance", "production", "Firestore instance to use.")
	repoURLs          = common.NewMultiStringFlag("repo", nil, "Repositories for which to perform gardening.")
	numCommits        = flag.Int("num-commits", 35, "Number of commits to load for each repo.")
	local             = flag.Bool("local", false, "True if running locally. Uses an emulator for writing to the DB.")
	apiKeySecret      = flag.String("api-key-secret", "autogardener-gemini-api-key", "GCP secret containing the Gemini API key.")
)

func main() {
	const appName = "autogardener"
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.StructuredLogging(local),
	)

	ctx := context.Background()

	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}

	dbOpts := []option.ClientOption{option.WithTokenSource(ts)}
	if *local {
		dbOpts = []option.ClientOption{
			option.WithEndpoint("127.0.0.1:8894"),
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		}
	}

	db, err := db.NewFirestoreDB(ctx, *firestoreProject, *firestoreInstance, dbOpts...)
	if err != nil {
		sklog.Fatal(err)
	}

	var geminiAPIKey string
	if *apiKeySecret == "" || *local {
		geminiAPIKey = os.Getenv("GEMINI_API_KEY")
		if geminiAPIKey == "" {
			sklog.Fatal("You must set GEMINI_API_KEY")
		}
	} else {
		sc, err := secret.NewClient(ctx)
		if err != nil {
			sklog.Fatal(err)
		}
		geminiAPIKey, err = sc.Get(ctx, *gcpProject, *apiKeySecret, secret.VersionLatest)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	geminiClient, err := gemini.NewClient(ctx, *gcpProject, *location, *cheapModel, *expensiveModel, geminiAPIKey, *mcpServer, *cheapRPM, *cheapTPM, *expensiveRPM, *expensiveTPM)
	if err != nil {
		sklog.Fatal(err)
	}

	// Git repo setup.
	btConf := &bt_gitstore.BTConfig{
		ProjectID:  *btProject,
		InstanceID: *btInstance,
		TableID:    *gitstoreTable,
		AppProfile: "status", // TODO(borenet): Using "autogardener" here results in a "Not Found" error.
	}
	repos, err := bt_gitstore.NewBTGitStoreMap(ctx, *repoURLs, btConf)
	if err != nil {
		sklog.Fatal(err)
	}

	// Task DB and Cache.
	tsDB, err := ts_firestore.NewDBWithParams(ctx, *firestoreProject, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	w, err := window.New(ctx, 4*24*time.Hour, *numCommits, repos)
	if err != nil {
		sklog.Fatalf("Failed to create time window: %s", err)
	}
	tCache, err := cache.NewTaskCache(ctx, tsDB, w, nil)
	if err != nil {
		sklog.Fatalf("Failed to create task cache: %s", err)
	}
	lv := metrics2.NewLiveness("autogardener_update")
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		failed := false
		if err := repos.Update(ctx); err != nil {
			sklog.Errorf("Failed to update repos: %s", err)
			failed = true
		}
		if err := w.Update(ctx); err != nil {
			sklog.Errorf("Failed to update window: %s", err)
			failed = true
		}
		if err := tCache.Update(ctx); err != nil {
			sklog.Errorf("Failed to update task cache")
			failed = true
		}
		if !failed {
			lv.Reset()
		}

	})

	ing, err := ingester.New(ctx, db, geminiClient, repos, tCache, tsDB)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Setup complete. Starting loop.")
	for _, repoURL := range *repoURLs {
		ing.StartIngestingTaskSummariesForRepo(ctx, repoURL, git.MainBranch, *numCommits)
	}

	httputils.RunHealthCheckServer(*port)
}
