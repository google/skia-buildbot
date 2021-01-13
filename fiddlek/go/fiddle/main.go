// fiddle is the web server for fiddle.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	ttemplate "html/template"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/gorilla/mux"
	"go.opencensus.io/trace"
	"go.skia.org/infra/fiddlek/go/named"
	"go.skia.org/infra/fiddlek/go/runner"
	"go.skia.org/infra/fiddlek/go/source"
	"go.skia.org/infra/fiddlek/go/store"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/scrap/go/client"
	"go.skia.org/infra/scrap/go/scrap"
)

// flags
var (
	distDir        = flag.String("dist_dir", ".", "The directory to find dist/, which contains templates, JS, and CSS files as producted by webpack.")
	fiddleRoot     = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	scrapExchange  = flag.String("scrapexchange", "scrapexchange:9000", "Scrap exchange service HTTP address.")
	sourceImageDir = flag.String("source_image_dir", "./source", "The directory to load the source images from.")
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
	namedFailures       = metrics2.GetCounter("named_failures", nil)
	maybeSecViolations  = metrics2.GetCounter("maybe_sec_container_violation", nil)
	runs                = metrics2.GetCounter("runs", nil)
	tryNamedLiveness    = metrics2.NewLiveness("try_named")

	fiddleStore  store.Store
	src          *source.Source
	names        *named.Named
	failingNamed = []store.Named{}
	failingMutex = sync.Mutex{}
	run          *runner.Runner
	scrapClient  scrap.ScrapExchange
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").Funcs(funcMap).ParseFiles(
		filepath.Join(*distDir, "dist/newindex.html"),
		filepath.Join(*distDir, "dist/named.html"),
	))
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	cp := *defaultFiddle
	cp.Sources = src.ListAsJSON()
	cp.Version = run.Version()
	if err := templates.ExecuteTemplate(w, "newindex.html", cp); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

type namedContext struct {
	Title string
	Named []store.Named
}

func failedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	failingMutex.Lock()
	defer failingMutex.Unlock()
	templateContext := namedContext{
		Title: "Failing Named Fiddles",
		Named: failingNamed,
	}

	if err := templates.ExecuteTemplate(w, "named.html", templateContext); err != nil {
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
		httputils.ReportError(w, err, "Failed to retrieve list of named fiddles.", http.StatusInternalServerError)
	}
	templateContext := namedContext{
		Title: "Named Fiddles",
		Named: named,
	}
	if err := templates.ExecuteTemplate(w, "named.html", templateContext); err != nil {
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
		Version: run.Version(),
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
		httputils.ReportError(w, err, "Failed to JSON Encode response.", http.StatusInternalServerError)
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
	if err := templates.ExecuteTemplate(w, "newindex.html", context); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// scrapHandler handles links to scrap exchange expanded templates and turns them into fiddles.
func scrapHandler(w http.ResponseWriter, r *http.Request) {
	// Load the scrap.
	typ := scrap.ToType(mux.Vars(r)["type"])
	hashOrName := mux.Vars(r)["hashOrName"]
	var b bytes.Buffer
	if err := scrapClient.Expand(r.Context(), typ, hashOrName, scrap.CPP, &b); err != nil {
		httputils.ReportError(w, err, "Failed to load templated scrap.", http.StatusInternalServerError)
		return
	}

	// Create the fiddle.
	fiddleHash, err := fiddleStore.Put(b.String(), defaultFiddle.Options, nil)
	if err != nil {
		httputils.ReportError(w, err, "Failed to write fiddle.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/c/"+fiddleHash, http.StatusTemporaryRedirect)
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
	ctx, span := trace.StartSpan(context.Background(), "fiddleRunHandler")
	defer span.End()

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET")
	if r.Method == "OPTIONS" {
		return
	}

	defer timer.NewWithSummary("latency", metrics2.GetFloat64SummaryMetric("latency", map[string]string{"path": r.URL.Path})).Stop()

	sklog.Infof("RemoteAddr: %s %s", r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
	req := &types.FiddleContext{}
	dec := json.NewDecoder(r.Body)
	defer util.Close(r.Body)
	if err := dec.Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}

	resp, err, msg := runImpl(ctx, req)
	if err != nil {
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to JSON Encode response.", http.StatusInternalServerError)
	}
}

func runImpl(ctx context.Context, req *types.FiddleContext) (*types.RunResults, error, string) {
	ctx, span := trace.StartSpan(ctx, "runImpl")
	defer span.End()

	resp := &types.RunResults{
		CompileErrors: []types.CompileError{},
		FiddleHash:    "",
	}
	if err := run.ValidateOptions(&req.Options); err != nil {
		return resp, fmt.Errorf("Invalid Options: %s", err), "Invalid options."
	}
	sklog.Infof("Request: %#v", *req)
	fiddleHash, err := req.Options.ComputeHash(req.Code)
	if err != nil {
		sklog.Infof("Failed to compute hash: %s", err)
		return resp, fmt.Errorf("Failed to compute hash: %s", err), "Invalid request."
	}

	// The fast path returns quickly if the fiddle already exists.
	if req.Fast {
		sklog.Infof("Trying the fast path.")
		if _, _, err := fiddleStore.GetCode(fiddleHash); err == nil {
			resp.FiddleHash = fiddleHash
			if req.Options.TextOnly {
				if b, _, _, err := fiddleStore.GetMedia(fiddleHash, store.TXT); err != nil {
					sklog.Infof("Failed to get text: %s", err)
				} else {
					resp.Text = string(b)
					return resp, nil, ""
				}
			} else {
				return resp, nil, ""
			}
		} else {
			sklog.Infof("Failed to match hash: %s", err)
		}
	}
	req.Hash = fiddleHash

	res, err := run.Run(ctx, *local, req)
	if err != nil {
		return resp, fmt.Errorf("Failed to run the fiddle: %s", err), "Failed to run the fiddle."
	}
	maybeSecViolation := false
	if res.Execute.Errors != "" {
		sklog.Infof("%q Execution errors: %q", req.Hash, res.Execute.Errors)
		maybeSecViolation = true
		resp.RunTimeError = fmt.Sprintf("Failed to run, possibly violated security container: %q", res.Execute.Errors)
	}
	// Take the compiler output and strip off all the implementation dependant information
	// and format it to be retured in types.RunResults.
	if res.Compile.Output != "" {
		lines := strings.Split(res.Compile.Output, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			match := parseCompilerOutput.FindAllStringSubmatch(line, -1)
			if match == nil || len(match[0]) < 5 {
				// Skip the ninja generated lines.
				if strings.HasPrefix(line, "ninja:") || strings.HasPrefix(line, "[") {
					continue
				}
				resp.CompileErrors = append(resp.CompileErrors, types.CompileError{
					Text: line,
					Line: 0,
					Col:  0,
				})
				continue
			}
			line_num, err := strconv.Atoi(match[0][3])
			if err != nil {
				sklog.Errorf("%q Failed to parse compiler output line number: %#v: %s", req.Hash, match, err)
				continue
			}
			col_num, err := strconv.Atoi(match[0][4])
			if err != nil {
				sklog.Errorf("%q Failed to parse compiler output column number: %#v: %s", req.Hash, match, err)
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
	fiddleHash = ""
	// Store the fiddle, but only if we are not in Fast mode and errors occurred.
	if !(res == nil && req.Fast) {
		fiddleHash, err = fiddleStore.Put(req.Code, req.Options, res)
		if err != nil {
			return resp, fmt.Errorf("Failed to store the fiddle: %s", err), "Failed to store the fiddle."
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
			return resp, fmt.Errorf("Text wasn't properly encoded base64: %s", err), "Failed to base64 decode text result."
		}
		resp.Text = string(decodedText)
	}

	return resp, nil, ""
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

func makeDistHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*distDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func basicModeHandler(w http.ResponseWriter, r *http.Request) {
	// This hash is that of the basic red line that is the starter code.
	// By linking to that, the result shows up for new users/basic mode.
	http.Redirect(w, r, "/c/cbb8dee39e9f1576cd97c2d504db8eee?mode=basic", http.StatusFound)
}

func addHandlers(r *mux.Router) {
	r.PathPrefix("/dist/").HandlerFunc(makeDistHandler())
	r.HandleFunc("/i/{id:[@0-9a-zA-Z._]+}", imageHandler).Methods("GET")
	r.HandleFunc("/c/{id:[@0-9a-zA-Z_]+}", individualHandle).Methods("GET")
	r.HandleFunc("/e/{id:[@0-9a-zA-Z_]+}", embedHandle).Methods("GET")
	r.HandleFunc("/s/{id:[0-9]+}", sourceHandler).Methods("GET")
	r.HandleFunc("/scrap/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}", scrapHandler).Methods("GET")
	r.HandleFunc("/f/", failedHandler).Methods("GET")
	r.HandleFunc("/named/", namedHandler).Methods("GET")
	r.HandleFunc("/new", basicModeHandler).Methods("GET")
	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/_/run", runHandler).Methods("POST")
	r.HandleFunc("/healthz", healthzHandler).Methods("GET")
}

func main() {
	common.InitWithMust(
		"fiddle",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		BundleDelayThreshold: time.Second / 10,
		BundleCountThreshold: 10})
	if err != nil {
		sklog.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	_, span := trace.StartSpan(context.Background(), "main")
	defer span.End()

	loadTemplates()
	fiddleStore, err = store.New(*local)
	if err != nil {
		sklog.Fatalf("Failed to connect to store: %s", err)
	}
	scrapClient, err = client.New(*scrapExchange)
	if err != nil {
		sklog.Fatalf("Failed to create scrap exchange client: %s", err)
	}
	run, err = runner.New(*local, *sourceImageDir)
	if err != nil {
		sklog.Fatalf("Failed to initialize runner: %s", err)
	}
	go run.Metrics()
	src, err = source.New(*sourceImageDir)
	if err != nil {
		sklog.Fatalf("Failed to initialize source images: %s", err)
	}
	names = named.New(fiddleStore)

	r := mux.NewRouter()
	addHandlers(r)

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.CrossOriginResourcePolicy(h)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
