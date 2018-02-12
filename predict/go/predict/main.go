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
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
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
	gitRepoDir     = flag.String("git_repo_dir", "", "Directory location for the Skia repo.")
	gitRepoURL     = flag.String("git_repo_url", "https://skia.googlesource.com/skia.git", "The URL to pass to git clone for the source repository.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	maxBotDuration = flag.Duration("max_bot_duration", 4*time.Hour, "The longest any bot takes to run.")
	modelPeriod    = flag.Duration("model_period", 12*7*24*time.Hour, "The model should be built over all data over the last time period.")
	namespace      = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
	period         = flag.Duration("period", time.Hour, "How often to rebuild the prediction model.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	projectName    = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	remoteDbURL    = flag.String("remote_db_url", "http://skia-task-scheduler:8008/db/", "The URL of the task scheduler remote db endpoint.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var (
	templates *template.Template

	state *app.App

	httpClient *http.Client

	gerritClient *gerrit.Gerrit
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

func predictHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	files := []string{}
	changeStr := r.FormValue("change")
	if changeStr != "" {
		issue, err := strconv.ParseInt(changeStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to parse change query parameter.")
			return
		}
		files, err = gerritClient.GetFileNames(issue, r.FormValue("revisions"))
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to retrieve file list from CL.")
			return
		}
		sklog.Infof("Got file list: %v", files)
	}
	summary := state.Predict(files)
	sort.Sort(failures.SummarySlice(summary))
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
	common.InitWithMust(
		"predict",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	loadTemplates()

	if *namespace == "" {
		sklog.Fatal("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if *gitRepoDir == "" {
		sklog.Fatal("The --git_repo_dir flag is required.")
	}
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}
	if os.Getenv("DATASTORE_EMULATOR_HOST") != "" {
		sklog.Warning("Running against the Cloud Datastore Emulator!")
	}

	ctx := context.Background()
	repo, err := git.NewCheckout(ctx, *gitRepoURL, *gitRepoDir)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient, err = auth.NewDefaultJWTServiceAccountClient(swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}
	gerritClient, err = gerrit.NewGerrit("https://skia-review.googlesource.com", "", httpClient)
	if err != nil {
		sklog.Fatal(err)
	}
	taskListProvider := failures.TaskListProvider(tasklistprovider.New(swarmApi).Get)
	statusProvider, err := statusprovider.New([]string{common.REPO_SKIA}, *remoteDbURL)
	if err != nil {
		sklog.Fatal(err)
	}
	fb := flaky.NewFlakyBuilder(flaky.FlakyProvider(statusProvider.Get))
	state, err = app.New(repo, httpClient, taskListProvider, *period, *modelPeriod, common.REPO_SKIA, fb, *maxBotDuration)
	if err != nil {
		sklog.Fatalf("Failed to initialize app: %s", err)
	}
	go state.Start()

	router := mux.NewRouter()
	router.HandleFunc("/", mainHandler)
	router.HandleFunc("/predict", predictHandler)
	router.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	var h http.Handler = router
	h = httputils.LoggingGzipRequestResponse(h)
	http.Handle("/", h)

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
