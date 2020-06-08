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

	"go.skia.org/infra/golden/go/clstore/fs_clstore"

	"github.com/gorilla/mux"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/baseline/simple_baseliner"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/web"
)

func main() {
	// Command line flags.
	var (
		fsNamespace        = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
		fsProjectID        = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		knownHashesGCSPath = flag.String("known_hashes_gcs_path", "", "GCS path, where the known hashes file should be stored. This should match the same flag in skiacorrectness which writes the hashes. Format: <bucket>/<path>.")
		local              = flag.Bool("local", false, "if running local (not in production)")
		primaryCRS         = flag.String("primary_crs", "gerrit", "Primary CodeReviewSystem (e.g. 'gerrit', 'github'")
		port               = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
		promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	)

	// Parse the options. So we can configure logging.
	flag.Parse()

	if *fsNamespace == "" {
		sklog.Fatalf("--fs_namespace must be set")
	}

	firestore.EnsureNotEmulator()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}
	ctx := context.Background()

	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)
	skiaversion.MustLogVersion()
	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "gold", *fsNamespace, nil)
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
		KnownHashesGCSPath: *knownHashesGCSPath,
	}

	tokenSource, err := auth.NewDefaultTokenSource(*local, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Could not create token source: %s", err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()

	gsClient, err := storage.NewGCSClient(ctx, client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GCSClient: %s", err)
	}

	// Baseline doesn't need to access this, just needs a way to indicate which CRS we are on.
	emptyCLStore := fs_clstore.New(nil, *primaryCRS)

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
	sklog.Infof("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, router))
}
