package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	gstorage "google.golang.org/api/storage/v1"
)

// Command line flags.
var (
	cacheSize          = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
	gsBucketNames      = flag.String("gs_buckets", "skia-infra-gm,chromium-skia-gm", "Comma-separated list of google storage bucket that hold uploaded images.")
	imageDir           = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	port               = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

const (
	IMAGE_URL_PREFIX = "/img/"
)

// diffStore handles all the diffing.
var diffStore diff.DiffStore = nil

func main() {
	defer common.LogPanic()
	var err error

	mainTimer := timer.New("main init")

	// Parse the options. So we can configure logging.
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

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatalf("Unable to retrieve version: %s", err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// Get the client to be used to access GCS.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// Get the expecations storage, the filediff storage and the tilestore.
	diffStore, err := diffstore.New(client, *imageDir, strings.Split(*gsBucketNames, ","), diffstore.DEFAULT_GCS_IMG_DIR_NAME, *cacheSize)
	if err != nil {
		sklog.Fatalf("Allocating DiffStore failed: %s", err)
	}
	mainTimer.Stop()

	router := mux.NewRouter()

	// Set up the resource to serve the image files.
	imgHandler, err := diffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)

	router.HandleFunc("/v1/getdiffs", jsonGetDiffsHandler).Methods("GET")
	router.HandleFunc("/v1/warm", jsonWarmHandler).Methods("GET")
	router.HandleFunc("/v1/warmdiffs", jsonWarmDiffsHandler).Methods("GET")
	router.HandleFunc("/v1/unavailable", jsonUnavailableHandler).Methods("GET")
	router.HandleFunc("/v1/purge", jsonPurgeHandler).Methods("GET")
	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	// Start the server
	sklog.Infoln("Serving on http://127.0.0.1" + *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func jsonGetDiffsHandler(w http.ResponseWriter, r *http.Request) {
	digest := ""
	compareTo := []string{}
	ret, err := diffStore.Get(diff.PRIORITY_NOW, digest, compareTo)
	if err != nil {
		httputils.ReportError(w, r, err, "Get failed: "+err.Error())
	}
	sendJsonResponse(w, ret)
}

func jsonWarmHandler(w http.ResponseWriter, r *http.Request) {

}

func jsonWarmDiffsHandler(w http.ResponseWriter, r *http.Request) {

}

func jsonUnavailableHandler(w http.ResponseWriter, r *http.Request) {

}

func jsonPurgeHandler(w http.ResponseWriter, r *http.Request) {

}

// sendJsonResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJsonResponse(w http.ResponseWriter, resp interface{}) {
	h := w.Header()
	h.Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, nil, err, "Failed to encode JSON response.")
	}
}
