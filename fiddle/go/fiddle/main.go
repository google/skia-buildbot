// fiddle is the web server for fiddle.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	ttemplate "html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/fiddle/go/buildlib"
	"go.skia.org/infra/fiddle/go/named"
	"go.skia.org/infra/fiddle/go/runner"
	"go.skia.org/infra/fiddle/go/source"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	FIDDLE_HASH_LENGTH = 32
)

// flags
var (
	fiddleRoot        = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	influxDatabase    = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost        = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword    = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser        = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	preserveTemp      = flag.Bool("preserve_temp", false, "If true then preserve the build artifacts in the fiddle/tmp directory. Used for debugging only.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
)

// FiddleContext is the structure we use for the expanding the index.html template.
//
// It is also used (without the Hash) as the incoming JSON request to /_/run.
type FiddleContext struct {
	Build     *vcsinfo.LongCommit `json:"build"`      // The version of Skia this was run on.
	Sources   string              `json:"sources"`    // All the source image ids serialized as a JSON array.
	Hash      string              `json:"fiddlehash"` // Can be the fiddle hash or the fiddle name.
	Code      string              `json:"code"`
	Name      string              `json:"name"`      // In a request can be the name to create for this fiddle.
	Overwrite bool                `json:"overwrite"` // In a request, should a name be overwritten if it already exists.
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

	funcMap = ttemplate.FuncMap{
		"chop": func(s string) string {
			if len(s) > 6 {
				return s[:6]
			}
			return s
		},
	}

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
	namedFailures       = metrics2.GetCounter("named-failures", nil)
	maybeSecViolations  = metrics2.GetCounter("maybe-sec-container-violation", nil)
	runs                = metrics2.GetCounter("runs", nil)
	tryNamedLiveness    = metrics2.NewLiveness("try-named")

	build        *buildskia.ContinuousBuilder
	fiddleStore  *store.Store
	repo         *gitinfo.GitInfo
	src          *source.Source
	names        *named.Named
	failingNamed = []store.Named{}
	failingMutex = sync.Mutex{}
	depotTools   string
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").Funcs(funcMap).ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/iframe.html"),
		filepath.Join(*resourcesDir, "templates/failing.html"),
		filepath.Join(*resourcesDir, "templates/named.html"),
		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/menu.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	cp := *defaultFiddle
	cp.Sources = src.ListAsJSON()
	cp.Build = build.Current()
	if err := templates.ExecuteTemplate(w, "index.html", cp); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func failedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	failingMutex.Lock()
	defer failingMutex.Unlock()
	if err := templates.ExecuteTemplate(w, "failing.html", failingNamed); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func namedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	named, err := fiddleStore.ListAllNames()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve list of named fiddles.")
	}
	if err := templates.ExecuteTemplate(w, "named.html", named); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// iframeHandle handles permalinks to individual fiddles.
func iframeHandle(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fiddleHash, err := names.DereferenceID(id)
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Invalid id: %s", err)
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
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// individualHandle handles permalinks to individual fiddles.
func individualHandle(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	fiddleHash, err := names.DereferenceID(id)
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Invalid id: %s", err)
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
		Build:   build.Current(),
		Sources: src.ListAsJSON(),
		Hash:    id,
		Code:    code,
		Options: *options,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "index.html", context); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
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
	id := mux.Vars(r)["id"]
	fiddleHash, media, err := names.DereferenceImageID(id)
	if fiddleHash == id {
		w.Header().Add("Cache-Control", "max-age=36000")
	}
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Invalid id: %s", err)
		return
	}
	body, contentType, _, err := fiddleStore.GetMedia(fiddleHash, media)
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Failed to retrieve media: %s", err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(body); err != nil {
		sklog.Errorf("Failed to write image: %s", err)
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
		sklog.Errorf("Invalid source id: %s", err)
		return
	}
	b, ok := src.Thumbnail(i)
	if !ok {
		http.NotFound(w, r)
		sklog.Errorf("Unknown source id %s", id)
		return
	}
	w.Header().Add("Cache-Control", "max-age=360000")
	w.Header().Set("Content-Type", "image/png")
	if _, err := w.Write(b); err != nil {
		sklog.Errorf("Failed to write image: %s", err)
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
	sklog.Infof("Request: %#v", *req)
	current := build.Current()
	sklog.Infof("Building at: %s", current.Hash)
	checkout := filepath.Join(*fiddleRoot, "versions", current.Hash)
	tmpDir, err := runner.WriteDrawCpp(checkout, *fiddleRoot, req.Code, &req.Options, *local)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to write the fiddle.")
	}
	res, err := runner.Run(checkout, *fiddleRoot, depotTools, current.Hash, *local, tmpDir)
	if !*local && !*preserveTemp {
		if err := os.RemoveAll(tmpDir); err != nil {
			sklog.Errorf("Failed to remove temp dir: %s", err)
		}
	}
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to run the fiddle")
		return
	}
	maybeSecViolation := false
	if res.Execute.Errors != "" {
		maybeSecViolation = true
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
				sklog.Errorf("Failed to parse compiler output line number: %#v: %s", match, err)
				continue
			}
			col_num, err := strconv.Atoi(match[0][4])
			if err != nil {
				sklog.Errorf("Failed to parse compiler output column number: %#v: %s", match, err)
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
	fiddleHash, err := fiddleStore.Put(req.Code, req.Options, current.Hash, current.Timestamp, res)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to store the fiddle.")
		return
	}
	if maybeSecViolation {
		maybeSecViolations.Inc(1)
		sklog.Warningf("Attempted Security Container Violation for https://fiddle.skia.org/c/%s", fiddleHash)
	}
	runs.Inc(1)
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
			sklog.Errorf("Failed to expand template: %s", err)
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

func singleStepTryNamed() {
	sklog.Infoln("Begin: Try all named fiddles.")
	namedFailures.Reset()
	allNames, err := fiddleStore.ListAllNames()
	if err != nil {
		sklog.Errorf("Failed to list all named fiddles: %s", err)
		return
	}
	failing := []store.Named{}
	current := build.Current()
	for _, name := range allNames {
		sklog.Infof("Trying: %s", name.Name)
		fiddleHash, err := names.DereferenceID("@" + name.Name)
		if err != nil {
			sklog.Errorf("Can't dereference %s: %s", name.Name, err)
			continue
		}
		code, options, err := fiddleStore.GetCode(fiddleHash)
		if err != nil {
			sklog.Errorf("Can't get code for %s: %s", name.Name, err)
			continue
		}
		checkout := filepath.Join(*fiddleRoot, "versions", current.Hash)
		tmpDir, err := runner.WriteDrawCpp(checkout, *fiddleRoot, code, options, *local)
		if err != nil {
			sklog.Errorf("Failed to write fiddle for %s: %s", name.Name, err)
			continue
		}
		res, err := runner.Run(checkout, *fiddleRoot, depotTools, current.Hash, *local, tmpDir)
		if err != nil {
			sklog.Errorf("Failed to run fiddle for %s: %s", name.Name, err)
			namedFailures.Inc(1)
			failing = append(failing, name)
			continue
		}
		if res.Compile.Errors != "" || res.Execute.Errors != "" {
			sklog.Errorf("Failed to compile or run the named fiddle: %s", name.Name)
			namedFailures.Inc(1)
			failing = append(failing, name)
		}
		if !*local && !*preserveTemp {
			if err := os.RemoveAll(tmpDir); err != nil {
				sklog.Errorf("Failed to remove temp dir: %s", err)
			}
		}
	}
	sklog.Infof("The following named fiddles are failing: %v", failing)
	tryNamedLiveness.Reset()
	failingMutex.Lock()
	defer failingMutex.Unlock()
	failingNamed = failing
}

// StartTryNamed starts the Go routine that daily tests all of the named
// fiddles and reports the ones that fail to build or run.
func StartTryNamed() {
	go func() {
		singleStepTryNamed()
		for _ = range time.Tick(24 * time.Hour) {
			singleStepTryNamed()
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
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://fiddle.skia.org/oauth2callback/"
	}
	if err := login.Init(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}
	if *fiddleRoot == "" {
		sklog.Fatal("The --fiddle_root flag is required.")
	}
	depotTools = filepath.Join(*fiddleRoot, "depot_tools")
	loadTemplates()
	var err error
	repo, err = gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*fiddleRoot, "skia"), true)
	if err != nil {
		sklog.Fatalf("Failed to clone Skia: %s", err)
	}
	fiddleStore, err = store.New()
	if err != nil {
		sklog.Fatalf("Failed to connect to store: %s", err)
	}
	if err := fiddleStore.DownloadAllSourceImages(*fiddleRoot); err != nil {
		sklog.Fatalf("Failed to download source images: %s", err)
	}
	src, err = source.New(fiddleStore)
	if err != nil {
		sklog.Fatalf("Failed to initialize source images: %s", err)
	}
	names = named.New(fiddleStore)
	build = buildskia.New(*fiddleRoot, depotTools, repo, buildlib.BuildLib, 64, *timeBetweenBuilds, true)
	build.Start()
	StartTryNamed()
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/i/{id:[@0-9a-zA-Z._]+}", imageHandler)
	r.HandleFunc("/c/{id:[@0-9a-zA-Z_]+}", individualHandle)
	r.HandleFunc("/iframe/{id:[@0-9a-zA-Z_]+}", iframeHandle)
	r.HandleFunc("/s/{id:[0-9]+}", sourceHandler)
	r.HandleFunc("/f/", failedHandler)
	r.HandleFunc("/named/", namedHandler)
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/_/run", runHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
