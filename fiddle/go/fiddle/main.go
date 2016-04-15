// fiddle is the web server for fiddle.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/builder"
	"go.skia.org/infra/fiddle/go/runner"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

// flags
var (
	depotTools        = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	fiddleRoot        = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	influxDatabase    = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost        = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword    = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser        = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
)

// FiddleContent is the serialized structure the we receive fiddle requests in.
type FiddleContent struct {
	Code    string `json:"code"`
	Options types.Options
}

// RunResults is the results we serialize to JSON as the results from a run.
type RunResults struct {
	Errors     string `json:"errors"`
	FiddleHash string `json:"fiddleHash"`
}

var (
	templates *template.Template

	defaultFiddle *FiddleContent = &FiddleContent{
		Code: `void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style);
    p.setStrokeWidth(10);

    canvas->drawLine(20, 20, 100, 100, p);
}`,
		Options: types.Options{
			Width:  256,
			Height: 256,
			Source: 0,
		},
	}

	buildLiveness = metrics2.NewLiveness("fiddle.build")
	build         *builder.Builder
	fiddleStore   *store.Store
	repo          *gitinfo.GitInfo
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := templates.ExecuteTemplate(w, "index.html", defaultFiddle); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	req := &FiddleContent{}
	dec := json.NewDecoder(r.Body)
	defer util.Close(r.Body)
	if err := dec.Decode(req); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode request.")
		return
	}
	glog.Infof("Request: %#v", *req)
	allBuilds, err := build.AvailableBuilds()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get list of available builds.")
		return
	}
	if len(allBuilds) == 0 {
		httputils.ReportError(w, r, fmt.Errorf("List of available builds is empty."), "No builds available.")
		return
	}
	gitHash := allBuilds[0]
	tmpDir, err := runner.WriteDrawCpp(*fiddleRoot, req.Code, &req.Options, *local)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to write the fiddle.")
	}
	res, err := runner.Run(*fiddleRoot, gitHash, *local, tmpDir)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to run the fiddle")
		return
	}
	ts := repo.Timestamp(gitHash)
	fiddleHash, err := fiddleStore.Put(req.Code, req.Options, gitHash, ts, res)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to store the fiddle")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	resp := RunResults{
		Errors:     res.Errors,
		FiddleHash: fiddleHash,
	}
	if err := enc.Encode(resp); err != nil {
		httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
	}
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if *local {
			loadTemplates()
		}
		if err := templates.ExecuteTemplate(w, name, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func singleBuildLatest() {
	if err := repo.Update(true, true); err != nil {
		glog.Errorf("Failed to update skia repo used to look up git hashes: %s", err)
	}
	ci, err := build.BuildLatestSkia(false, false, false)
	if err != nil {
		glog.Errorf("Failed to build LKGR: %s", err)
		return
	}
	buildLiveness.Reset()
	glog.Infof("Successfully built: %s %s", ci.Hash, ci.Subject)
}

// StartBuilding starts a Go routine that periodically tries to
// download the Skia LKGR and build it.
func StartBuilding() {
	go func() {
		singleBuildLatest()
		for _ = range time.Tick(*timeBetweenBuilds) {
			singleBuildLatest()
		}
	}()
}

func main() {
	defer common.LogPanic()
	if *local {
		common.Init()
	} else {
		common.InitWithMetrics2("fiddle", influxHost, influxUser, influxPassword, influxDatabase, local)
	}
	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://fiddle.skia.org/oauth2callback/"
	}
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatalf("Failed to initialize the login system: %s", err)
	}
	if *fiddleRoot == "" {
		glog.Fatal("The --fiddle_root flag is required.")
	}
	if *depotTools == "" {
		glog.Fatal("The --depot_tools flag is required.")
	}
	loadTemplates()
	var err error
	repo, err = gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*fiddleRoot, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to clone Skia: %s", err)
	}
	fiddleStore, err = store.New()
	if err != nil {
		glog.Fatalf("Failed to connect to store: %s", err)
	}
	build = builder.New(*fiddleRoot, *depotTools)
	StartBuilding()
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/_/run", runHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
