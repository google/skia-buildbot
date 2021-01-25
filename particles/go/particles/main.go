package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/scrap/go/client"
	"go.skia.org/infra/scrap/go/scrap"
)

// flags
var (
	local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port          = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir  = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	scrapExchange = flag.String("scrapexchange", "http://scrapexchange:9000", "Scrap exchange service HTTP address.")
)

// server is the state of the server.
type server struct {
	scrapClient scrap.ScrapExchange
	templates   *template.Template
}

func new() (*server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		return nil, err
	}
	scrapClient, err := client.New(*scrapExchange)
	if err != nil {
		sklog.Fatalf("Failed to create scrap exchange client: %s", err)
	}

	srv := &server{
		scrapClient: scrapClient,
	}
	srv.loadTemplates()
	return srv, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "main.html"),
	))
}

func (srv *server) templateHandler(filename string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// Set the HTML to expire at the same time as the JS and WASM, otherwise the HTML
		// (and by extension, the JS with its cachbuster hash) might outlive the WASM
		// and then the two will skew
		w.Header().Set("Cache-Control", "max-age=60")
		if *local {
			srv.loadTemplates()
		}
		if err := srv.templates.ExecuteTemplate(w, filename, nil); err != nil {
			sklog.Errorf("Failed to expand template %s: %s", filename, err)
		}
	}
}

func resourceHandler(resourcesDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		// Use a shorter cache live to limit the risk of canvaskit.js (in indexbundle.js)
		// from drifting away from the version of canvaskit.wasm. Ideally, the WASM
		// will roll at ToT (~35 commits per day), so living for a minute should
		// reduce the risk of JS/WASM being out of sync.
		w.Header().Add("Cache-Control", "max-age=60")
		fileServer.ServeHTTP(w, r)
	}
}

func (srv *server) jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	hashOrName := mux.Vars(r)["hashOrName"]

	body, err := srv.scrapClient.LoadScrap(r.Context(), scrap.Particle, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to read JSON file.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func (srv *server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)

	// Decode Request.
	var req scrap.ScrapBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Error decoding JSON.", http.StatusBadRequest)
		return
	}
	if req.Type != scrap.Particle {
		httputils.ReportError(w, fmt.Errorf("Received invalid scrap type: %q", req.Type), "Invalid Type.", http.StatusBadRequest)
		return
	}

	scrapID, err := srv.scrapClient.CreateScrap(r.Context(), req)
	if err != nil {
		httputils.ReportError(w, err, "Error creating scrap.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scrapID); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func main() {
	common.InitWithMust(
		"particles",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	srv, err := new()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.CorsHandler(resourceHandler(*resourcesDir))))).Methods("GET")
	r.HandleFunc("/_/j/{hashOrName:[@0-9a-zA-Z-_]+}", srv.jsonHandler).Methods("GET")
	r.HandleFunc("/_/upload", srv.uploadHandler).Methods("POST")
	r.HandleFunc("/", srv.templateHandler("main.html")).Methods("GET")

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
