// This program serves content that is mostly static and needs to be highly
// available. The content comes from highly available backend services like
// GCS. It needs to be deployed in a redundant way to ensure high uptime.
// It is read-only; it does not create new baselines or update expectations.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/oauth2/google"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tracing"
	"go.skia.org/infra/golden/go/web"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// Arbitrary number
	maxSQLConnections = 12
)

type baselineServerConfig struct {
	config.Common
}

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to baseline server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the flags, so we can load the configuration files.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var bsc baselineServerConfig
	if err := config.LoadFromJSON5(&bsc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", bsc)

	if err := tracing.Initialize(0.1, bsc.SQLDatabaseName); err != nil {
		sklog.Fatalf("Could not initialize tracing: %s", err)
	}

	ctx := context.Background()
	db := mustInitSQLDatabase(ctx, bsc)

	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(
		appName,
		common.PrometheusOpt(&bsc.PromPort),
	)

	gsClientOpt := storage.GCSClientOptions{
		Bucket:             bsc.GCSBucket,
		KnownHashesGCSPath: bsc.KnownHashesGCSPath,
	}

	tokenSource, err := google.DefaultTokenSource(ctx, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Could not create token source: %s", err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	// Baselines just need a list of valid CRS; we can leave all other fields blank.
	var reviewSystems []clstore.ReviewSystem
	for _, cfg := range bsc.CodeReviewSystems {
		reviewSystems = append(reviewSystems, clstore.ReviewSystem{ID: cfg.ID})
	}

	// We only need to fill in the HandlersConfig struct with the following subset, since the baseline
	// server only supplies a subset of the functionality.
	handlers, err := web.NewHandlers(web.HandlersConfig{
		DB:                        db,
		GCSClient:                 gsClient,
		ReviewSystems:             reviewSystems,
		GroupingParamKeysByCorpus: bsc.GroupingParamKeysByCorpus,
	}, web.BaselineSubset, proxylogin.NewWithDefaults())
	if err != nil {
		sklog.Fatalf("Failed to initialize web handlers: %s", err)
	}

	handlers.StartKnownHashesCacheProcess(ctx)

	// Set up a router for all the application endpoints which are part of the Gold API.
	appRouter := chi.NewRouter()

	// Version 0 of the routes are actually the unversioned legacy versions of the route.
	v0 := func(method, rpcRoute string, handlerFunc http.HandlerFunc) {
		counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
			// For consistency, we remove the /json from all routes when adding them in the metrics.
			"route":   strings.TrimPrefix(rpcRoute, "/json"),
			"version": "v0",
		})
		appRouter.MethodFunc(method, rpcRoute, func(w http.ResponseWriter, r *http.Request) {
			counter.Inc(1)
			handlerFunc(w, r)
		})
	}

	v1 := func(method, rpcRoute string, handlerFunc http.HandlerFunc) {
		counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
			// For consistency, we remove the /json/vN from all routes when adding them in the metrics.
			"route":   strings.TrimPrefix(rpcRoute, "/json/v1"),
			"version": "v1",
		})
		appRouter.MethodFunc(method, rpcRoute, func(w http.ResponseWriter, r *http.Request) {
			counter.Inc(1)
			handlerFunc(w, r)
		})
	}

	v2 := func(method, rpcRoute string, handlerFunc http.HandlerFunc) {
		counter := metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
			// For consistency, we remove the /json/vN from all routes when adding them in the metrics.
			"route":   strings.TrimPrefix(rpcRoute, "/json/v2"),
			"version": "v2",
		})
		appRouter.MethodFunc(method, rpcRoute, func(w http.ResponseWriter, r *http.Request) {
			counter.Inc(1)
			handlerFunc(w, r)
		})
	}

	// Serve the known hashes from GCS.
	v0("GET", frontend.KnownHashesRoute, handlers.KnownHashesHandler)
	v1("GET", frontend.KnownHashesRouteV1, handlers.KnownHashesHandler)
	// Serve the expectations for the primary branch and for CLs in progress.
	v2("GET", frontend.ExpectationsRouteV2, handlers.BaselineHandlerV2)
	v1("GET", frontend.GroupingsRouteV1, handlers.GroupingsHandler)

	// Only log and compress the app routes, but not the health check.
	router := chi.NewRouter()
	router.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	router.Handle("/*", httputils.LoggingGzipRequestResponse(appRouter))

	// Start the internal server on the internal port if requested.
	if bsc.DebugPort != "" {
		go func() {
			// Sample usage:
			//     $ kubectl port-forward --address 0.0.0.0 gold-skia-infra-baselineserver-xxxxxxxxxx-yyyyy 8000:8001
			sklog.Infof("Internal server on http://127.0.0.1" + bsc.DebugPort)
			httputils.ServePprof(bsc.DebugPort)
		}()
	}

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + bsc.ReadyPort)
	sklog.Fatal(http.ListenAndServe(bsc.ReadyPort, router))
}

func mustInitSQLDatabase(ctx context.Context, bsc baselineServerConfig) *pgxpool.Pool {
	if bsc.SQLDatabaseName == "" {
		sklog.Fatalf("Must have SQL Database Information")
	}
	url := sql.GetConnectionURL(bsc.SQLConnection, bsc.SQLDatabaseName)
	conf, err := pgxpool.ParseConfig(url)
	if err != nil {
		sklog.Fatalf("error getting postgres config %s: %s", url, err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	sklog.Infof("Connected to SQL database %s", bsc.SQLDatabaseName)
	return db
}
