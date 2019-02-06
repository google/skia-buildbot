// This program serves content that is mostly static and needs to be highly
// available. The content comes from highly available backend services like
// GCS. It needs to be deployed in a redundant way to ensure high uptime.
package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/web"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
)

// Command line flags.
var (
	baselineGSPath     = flag.String("baseline_gs_path", "", "GS path, where the baseline file are stored. This should match the same flag in skiacorrectness which writes the baselines. Format: <bucket>/<path>.")
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	gitRepoDir         = flag.String("git_repo_dir", "", "Directory location for the Skia repo.")
	gitRepoURL         = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	hashesGSPath       = flag.String("hashes_gs_path", "", "GS path, where the known hashes file should be stored. This should match the same flag in skiacorrectness which writes the hashes. Format: <bucket>/<path>.")
	port               = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	projectID          = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

func main() {

	// Set up logging.
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, []common.Opt{common.PrometheusOpt(promPort)}...)
	skiaversion.MustLogVersion()

	// Get the client to be used to access GCS and the Monorail issue tracker.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// TODO(dogben): Ok to add request/dial timeouts?
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).WithoutRetries().Client()
	evt := eventbus.New()
	ctx := context.Background()

	// TODO(stephana): There is a lot of overlap with code in skiacorrectness/main.go. This should
	// be factored out into a common function.
	gsClientOpt := &storage.GSClientOptions{
		HashesGSPath:   *hashesGSPath,
		BaselineGSPath: *baselineGSPath,
	}

	gsClient, err := storage.NewGStorageClient(client, gsClientOpt)
	if err != nil {
		sklog.Fatalf("Unable to create GStorageClient: %s", err)
	}

	if err := ds.InitWithOpt(*projectID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	// Set up the cloud expectations store, since at least the issue portion
	// will be used even if we use MySQL.
	expStore, issueExpStoreFactory, err := expstorage.NewCloudExpectationsStore(ds.DS, evt)
	if err != nil {
		sklog.Fatalf("Unable to configure cloud expectations store: %s", err)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(ds.DS, issueExpStoreFactory, evt)
	if err != nil {
		sklog.Fatalf("Unable to instantiate tryjob store: %s", err)
	}

	git, err := gitinfo.CloneOrUpdate(ctx, *gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	storages := &storage.Storage{
		GStorageClient:       gsClient,
		ExpectationsStore:    expstorage.NewCachingExpectationStore(expStore, evt),
		IssueExpStoreFactory: issueExpStoreFactory,
		TryjobStore:          tryjobStore,
		Git:                  git,
	}

	handlers := web.WebHandlers{
		Storages: storages,
	}

	router := mux.NewRouter()

	// Retrieving that baseline for master and an Gerrit issue are handled the same way
	router.HandleFunc(shared.EXPECATIONS_ROUTE, handlers.JsonBaselineHandler).Methods("GET")
	router.HandleFunc(shared.EXPECATIONS_ISSUE_ROUTE, handlers.JsonBaselineHandler).Methods("GET")

	// Start the server
	sklog.Infof("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, router))
}
