// For a given time period, find all the bots that failed when a CL, that was
// later reverted, first landed. The count of failed bots does not include bots
// that failed at both the initial commit and at the revert. Note that "-All"
// is removed from bot names.
//
// Running this requires a client_secret.json file in the current directory that
// is good for accessing the swarming API.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/predict/go/app"
	"go.skia.org/infra/predict/go/failures"
	"go.skia.org/infra/predict/go/flaky"
	"go.skia.org/infra/predict/go/statusprovider"
	"go.skia.org/infra/predict/go/tasklistprovider"
)

var (
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia.git", "The URL to pass to git clone for the source repository.")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	namespace    = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	projectName  = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	since        = flag.Duration("since", 24*time.Hour, "How far back to search in swarming history.")
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello world!")
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	if *namespace == "" {
		sklog.Fatal("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}

	// Check out or pull the repo.
	ctx := context.Background()
	git, err := git.NewCheckout(ctx, *gitRepoURL, *gitRepoDir)
	if err != nil {
		sklog.Fatal(err)
	}
	// TODO Run ./bin/try --list to get the list of legal bots.

	httpClient, err := auth.NewDefaultClient(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	provider := failures.TaskListProvider(tasklistprovider.NewProvider(swarmApi).Get)

	statusProvider := statusprovider.New(httpClient)

	fb := flaky.NewFlakyBuilder(flaky.FlakyProvider(statusProvider.Get))

	app, err := app.New(ctx, git, httpClient, provider, *since, "https://skia.googlesource.com/skia.git", fb)
	if err != nil {
		sklog.Fatalf("Failed to initialize app: %s", err)
	}
	app.Start()

	router := mux.NewRouter()
	router.HandleFunc("/", mainHandler)

	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())

	var h http.Handler = router
	h = httputils.LoggingGzipRequestResponse(h)
	http.Handle("/", h)

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
