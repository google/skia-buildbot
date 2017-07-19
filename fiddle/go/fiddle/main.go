// fiddle is the web server for fiddle.
package main

import (
	"encoding/base64"
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

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	"go.skia.org/infra/fiddle/go/buildlib"
	"go.skia.org/infra/fiddle/go/buildsecwrap"
	"go.skia.org/infra/fiddle/go/named"
	"go.skia.org/infra/fiddle/go/runner"
	"go.skia.org/infra/fiddle/go/source"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	FIDDLE_HASH_LENGTH = 32
)

// flags
var (
	fiddleRoot        = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	preserveTemp      = flag.Bool("preserve_temp", false, "If true then preserve the build artifacts in the fiddle/tmp directory. Used for debugging only.")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
	tryNamed          = flag.Bool("try_named", true, "Start the Go routine that periodically tries all the named fiddles.")
)

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

	defaultFiddle *types.FiddleContext = &types.FiddleContext{
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
	context := &types.FiddleContext{
		Hash:    id,
		Code:    code,
		Options: *options,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := templates.ExecuteTemplate(w, "iframe.html", context); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func loadContext(w http.ResponseWriter, r *http.Request) (*types.FiddleContext, error) {
	id := mux.Vars(r)["id"]
	fiddleHash, err := names.DereferenceID(id)
	if err != nil {
		return nil, fmt.Errorf("Invalid id: %s", err)
	}
	if *local {
		loadTemplates()
	}
	code, options, err := fiddleStore.GetCode(fiddleHash)
	if err != nil {
		return nil, fmt.Errorf("Fiddle not found.")
	}
	return &types.FiddleContext{
		Build:   build.Current(),
		Sources: src.ListAsJSON(),
		Hash:    id,
		Code:    code,
		Options: *options,
	}, nil
}

// embedHandle returns a JSON description of a fiddle.
func embedHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	context, err := loadContext(w, r)
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Failed to load context: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(context); err != nil {
		httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
	}
}

// individualHandle handles permalinks to individual fiddles.
func individualHandle(w http.ResponseWriter, r *http.Request) {
	context, err := loadContext(w, r)
	if err != nil {
		http.NotFound(w, r)
		sklog.Errorf("Failed to load context: %s", err)
		return
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
	w.Header().Add("Access-Control-Allow-Origin", "*")
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
	w.Header().Add("Access-Control-Allow-Origin", "*")
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
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET")
	if r.Method == "OPTIONS" {
		return
	}
	req := &types.FiddleContext{}
	dec := json.NewDecoder(r.Body)
	defer util.Close(r.Body)
	if err := dec.Decode(req); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode request.")
		return
	}

	resp, err := run(login.LoggedInAs(r), req)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to run the fiddle.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
	}
}

func run(user string, req *types.FiddleContext) (*types.RunResults, error) {
	resp := &types.RunResults{
		CompileErrors: []types.CompileError{},
		FiddleHash:    "",
	}
	if err := runner.ValidateOptions(&req.Options); err != nil {
		return resp, fmt.Errorf("Invalid Options: %s", err)
	}
	sklog.Infof("Request: %#v", *req)

	// The fast path returns quickly if the fiddle already exists.
	if req.Fast {
		sklog.Infof("Trying the fast path.")
		if fiddleHash, err := req.Options.ComputeHash(req.Code); err == nil {
			if _, _, err := fiddleStore.GetCode(fiddleHash); err == nil {
				resp.FiddleHash = fiddleHash
				if req.Options.TextOnly {
					if b, _, _, err := fiddleStore.GetMedia(fiddleHash, store.TXT); err != nil {
						sklog.Infof("Failed to get text: %s", err)
					} else {
						resp.Text = string(b)
						return resp, nil
					}
				} else {
					return resp, nil
				}
			} else {
				sklog.Infof("Failed to match hash: %s", err)
			}
		} else {
			sklog.Infof("Failed to compute hash: %s", err)
		}
	}

	current := build.Current()
	sklog.Infof("Building at: %s", current.Hash)
	checkout := filepath.Join(*fiddleRoot, "versions", current.Hash)
	tmpDir, err := runner.WriteDrawCpp(checkout, *fiddleRoot, req.Code, &req.Options)
	if err != nil {
		return resp, fmt.Errorf("Failed to write the fiddle.")
	}
	res, err := runner.Run(checkout, *fiddleRoot, depotTools, current.Hash, *local, tmpDir, &req.Options)
	if !*local && !*preserveTemp {
		if err := os.RemoveAll(tmpDir); err != nil {
			sklog.Errorf("Failed to remove temp dir: %s", err)
		}
	}
	if err != nil {
		return resp, fmt.Errorf("Failed to run the fiddle")
	}
	maybeSecViolation := false
	if res.Execute.Errors != "" {
		sklog.Infof("Execution errors: %q", res.Execute.Errors)
		maybeSecViolation = true
		resp.RunTimeError = "Failed to run, possibly violated security container."
	}
	// Take the compiler output and strip off all the implementation dependant information
	// and format it to be retured in types.RunResults.
	if res.Compile.Errors != "" {
		lines := strings.Split(res.Compile.Output, "\n")
		for _, line := range lines {
			match := parseCompilerOutput.FindAllStringSubmatch(line, -1)
			if match == nil || len(match[0]) < 5 {
				resp.CompileErrors = append(resp.CompileErrors, types.CompileError{
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
			resp.CompileErrors = append(resp.CompileErrors, types.CompileError{
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
	fiddleHash := ""
	// Store the fiddle, but only if we are not in Fast mode and errors occurred.
	if !(res == nil && req.Fast) {
		fiddleHash, err = fiddleStore.Put(req.Code, req.Options, current.Hash, current.Timestamp, res)
		if err != nil {
			return resp, fmt.Errorf("Failed to store the fiddle")
		}
	}
	if maybeSecViolation {
		maybeSecViolations.Inc(1)
		sklog.Warningf("Attempted Security Container Violation for https://fiddle.skia.org/c/%s", fiddleHash)
	}
	runs.Inc(1)
	resp.FiddleHash = fiddleHash

	if req.Options.TextOnly && res != nil {
		// decode
		decodedText, err := base64.StdEncoding.DecodeString(res.Execute.Output.Text)
		if err != nil {
			return resp, fmt.Errorf("Text wasn't properly encoded base64: %s", err)
		}
		resp.Text = string(decodedText)
	}

	// Only logged in users can create named fiddles.
	if req.Name != "" && user != "" && fiddleHash != "" {
		// Create a name for this fiddle. Validation is done in this func.
		err := names.Add(req.Name, fiddleHash, user, req.Overwrite)
		if err == named.DuplicateNameErr {
			return resp, fmt.Errorf("Duplicate fiddle name.")
		}
		if err != nil {
			return resp, fmt.Errorf("Failed to store the name.")
		}
		// Replace fiddleHash with name.
		resp.FiddleHash = "@" + req.Name
	}
	return resp, nil
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
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func singleStepTryNamed() {
	sklog.Infoln("Begin: Try all named fiddles.")
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
		tmpDir, err := runner.WriteDrawCpp(checkout, *fiddleRoot, code, options)
		if err != nil {
			sklog.Errorf("Failed to write fiddle for %s: %s", name.Name, err)
			continue
		}
		res, err := runner.Run(checkout, *fiddleRoot, depotTools, current.Hash, *local, tmpDir, options)
		if err != nil {
			sklog.Errorf("Failed to run fiddle for %s: %s", name.Name, err)
			failing = append(failing, name)
		} else if res.Compile.Errors != "" || res.Execute.Errors != "" {
			sklog.Errorf("Failed to compile or run the named fiddle: %s", name.Name)
			failing = append(failing, name)
		}
		if !*preserveTemp {
			if err := os.RemoveAll(tmpDir); err != nil {
				sklog.Errorf("Failed to remove temp dir: %s", err)
			}
		}
	}
	sklog.Infof("The following named fiddles are failing: %v", failing)
	namedFailures.Reset()
	namedFailures.Inc(int64(len(failing)))
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
		for range time.Tick(24 * time.Hour) {
			singleStepTryNamed()
		}
	}()
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"fiddle",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	login.SimpleInitMust(*port, *local)

	if *fiddleRoot == "" {
		sklog.Fatal("The --fiddle_root flag is required.")
	}
	if !*local {
		if err := buildsecwrap.Build(*fiddleRoot); err != nil {
			sklog.Fatalf("Failed to compile fiddle_secwrap: %s", err)
		}
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
	if *tryNamed {
		StartTryNamed()
	}

	go func() {
		sklog.Fatal(http.ListenAndServe("localhost:6060", nil))
	}()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/i/{id:[@0-9a-zA-Z._]+}", imageHandler)
	r.HandleFunc("/c/{id:[@0-9a-zA-Z_]+}", individualHandle)
	r.HandleFunc("/e/{id:[@0-9a-zA-Z_]+}", embedHandle)
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
