package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
)

// flags
var (
	depotTools        = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	influxDatabase    = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost        = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword    = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser        = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
	workRoot          = flag.String("work_root", "", "Directory location where all the work is done.")
)

var (
	templates *template.Template

	// repo is the Skia checkout.
	repo *gitinfo.GitInfo

	// build is responsible to building the LKGR of skiaserve periodically.
	build *buildskia.ContinuousBuilder
)

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
	))
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if *local {
			loadTemplates()
		}
		if err := templates.ExecuteTemplate(w, name, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func buildSkiaServe(checkout, depotTools string) error {
	// Do a gyp build of skiaserve.
	glog.Info("Starting build of skiaserve")
	if err := buildskia.NinjaBuild(checkout, depotTools, []string{}, buildskia.BUILD_TYPE, "skiaserve", runtime.NumCPU(), true); err != nil {
		return fmt.Errorf("Failed to build: %s", err)
	}
	return nil
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("debugger", influxHost, influxUser, influxPassword, influxDatabase, local)
	if *workRoot == "" {
		glog.Fatal("The --work_root flag is required.")
	}
	if *depotTools == "" {
		glog.Fatal("The --depot_tools flag is required.")
	}
	Init()

	var err error
	repo, err = gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*workRoot, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to clone Skia: %s", err)
	}
	build = buildskia.New(*workRoot, *depotTools, repo, buildSkiaServe, 64, *timeBetweenBuilds)
	build.Start()

	router := mux.NewRouter()
	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	router.HandleFunc("/", templateHandler("index.html"))
	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
