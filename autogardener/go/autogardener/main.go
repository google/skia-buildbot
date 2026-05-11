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
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	ts_firestore "go.skia.org/infra/task_scheduler/go/db/firestore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	// Flags
	port              = flag.String("port", ":8000", "HTTP service port.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	gcpProject        = flag.String("project", "skia-infra-public", "GCP project to use for Gemini API billing")
	location          = flag.String("location", "us-central1", "GCP location to use for Gemini API")
	model             = flag.String("model", "gemini-flash-latest", "Gemini model name to use")
	mcpServer         = flag.String("mcp-server", "https://mcp-skia.luci.app/sse", "MCP server to use.")
	firestoreProject  = flag.String("firestore-project", firestore.FIRESTORE_PROJECT, "Project to use for firestore.")
	firestoreInstance = flag.String("firestore-instance", "production", "Firestore instance to use.")
	repoURLs          = common.NewMultiStringFlag("repo", nil, "Repositories for which to perform gardening.")
	numCommits        = flag.Int("num-commits", 35, "Number of commits to load for each repo.")
	local             = flag.Bool("local", false, "True if running locally. Uses an emulator for writing to the DB.")
	apiKeySecret      = flag.String("api-key-secret", "autogardener-gemini-api-key", "GCP secret containing the Gemini API key.")
)

func main() {
	common.InitWithMust(
		"autogardener",
		common.PrometheusOpt(promPort),
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
	geminiClient, err := gemini.NewClient(ctx, *gcpProject, *location, *model, geminiAPIKey, *mcpServer)
	if err != nil {
		sklog.Fatal(err)
	}
	tsDB, err := ts_firestore.NewDBWithParams(ctx, *firestoreProject, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	ing, err := ingester.New(ctx, db, geminiClient, tsDB)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Setup complete. Starting loop.")

	// TODO(borenet): Once the initial ingestion is complete, it would be much
	// better to maintain a queue and trigger task summary ingestion for each
	// task as soon as it fails.
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		sklog.Infof("Ingesting tasks for %d repos.", len(*repoURLs))
		for _, repoURL := range *repoURLs {
			if err := ing.IngestTaskSummariesForRepo(ctx, repoURL, git.MainBranch, *numCommits); err != nil {
				sklog.Errorf("failed ingesting tasks for repo %s: %s", repoURL, err)
			}
		}
	})
	httputils.RunHealthCheckServer(*port)
}
