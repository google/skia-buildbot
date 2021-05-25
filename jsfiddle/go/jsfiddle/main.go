package main

// The webserver for jsfiddle.skia.org. It serves up the web page

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/jsfiddle/go/store"
)

var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

const maxFiddleSize = 100 * 1024 // 100KB ought to be enough for anyone.

var pathkitPage []byte
var canvaskitPage []byte

var knownTypes = []string{"pathkit", "canvaskit"}

var fiddleStore *store.Store

func htmlHandler(page []byte) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if *local {
			// reload during local development
			loadPages()
		}
		w.Header().Set("Content-Type", "text/html")
		// Set the HTML to expire at the same time as the JS and WASM, otherwise the HTML
		// (and by extension, the JS with its cachbuster hash) might outlive the WASM
		// and then the two will skew
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(page); err != nil {
			httputils.ReportError(w, err, "Server could not load page", http.StatusInternalServerError)
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		// Use a shorter cache live to limit the risk of canvaskit.js (in indexbundle.js)
		// from drifting away from the version of canvaskit.wasm. Ideally, canvaskit
		// will roll at ToT (~35 commits per day), so living for a minute should
		// reduce the risk of JS/WASM being out of sync.
		w.Header().Add("Cache-Control", "max-age=60")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		p := r.URL.Path
		r.URL.Path = strings.TrimPrefix(p, "/res")
		fileServer.ServeHTTP(w, r)
	}
}

type fiddleContext struct {
	Code string `json:"code"`
	Type string `json:"type,omitempty"`
}

type saveResponse struct {
	NewURL string `json:"new_url"`
}

func codeHandler(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()
	fiddleType := ""
	if xt, ok := qp["type"]; ok {
		fiddleType = xt[0]
	}
	if !util.In(fiddleType, knownTypes) {
		sklog.Warningf("Unknown type requested %s", qp["type"])
		http.Error(w, "Invalid Type", http.StatusBadRequest)
		return
	}

	hash := ""
	if xh, ok := qp["hash"]; ok {
		hash = xh[0]
	}
	if hash == "" {
		// use demo code
		hash = "d962f6408d45d22c5e0dfe0a0b5cf2bad9dfaa49c4abc0e2b1dfb30726ab838d"
		if fiddleType == "canvaskit" {
			hash = "9c4df220dd05d8109ceb3a5cdb59653fd20d001affc2f4c1dabdd7b60c57d206"
		}
	}

	code, err := fiddleStore.GetCode(hash, fiddleType)
	if err != nil {
		http.Error(w, "Not found", http.StatusBadRequest)
		return
	}
	cr := fiddleContext{Code: code}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cr); err != nil {
		httputils.ReportError(w, err, "Failed to JSON Encode response.", http.StatusInternalServerError)
	}
}

func loadPages() {
	if p, err := ioutil.ReadFile(filepath.Join(*resourcesDir, "pathkit-index.html")); err != nil {
		sklog.Fatalf("Could not find pathkit html: %s", err)
	} else {
		pathkitPage = p
	}

	if p, err := ioutil.ReadFile(filepath.Join(*resourcesDir, "canvaskit-index.html")); err != nil {
		sklog.Fatalf("Could not find canvaskit html: %s", err)
	} else {
		canvaskitPage = p
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	req := fiddleContext{}
	dec := json.NewDecoder(r.Body)
	defer util.Close(r.Body)
	if err := dec.Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}
	if !util.In(req.Type, knownTypes) {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
	if len(req.Code) > maxFiddleSize {
		http.Error(w, fmt.Sprintf("Fiddle Too Big, max size is %d bytes", maxFiddleSize), http.StatusBadRequest)
		return
	}

	hash, err := fiddleStore.PutCode(req.Code, req.Type)
	if err != nil {
		httputils.ReportError(w, err, "Failed to save fiddle.", http.StatusInternalServerError)
	}
	sr := saveResponse{NewURL: fmt.Sprintf("/%s/%s", req.Type, hash)}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sr); err != nil {
		httputils.ReportError(w, err, "Failed to JSON Encode response.", http.StatusInternalServerError)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(kjlubick) have a nicer landing page, maybe one that shows canvaskit and pathkit.
	http.Redirect(w, r, "/canvaskit", http.StatusFound)
}

// cspHandler is an HTTP handler function which adds CSP (Content-Security-Policy)
// headers to this request
func cspHandler(h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// recommended by https://content-security-policy.com/
		// "This policy allows images, scripts, AJAX, and CSS from the same origin, and does
		// not allow any other resources to load (eg object, frame, media, etc).
		// It is a good starting point for many sites."
		w.Header().Add("Access-Control-Allow-Origin", "default-src 'none'; script-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self';")
		h(w, r)
	}
}

func main() {
	common.InitWithMust(
		"jsfiddle",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	loadPages()
	var err error
	fiddleStore, err = store.New(*local)
	if err != nil {
		sklog.Fatalf("Failed to connect to store: %s", err)
	}

	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		sklog.Fatal(err)
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler()).Methods("GET")
	r.HandleFunc("/canvaskit", cspHandler(htmlHandler(canvaskitPage))).Methods("GET")
	r.HandleFunc("/canvaskit/{id:[@0-9a-zA-Z_]+}", cspHandler(htmlHandler(canvaskitPage))).Methods("GET")
	r.HandleFunc("/pathkit", cspHandler(htmlHandler(pathkitPage))).Methods("GET")
	r.HandleFunc("/pathkit/{id:[@0-9a-zA-Z_]+}", cspHandler(htmlHandler(pathkitPage))).Methods("GET")
	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/_/save", saveHandler).Methods("PUT")
	r.HandleFunc("/_/code", codeHandler).Methods("GET")

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
