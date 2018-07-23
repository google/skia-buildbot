package main

import (
	"context"
	"flag"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/ct_pixel_diff/go/ctdiffingestion"
	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diffstore"
	gstorage "google.golang.org/api/storage/v1"
)

// Command line flags.
var (
	appTitle           = flag.String("app_title", "CT Pixel Diff", "Title of deployed app on front end")
	boltDir            = flag.String("bolt_dir", "diffs", "Directory that ResultStore uses to store its boltDB instance")
	boltName           = flag.String("bolt_name", "diffs.db", "Name of the boltDB instance of ResultStore")
	cacheSize          = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
	forceLogin         = flag.Bool("force_login", true, "Force the user to be authenticated for all requests.")
	gsBucket           = flag.String("gs_bucket", "cluster-telemetry", "Google storage bucket that holds screenshots from CT.")
	gsBaseDirs         = flag.String("gs_basedirs", "tasks/pixel_diff_runs", "Path of subdirectories after the GS bucket that lead to YYYY/MM/DD directories.")
	imageDir           = flag.String("image_dir", "imagedir", "Directory that DiffStore uses to store screenshots and diff images.")
	ingestDays         = flag.Int("ingest_days", 30, "The number of days in the past that the ingester will consider. (e.g. specifying 30 will make the ingester pull data from 30 days ago to now)")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	port               = flag.String("port", ":8000", "HTTP service address")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	redirectURL        = flag.String("redirect_url", "https://skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
	statusDir          = flag.String("status_dir", "statusdir", "Directory that stores the status for the ingester")
)

// Module level variables.
var (
	templates   *template.Template
	resultStore resultstore.ResultStore
)

const (
	IMAGE_URL_PREFIX = "/img/"
	INGESTER_ID      = "ct-pixel-diff"
	OAUTH2_CALLBACK  = "/oauth2callback/"
)

func main() {

	// Parse the options, so we can configure logging.
	flag.Parse()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}

	// Should we disable cloud logging.
	if !*noCloudLog {
		logOpts = append(logOpts, common.CloudLoggingOpt())
	}
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)

	ctx := context.Background()

	// Get the version of the repo.
	skiaversion.MustLogVersion()

	// Set the resource directory if it's empty.
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
		*resourcesDir += "/frontend"
	}

	// Set up logging in.
	login.SimpleInitMust(*port, *local)

	// Load the frontend templates.
	loadTemplates()

	// Get the client to be used to access GCS.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// Set up the DiffStore.
	mapper := dynamicdiff.NewPixelDiffStoreMapper(&dynamicdiff.DynamicDiffMetrics{})
	diffStore, err := diffstore.NewMemDiffStore(client, *imageDir, []string{*gsBucket}, *gsBaseDirs, *cacheSize, mapper)
	if err != nil {
		sklog.Fatalf("Allocating local DiffStore failed: %s", err)
	}

	// Set up the ingester config.
	ingesterConfig := &sharedconfig.IngesterConfig{
		RunEvery:   config.Duration{Duration: time.Minute},
		MinDays:    *ingestDays,
		StatusDir:  *statusDir,
		MetricName: "ct-pixel-diff-ingest",
	}

	// Instantiate the source for the ingester.
	source, err := ingestion.NewGoogleStorageSource(INGESTER_ID, *gsBucket, *gsBaseDirs, client)
	if err != nil {
		sklog.Fatalf("Unable to initialize source for ingester: %s", err)
	}
	sources := []ingestion.Source{source}

	// Initialize the ResultStore.
	resultStore, err = resultstore.NewBoltResultStore(*boltDir, *boltName)
	if err != nil {
		sklog.Fatalf("Unable to initialize ResultStore: %s", err)
	}

	// Create the processor for the ingester.
	processor, err := ctdiffingestion.NewPixelDiffProcessor(diffStore, resultStore)
	if err != nil {
		sklog.Fatalf("Unable to initialize PixelDiffProcessor: %s", err)
	}

	// Initialize the ingester.
	ingester, err := ingestion.NewIngester(INGESTER_ID, ingesterConfig, nil, sources, processor)
	if err != nil {
		sklog.Fatalf("Unable to initialize Ingester: %s", err)
	}
	ingester.Start(ctx)

	router := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)

	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler(*resourcesDir))

	router.HandleFunc("/", templateHandler("runs.html"))
	router.HandleFunc("/load", templateHandler("results.html"))
	router.HandleFunc("/search", templateHandler("search.html"))
	router.HandleFunc("/stats", templateHandler("stats.html"))
	router.HandleFunc(OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)

	router.HandleFunc("/json/version", skiaversion.JsonHandler)
	router.HandleFunc("/json/runs", jsonRunsHandler).Methods("GET")
	router.HandleFunc("/json/delete", jsonDeleteHandler).Methods("GET")
	router.HandleFunc("/json/render", jsonRenderHandler).Methods("GET")
	router.HandleFunc("/json/sort", jsonSortHandler).Methods("GET")
	router.HandleFunc("/json/urls", jsonURLsHandler).Methods("GET")
	router.HandleFunc("/json/search", jsonSearchHandler).Methods("GET")
	router.HandleFunc("/json/stats", jsonStatsHandler).Methods("GET")

	rootHandler := httputils.LoggingGzipRequestResponse(router)
	if *forceLogin {
		rootHandler = login.ForceAuth(rootHandler, OAUTH2_CALLBACK)
	}
	http.Handle("/", rootHandler)

	// Start the HTTP server.
	sklog.Infoln("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "templates/runs.html"),
		filepath.Join(*resourcesDir, "templates/results.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/search.html"),
		filepath.Join(*resourcesDir, "templates/stats.html"),
	))
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if *local {
			loadTemplates()
		}
		appConfig := &struct {
			Title string `json:"title"`
		}{
			Title: *appTitle,
		}
		if err := templates.ExecuteTemplate(w, name, appConfig); err != nil {
			sklog.Errorln("Failed to expand template:", err)
		}
	}
}
