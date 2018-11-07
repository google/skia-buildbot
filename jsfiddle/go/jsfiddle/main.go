package main

// The webserver for jsfiddle.skia.org. It serves up the web page

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

const MAX_FIDDLE_SIZE = 10 * 1024 * 1024 // 10KB ought to be enough for anyone.

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
		// This page should not be iframed. Maybe one day, something will be iframed,
		// but likely not this page.
		w.Header().Add("X-Frame-Options", "deny")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(page); err != nil {
			httputils.ReportError(w, r, err, "Server could not load page")
		}
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
			hash = "f06c4c41b975385830ae74aaff5caf79272d2ff096c98de28bd2cacb149a9f9d"
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
		httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		p := r.URL.Path
		r.URL.Path = strings.TrimPrefix(p, "/res")
		if strings.HasSuffix(p, "wasm") {
			// WASM won't do a streaming-compile if the mime-type isn't set
			w.Header().Set("Content-Type", "application/wasm")
		}
		fileServer.ServeHTTP(w, r)
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
		httputils.ReportError(w, r, err, "Failed to decode request.")
		return
	}
	if !util.In(req.Type, knownTypes) {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
	if len(req.Code) > MAX_FIDDLE_SIZE {
		http.Error(w, fmt.Sprintf("Fiddle Too Big, max size is %d bytes", MAX_FIDDLE_SIZE), http.StatusBadRequest)
		return
	}

	hash, err := fiddleStore.PutCode(req.Code, req.Type)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to save fiddle.")
	}
	sr := saveResponse{NewURL: fmt.Sprintf("/%s/%s", req.Type, hash)}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sr); err != nil {
		httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(kjlubick) have a nicer landing page, maybe one that shows canvaskit and pathkit.
	http.Redirect(w, r, "/pathkit", http.StatusFound)
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
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
