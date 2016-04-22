// fiddle is the web server for fiddle.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/builder"
	"go.skia.org/infra/fiddle/go/named"
	"go.skia.org/infra/fiddle/go/runner"
	"go.skia.org/infra/fiddle/go/source"
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

const (
	FIDDLE_HASH_LENGTH = 32
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

// FiddleContext is the structure we use for the expanding the index.html template.
//
// It is also used (without the Hash) as the incoming JSON request to /_/run.
type FiddleContext struct {
	Sources   string `json:"sources"`    // All the source image ids serialized as a JSON array.
	Hash      string `json:"fiddlehash"` // Can be the fiddle hash or the fiddle name.
	Code      string `json:"code"`
	Name      string `json:"name"`      // In a request can be the name to create for this fiddle.
	Overwrite bool   `json:"overwrite"` // In a request, should a name be overwritten if it already exists.
	Options   types.Options
}

// CompileError is a single line of compiler error output, along with the line
// and column that the error occurred at.
type CompileError struct {
	Text string `json:"text"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// RunResults is the results we serialize to JSON as the results from a run.
type RunResults struct {
	CompileErrors []CompileError `json:"compile_errors"`
	RunTimeError  string         `json:"runtime_error"`
	FiddleHash    string         `json:"fiddleHash"`
}

var (
	templates *template.Template

	defaultFiddle *FiddleContext = &FiddleContext{
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

	// trailingToMedia maps the end of each image URL to the store.Media type
	// that it corresponds to.
	trailingToMedia = map[string]store.Media{
		"_raster.png": store.CPU,
		"_gpu.png":    store.GPU,
		".pdf":        store.PDF,
		".skp":        store.SKP,
	}

	// parseCompilerOutput parses the compiler output to look for lines
	// that begin with "draw.cpp:<N>:<M>:" where N and M are the line and column
	// number where the error occurred. It also strips off the full path name
	// of draw.cpp.
	//
	// For example if we had the following input line:
	//
	//    "/usr/local../src/draw.cpp:8:5: error: expected ‘)’ before ‘canvas’\n void draw(SkCanvas* canvas) {\n     ^\n"
	//
	// Then the re.FindAllStringSubmatch(s, -1) will return a match of the form:
	//
	//    [][]string{
	//      []string{
	//        "/usr/local.../src/draw.cpp:8:5: error: expected ‘)’ before ‘canvas’",
	//        "/usr/local.../src/",
	//        "draw.cpp:8:5: error: expected ‘)’ before ‘canvas’",
	//        "8",
	//        "5",
	//      },
	//    }
	//
	// Note that slice items 2, 3, and 4 are the ones we are really interested in.
	parseCompilerOutput = regexp.MustCompile("^(.*/)(draw.cpp:([0-9]+):([-0-9]+):.*)")

	buildLiveness    = metrics2.NewLiveness("fiddle.build")
	buildFailures    = metrics2.GetCounter("builds-failed", nil)
	repoSyncFailures = metrics2.GetCounter("repo-sync-failed", nil)
	build            *builder.Builder
	fiddleStore      *store.Store
	repo             *gitinfo.GitInfo
	src              *source.Source
	names            *named.Named
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/iframe.html"),
		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	defaultFiddle.Sources = src.ListAsJSON()
	if err := templates.ExecuteTemplate(w, "index.html", defaultFiddle); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
	}
}

// iframeHandle handles permalinks to individual fiddles.
func iframeHandle(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fiddleHash, err := names.DereferenceID(id)
	if err != nil {
		http.NotFound(w, r)
		glog.Errorf("Invalid id: %s", err)
		return
	}
	if *local {
		loadTemplates()
	}
	code, options, err := fiddleStore.GetCode(fiddleHash)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	context := &FiddleContext{
		Hash:    id,
		Code:    code,
		Options: *options,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "iframe.html", context); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
	}
}

// individualHandle handles permalinks to individual fiddles.
func individualHandle(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fiddleHash, err := names.DereferenceID(id)
	if err != nil {
		http.NotFound(w, r)
		glog.Errorf("Invalid id: %s", err)
		return
	}
	if *local {
		loadTemplates()
	}
	code, options, err := fiddleStore.GetCode(fiddleHash)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	context := &FiddleContext{
		Sources: src.ListAsJSON(),
		Hash:    id,
		Code:    code,
		Options: *options,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "index.html", context); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
	}
}

// imageHandler serves up images from the fiddle store.
//
// The URLs look like:
//
//   /i/cbb8dee39e9f1576cd97c2d504db8eee_raster.png
//   /i/cbb8dee39e9f1576cd97c2d504db8eee_gpu.png
//   /i/cbb8dee39e9f1576cd97c2d504db8eee.pdf
//   /i/cbb8dee39e9f1576cd97c2d504db8eee.skp
//
// or
//
//   /i/@some_name.png
//   /i/@some_name_gpu.png
//   /i/@some_name.pdf
//   /i/@some_name.skp
func imageHandler(w http.ResponseWriter, r *http.Request) {
	fiddleHash, media, err := names.DereferenceImageID(mux.Vars(r)["id"])
	if err != nil {
		http.NotFound(w, r)
		glog.Errorf("Invalid id: %s", err)
		return
	}
	body, contentType, _, err := fiddleStore.GetMedia(fiddleHash, media)
	if err != nil {
		http.NotFound(w, r)
		glog.Errorf("Failed to retrieve media: %s", err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(body); err != nil {
		glog.Errorf("Failed to write image: %s", err)
	}
}

// sourceHandler serves up source image thumbnails.
//
// The URLs look like:
//
//   /s/NNN
//
// Where NNN is the id of the source image.
func sourceHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	i, err := strconv.Atoi(id)
	if err != nil {
		http.NotFound(w, r)
		glog.Errorf("Invalid source id: %s", err)
		return
	}
	b, ok := src.Thumbnail(i)
	if !ok {
		http.NotFound(w, r)
		glog.Errorf("Unknown source id %s", id)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	if _, err := w.Write(b); err != nil {
		glog.Errorf("Failed to write image: %s", err)
		return
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	resp := RunResults{
		CompileErrors: []CompileError{},
		FiddleHash:    "",
	}
	req := &FiddleContext{}
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
	glog.Infof("Building at: %s", gitHash)
	tmpDir, err := runner.WriteDrawCpp(*fiddleRoot, req.Code, &req.Options, *local)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to write the fiddle.")
	}
	res, err := runner.Run(*fiddleRoot, gitHash, *local, tmpDir)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to run the fiddle")
		return
	}
	if res.Execute.Errors != "" {
		glog.Infof("Runtime error: %s", res.Execute.Errors)
		resp.RunTimeError = "Failed to run, possibly violated security container."
	}
	// Take the compiler output and strip off all the implementation dependant information
	// and format it to be retured in RunResults.
	if res.Compile.Errors != "" {
		lines := strings.Split(res.Compile.Output, "\n")
		for _, line := range lines {
			match := parseCompilerOutput.FindAllStringSubmatch(line, -1)
			if match == nil || len(match[0]) < 5 {
				resp.CompileErrors = append(resp.CompileErrors, CompileError{
					Text: line,
					Line: 0,
					Col:  0,
				})
				continue
			}
			line_num, err := strconv.Atoi(match[0][3])
			if err != nil {
				glog.Errorf("Failed to parse compiler output line number: %#v: %s", match, err)
				continue
			}
			col_num, err := strconv.Atoi(match[0][4])
			if err != nil {
				glog.Errorf("Failed to parse compiler output column number: %#v: %s", match, err)
				continue
			}
			resp.CompileErrors = append(resp.CompileErrors, CompileError{
				Text: match[0][2],
				Line: line_num,
				Col:  col_num,
			})
		}
	}
	// Since the compile failed we will only store the code, not the media.
	if res.Compile.Errors != "" || res.Execute.Errors != "" {
		res = nil
	}
	ts := repo.Timestamp(gitHash)
	fiddleHash, err := fiddleStore.Put(req.Code, req.Options, gitHash, ts, res)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to store the fiddle.")
		return
	}
	resp.FiddleHash = fiddleHash

	user := login.LoggedInAs(r)
	// Only logged in users can create named fiddles.
	if req.Name != "" && user != "" {
		// Create a name for this fiddle. Validation is done in this func.
		err := names.Add(req.Name, fiddleHash, user, req.Overwrite)
		if err == named.DuplicateNameErr {
			httputils.ReportError(w, r, err, "Name already exists.")
			return
		}
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to store the name.")
			return
		}
		// Replace fiddleHash with name.
		resp.FiddleHash = "@" + req.Name
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
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
			glog.Errorf("Failed to expand template: %s", err)
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
		repoSyncFailures.Inc(1)
	}
	repoSyncFailures.Reset()
	ci, err := build.BuildLatestSkia(false, false, false)
	if err != nil {
		glog.Errorf("Failed to build LKGR: %s", err)
		// Only measure real build failures, not a failure if LKGR hasn't updated.
		if err != builder.AlreadyExistsErr {
			buildFailures.Inc(1)
		}
		return
	}
	buildFailures.Reset()
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
	if err := fiddleStore.DownloadAllSourceImages(*fiddleRoot); err != nil {
		glog.Fatalf("Failed to download source images: %s", err)
	}
	src, err = source.New(fiddleStore)
	if err != nil {
		glog.Fatalf("Failed to initialize source images: %s", err)
	}
	names = named.New(fiddleStore)
	build = builder.New(*fiddleRoot, *depotTools)
	StartBuilding()
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/i/{id:[@0-9a-zA-Z._]+}", imageHandler)
	r.HandleFunc("/c/{id:[@0-9a-zA-Z_]+}", individualHandle)
	r.HandleFunc("/iframe/{id:[@0-9a-zA-Z_]+}", iframeHandle)
	r.HandleFunc("/s/{id:[0-9]+}", sourceHandler)
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/_/run", runHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
