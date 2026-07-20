package main

import (
	"context"
	"flag"
	"os"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/autogardener/go/db"
	"go.skia.org/infra/autogardener/go/gemini"
	"go.skia.org/infra/autogardener/go/ingester"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/sklog"
	ts_firestore "go.skia.org/infra/task_scheduler/go/db/firestore"
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
	timePeriod        = flag.String("time-window", "4d", "Time period to use.")
	local             = flag.Bool("local", false, "True if running locally. Uses an emulator for writing to the DB.")
	apiKeySecret      = flag.String("api-key-secret", "autogardener-gemini-api-key", "GCP secret containing the Gemini API key.")
	gcsBucketDebug    = flag.String("gcs-bucket-debug", "", "Optional, GCS bucket name to upload debug information.")
)

func main() {
	const appName = "autogardener"
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.StructuredLogging(local),
	)

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		sklog.Fatal(err)
	}

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
	geminiClient, err := gemini.NewClient(ctx, *gcpProject, *location, *cheapModel, *expensiveModel, geminiAPIKey, *mcpServer, *gcsBucketDebug, *cheapRPM, *cheapTPM, *expensiveRPM, *expensiveTPM)
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

	// Task DB.
	tsDB, err := ts_firestore.NewDBWithParams(ctx, *firestoreProject, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	ing, err := ingester.New(ctx, db, geminiClient, repos, tsDB)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Setup complete. Starting loop.")
	for _, repoURL := range *repoURLs {
		ing.StartIngestingTaskSummariesForRepo(ctx, repoURL, period)
	}

	httputils.RunHealthCheckServer(*port)
}
