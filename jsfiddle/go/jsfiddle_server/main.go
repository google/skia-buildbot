package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var pathkitPage []byte

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func pathkitHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		// reload during local development
		loadPages()
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pathkitPage); err != nil {
		httputils.ReportError(w, r, err, "Server could not load page")
	}
}

type codeResponse struct {
	Code string `json:"code"`
}

func codeHandler(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()
	if util.In("pathkit", qp["type"]) {
		if util.In("demo", qp["hash"]) {
			cr := codeResponse{Code: pathkitDemoCode}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(cr); err != nil {
				httputils.ReportError(w, r, err, "Failed to JSON Encode response.")
			}
			return
		}
		// TODO(kjlubick): actually look up the code from GCS
	}
	http.Error(w, "Not found", http.StatusBadRequest)
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
		sklog.Errorf("Could not find pathkit html", err)
	} else {
		pathkitPage = p
	}
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(kjlubick)
	http.Error(w, "Not implemented", 500)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(kjlubick) have a nicer landing page, maybe one that shows canvaskit and pathkit.
	http.Redirect(w, r, "/pathkit", http.StatusFound)
}

func main() {
	common.InitWithMust(
		"jsfiddle",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	loadPages()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/pathkit", pathkitHandler)
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/_/save", saveHandler)
	r.HandleFunc("/_/code", codeHandler)

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

const pathkitDemoCode = `// canvas and PathKit are globally available
let firstPath = PathKit.FromSVGString('M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zM12 20c-4.42 0-8-3.58-8-8s3.58-8 8-8 8 3.58 8 8-3.58 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z');

let secondPath = PathKit.NewPath();
// Acts somewhat like the Canvas API, except can be chained
secondPath.moveTo(1, 1)
          .lineTo(20, 1)
          .lineTo(10, 30)
          .closePath();

// Join the two paths together (mutating firstPath in the process)
firstPath.op(secondPath, PathKit.PathOp.INTERSECT);

// Draw directly to Canvas
let ctx = canvas.getContext('2d');
ctx.strokeStyle = '#CC0000';
ctx.fillStyle = '#000000';
ctx.scale(20, 20);
ctx.beginPath();
firstPath.toCanvas(ctx);
ctx.fill();
ctx.stroke();


// clean up WASM memory
// See http://kripken.github.io/emscripten-site/docs/porting/connecting_cpp_and_javascript/embind.html?highlight=memory#memory-management
firstPath.delete();
secondPath.delete();`
