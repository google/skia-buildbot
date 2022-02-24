package main

import (
	"flag"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	// The loaded index page of the lit-html version.
	indexPage []byte
)

func main() {
	// flags
	var (
		port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
		promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		resourcesDir = flag.String("resources_dir", "/usr/local/share/debugger-app/", "The directory to find lit-html stuff.")
	)

	common.InitWithMust(
		"debugger-app",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	loadPage(*resourcesDir)

	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		sklog.Fatal(err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/dist/").HandlerFunc(makeResourceHandler(*resourcesDir))
	router.HandleFunc("/", mainHandler)

	http.Handle("/", httputils.HealthzAndHTTPS(httputils.LoggingRequestResponse(router)))
	sklog.Info("Application served at http://localhost:8000/dist/main.html")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func loadPage(resourceDir string) {
	p, err := ioutil.ReadFile(filepath.Join(resourceDir, "main.html"))
	if err != nil {
		sklog.Fatalf("Could not find index html: %s", err)
	}
	indexPage = p
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	// Set the HTML to expire at the same time as the JS and WASM, otherwise the HTML
	// (and by extension, the JS with its cachbuster hash) might outlive the WASM
	// and then the two will skew
	w.Header().Set("Cache-Control", "max-age=60")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(indexPage); err != nil {
		httputils.ReportError(w, err, "Server could not load page", http.StatusInternalServerError)
	}
}

func makeResourceHandler(dir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		// This is structured this way so we can control Cache-Control settings to avoid
		// WASM and JS from skewing out of date.
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		p := r.URL.Path
		r.URL.Path = strings.TrimPrefix(p, "/dist")
		fileServer.ServeHTTP(w, r)
	}
}
