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

	"github.com/gorilla/mux"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/web"
)

type baselineServerConfig struct {
	config.Common

	// HTTP service address (e.g., ':9000')
	Port string `json:"port"`

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`
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

	firestore.EnsureNotEmulator()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(&bsc.PromPort),
	}
	ctx := context.Background()

	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)
	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, bsc.FirestoreProjectID, "gold", bsc.FirestoreNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	expStore := fs_expectationstore.New(fsClient, nil, fs_expectationstore.ReadOnly)
	if err := expStore.Initialize(ctx); err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}

	// Initialize the Baseliner instance from the values set above.
	baseliner := simple_baseliner.New(expStore)

	gsClientOpt := storage.GCSClientOptions{
		KnownHashesGCSPath: bsc.KnownHashesGCSPath,
	}

	tokenSource, err := auth.NewDefaultTokenSource(bsc.Local, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Could not create token source: %s", err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	// Baseline doesn't need to access this, just needs a way to indicate which CRS we are on.
	emptyCLStore := fs_clstore.New(nil, bsc.PrimaryCRS)

	// We only need to fill in the HandlersConfig struct with the following subset, since the baseline
	// server only supplies a subset of the functionality.
	handlers, err := web.NewHandlers(web.HandlersConfig{
		GCSClient:       gsClient,
		Baseliner:       baseliner,
		ChangeListStore: emptyCLStore,
	}, web.BaselineSubset)
	if err != nil {
		sklog.Fatalf("Failed to initialize web handlers: %s", err)
	}

	// Set up a router for all the application endpoints which are part of the Gold API.
	appRouter := mux.NewRouter()

	// Serve the known hashes from GCS.
	appRouter.HandleFunc(shared.KnownHashesRoute, handlers.TextKnownHashesProxy).Methods("GET")

	// Serve the expectations for the master branch and for CLs in progress.
	appRouter.HandleFunc(shared.ExpectationsRoute, handlers.BaselineHandler).Methods("GET")
	// TODO(lovisolo): Remove the below route once goldctl is fully migrated.
	appRouter.HandleFunc(shared.ExpectationsLegacyRoute, handlers.BaselineHandler).Methods("GET")

	// Only log and compress the app routes, but not the health check.
	router := mux.NewRouter()
	router.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	router.PathPrefix("/").Handler(httputils.LoggingGzipRequestResponse(appRouter))

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + bsc.Port)
	sklog.Fatal(http.ListenAndServe(bsc.Port, router))
}
