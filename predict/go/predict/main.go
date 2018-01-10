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
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	"go.skia.org/infra/predict/go/gerrit"
	"go.skia.org/infra/predict/go/statusprovider"
	"go.skia.org/infra/predict/go/tasklistprovider"
)

var (
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia.git", "The URL to pass to git clone for the source repository.")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	modelPeriod  = flag.Duration("model_period", 12*7*24*time.Hour, "The model should be built over all data over the last time period.")
	namespace    = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
	period       = flag.Duration("period", time.Hour, "How often to rebuild the prediction model.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	projectName  = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var (
	templates *template.Template

	state *app.App

	httpClient *http.Client
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	f, err := state.Failures(24 * time.Hour)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve failures")
		return
	}
	context := struct {
		FlakyRanges      flaky.Flaky
		SinceLastRun     string
		Failures         []*failures.StoredFailure
		ComputedFailures failures.Failures
	}{
		FlakyRanges:      state.FlakyRanges(),
		SinceLastRun:     state.SinceLastRun(),
		Failures:         f,
		ComputedFailures: state.ComputedFailures(),
	}
	if err := templates.ExecuteTemplate(w, "index.html", context); err != nil {
		sklog.Errorln("Failed to expand template:", err)
	}
}

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/predict.html"),
	))
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	files, err := gerrit.Files(httpClient, r.FormValue("change"), r.FormValue("revision"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve file list from CL.")
		return
	}
	sklog.Infof("Got file list: %v", files)
	summary := state.Predict(files)
	sort.Sort(failures.SummarySlice(summary))
	for _, s := range summary {
		sklog.Infof("Summary: %v", *s)
	}
	if r.FormValue("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(summary); err != nil {
			sklog.Errorln("Failed to encode json:", err)
		}
	} else {
		w.Header().Set("Content-Type", "text/html")
		context := struct {
			Prediction []*failures.Summary
			Files      []string
		}{
			Prediction: summary,
			Files:      files,
		}
		if err := templates.ExecuteTemplate(w, "predict.html", context); err != nil {
			sklog.Errorln("Failed to expand template:", err)
		}
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"predict",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	loadTemplates()

	if *namespace == "" {
		sklog.Fatal("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}
	if os.Getenv("DATASTORE_EMULATOR_HOST") != "" {
		sklog.Warning("Running against the Cloud Datastore Emulator!")
	}

	// Check out or pull the repo.
	ctx := context.Background()
	git, err := git.NewCheckout(ctx, *gitRepoURL, *gitRepoDir)
	if err != nil {
		sklog.Fatal(err)
	}

	httpClient, err = auth.NewDefaultClient(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	provider := failures.TaskListProvider(tasklistprovider.New(swarmApi).Get)

	statusProvider := statusprovider.New(httpClient)

	fb := flaky.NewFlakyBuilder(flaky.FlakyProvider(statusProvider.Get))

	state, err = app.New(ctx, git, httpClient, provider, *period, *modelPeriod, "https://skia.googlesource.com/skia.git", fb, *gitRepoDir)
	if err != nil {
		sklog.Fatalf("Failed to initialize app: %s", err)
	}
	go state.Start()

	router := mux.NewRouter()
	router.HandleFunc("/", mainHandler)
	router.HandleFunc("/predict", predictHandler)
	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())

	var h http.Handler = router
	h = httputils.LoggingGzipRequestResponse(h)
	http.Handle("/", h)

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
