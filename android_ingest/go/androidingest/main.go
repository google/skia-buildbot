package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/android_ingest/go/buildapi"
	"go.skia.org/infra/android_ingest/go/continuous"
	"go.skia.org/infra/android_ingest/go/lookup"
	"go.skia.org/infra/android_ingest/go/parser"
	"go.skia.org/infra/android_ingest/go/recent"
	"go.skia.org/infra/android_ingest/go/upload"
	"go.skia.org/infra/go/allowed"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// txLogDir is the sub-directory of *storageUrl that is used to store the incoming POST transaction log.
	txLogDir = "tx_log"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	repoURL      = flag.String("repo_url", "", "URL of the git repo where buildids are to be stored.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	storageURL   = flag.String("storage_url", "gs://skia-perf/android-ingest", "The GCS URL of where to store the ingested perf data.")
	workRoot     = flag.String("work_root", "", "Directory location where all the work is done.")
	subdomain    = flag.String("subdomain", "android-ingest", "The subdomain [foo].skia.org of where this app is running.")
	authorEmail  = flag.String("author_email", "skia-android-ingest@skia-public.iam.gserviceaccount.com", "Email address of the git author.")
)

var (
	templates         *template.Template
	bucket            *storage.BucketHandle
	gcsPath           string
	converter         *parser.Converter
	process           *continuous.Process
	recentRequests    *recent.Recent
	api               *buildapi.API
	uploads           metrics2.Counter
	badFiles          metrics2.Counter
	txLogWriteFailure metrics2.Counter

	lookupCache *lookup.Cache
)

func initialize() {
	ctx := context.Background()
	loadTemplates()

	txLogWriteFailure = metrics2.GetCounter("tx_log_write_failure", nil)
	uploads = metrics2.GetCounter("uploads", nil)
	badFiles = metrics2.GetCounter("bad_files", nil)
	// Create a new auth'd client for androidbuildinternal.
	ts, err := auth.NewDefaultTokenSource(*local, androidbuildinternal.AndroidbuildInternalScope, storage.ScopeReadWrite, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatalf("Unable to create authenticated token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithoutRetries().WithTokenSource(ts).Client()

	if err := os.MkdirAll(*workRoot, 0755); err != nil {
		sklog.Fatalf("Failed to create directory %q: %s", *workRoot, err)
	}

	if !*local {
		if _, err := gitauth.New(ts, "/tmp/git-cookie", true, *authorEmail); err != nil {
			sklog.Fatal(err)
		}
	}

	// The repo we're adding commits to.
	checkout, err := git.NewCheckout(ctx, *repoURL, *workRoot)
	if err != nil {
		sklog.Fatalf("Unable to create the checkout of %q at %q: %s", *repoURL, *workRoot, err)
	}
	if err := checkout.UpdateBranch(ctx, git.MainBranch); err != nil {
		sklog.Fatalf("Unable to update the checkout of %q at %q: %s", *repoURL, *workRoot, err)
	}

	// checkout isn't go routine safe, but lookup.New() only uses it in New(), so this
	// is safe, i.e. when we later pass checkout to continuous.New().
	lookupCache, err = lookup.New(ctx, checkout)
	if err != nil {
		sklog.Fatalf("Failed to create buildid lookup cache: %s", err)
	}

	api, err = buildapi.NewAPI(client)
	if err != nil {
		sklog.Fatalf("Failed to construct buildapi.API: %s", err)
	}

	// Start process that adds buildids to the git repo.
	process, err = continuous.New(checkout, lookupCache, client, *local, *subdomain)
	if err != nil {
		sklog.Fatalf("Failed to start continuous process of adding new buildids to git repo: %s", err)
	}
	process.Start(ctx)

	storageHTTPClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(storageHTTPClient))
	if err != nil {
		sklog.Fatalf("Problem creating storage client: %s", err)
	}
	gsURL, err := url.Parse(*storageURL)
	if err != nil {
		sklog.Fatalf("--storage_url value %q is not a valid URL: %s", *storageURL, err)
	}
	bucket = storageClient.Bucket(gsURL.Host)
	gcsPath = gsURL.Path
	if strings.HasPrefix(gcsPath, "/") {
		gcsPath = gcsPath[1:]
	}

	recentRequests = recent.New()

	converter = parser.New(lookupCache)
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func badRequest(w http.ResponseWriter, r *http.Request, err error, message string) {
	sklog.Warning(message, err)
	w.WriteHeader(http.StatusBadRequest)
	_, err = fmt.Fprintf(w, "%s: %s", message, err)
	if err != nil {
		sklog.Errorf("Failed to write badRequest response: %s", err)
	}
}

// UploadHandler handles POSTs of images to be analyzed.
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	uploads.Inc(1)

	// Parse incoming JSON.
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		badRequest(w, r, err, "Failed to read body.")
		recentRequests.AddBad(b, "Failed to read body")
		badFiles.Inc(1)
		return
	}
	// Write the data to the transaction log before even attempting to parse.
	txLogName := upload.LogPath(filepath.Join(gcsPath, txLogDir), time.Now().UTC(), b)
	writer := bucket.Object(txLogName).NewWriter(context.Background())
	if _, err := writer.Write(b); err != nil {
		sklog.Errorf("Failed to create a log entry for incoming JSON data: %s", err)
		txLogWriteFailure.Inc(1)
	}
	util.Close(writer)

	// Convert to benchData.
	buf := bytes.NewBuffer(b)
	benchData, err := converter.Convert(buf, txLogName)
	if err == parser.ErrIgnorable {
		return
	}
	if err != nil {
		sklog.Errorf("Failed to find valid incoming JSON in: %q : %s", txLogName, err)
		recentRequests.AddBad(b, err.Error())
		badFiles.Inc(1)
		return
	}

	// Record the tx_log for traceability.
	benchData.Source = txLogName

	// Write the benchData out as JSON in the right spot in Google Storage.
	writer = bucket.Object(upload.ObjectPath(benchData, gcsPath, time.Now().UTC(), b)).NewWriter(context.Background())
	b, err = json.MarshalIndent(benchData, "", "  ")
	if err != nil {
		badRequest(w, r, err, "Failed to encode benchData as JSON.")
		return
	}
	if _, err := writer.Write(b); err != nil {
		badRequest(w, r, err, "Failed to write JSON body.")
		return
	}
	util.Close(writer)

	// Store locally.
	recentRequests.AddGood(b)
}

// IndexContext is the data passed to the index.html template.
type IndexContext struct {
	RecentGood  []*recent.Request
	RecentBad   []*recent.Request
	LastBuildID int64
}

// indexHandler displays the main page with the last MAX_RECENT Requests.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := login.LoggedInAs(r)
	if !*local && user == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
		return
	}
	if *local {
		loadTemplates()
	}

	var lastBuildID int64 = -1
	// process is nil when testing.
	if process != nil {
		lastBuildID, _, _, _ = process.Last(context.Background())
	}

	good, bad := recentRequests.List()
	indexContent := &IndexContext{
		RecentGood:  good,
		RecentBad:   bad,
		LastBuildID: lastBuildID,
	}

	if err := templates.ExecuteTemplate(w, "index.html", indexContent); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// rangeRedirectHandler handles the commit range links that we added to cluster-summary2-sk and redirects
// them to the android-build dashboard.
func rangeRedirectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	begin := mux.Vars(r)["begin"]
	end := mux.Vars(r)["end"]

	if begin == "" || end == "" {
		http.NotFound(w, r)
		return
	}
	ctx := context.Background()
	beginID, err := process.Repo.LookupBuildID(ctx, begin)
	if err != nil {
		httputils.ReportError(w, err, "Failed looking up Build ID.", http.StatusInternalServerError)
		return
	}
	endID, err := process.Repo.LookupBuildID(ctx, end)
	if err != nil {
		httputils.ReportError(w, err, "Failed looking up Build ID.", http.StatusInternalServerError)
		return
	}
	redirBranch, err := api.GetBranchFromBuildID(beginID)
	if err != nil {
		httputils.ReportError(w, err, "Failed looking up Branch.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("https://android-build.googleplex.com/builds/%d/branches/%s/cls?end=%d", beginID, redirBranch, endID), http.StatusFound)
}

// redirectHandler handles the links that we added to the git repo and redirects
// them to the source android-build dashboard.
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	redirBranch := r.FormValue("branch")

	// If this is an old link, before we recorded branches, then we want to link to the old master branch.
	if redirBranch == "" {
		redirBranch = "git_master-skia"
	}
	http.Redirect(w, r, fmt.Sprintf("https://android-build.googleplex.com/builds/branches/%s/grid?head=%s&tail=%s", redirBranch, id, id), http.StatusFound)
}

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
	))
}

func main() {
	common.InitWithMust(
		filepath.Base(os.Args[0]),
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	if *workRoot == "" {
		sklog.Fatal("The --work_root flag must be supplied.")
	}
	if *repoURL == "" {
		sklog.Fatal("The --repo_url flag must be supplied.")
	}
	if !*local {
		login.SimpleInitWithAllow(*port, *local, nil, nil, allowed.Googlers())
	}

	initialize()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/upload", UploadHandler).Methods("POST")
	r.HandleFunc("/r/{id:[a-zA-Z0-9]+}", redirectHandler)
	r.HandleFunc("/rr/{begin:[a-zA-Z0-9]+}/{end:[a-zA-Z0-9]+}", rangeRedirectHandler)
	r.HandleFunc("/", indexHandler)

	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
