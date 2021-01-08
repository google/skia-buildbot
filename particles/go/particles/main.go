package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// BUCKET is the Cloud Storage bucket we store files in.
	BUCKET          = "skparticles-renderer"
	BUCKET_INTERNAL = "skparticles-renderer-internal"

	MAX_FILENAME_SIZE = 5 * 1024
	MAX_JSON_SIZE     = 10 * 1024 * 1024
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	lockedDown   = flag.Bool("locked_down", false, "Restricted to only @google.com accounts.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

// Server is the state of the server.
type Server struct {
	bucket    *storage.BucketHandle
	templates *template.Template
}

func New() (*Server, error) {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		return nil, err
	}

	ts, err := auth.NewDefaultTokenSource(*local, storage.ScopeFullControl)
	if err != nil {
		return nil, fmt.Errorf("Failed to get token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}

	if *lockedDown {
		allow := allowed.NewAllowedFromList([]string{"google.com"})
		login.SimpleInitWithAllow(*port, *local, nil, nil, allow)
	}

	bucket := BUCKET
	if *lockedDown {
		bucket = BUCKET_INTERNAL
	}

	srv := &Server{
		bucket: storageClient.Bucket(bucket),
	}
	srv.loadTemplates()
	return srv, nil
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
	))
}

func (srv *Server) templateHandler(filename string) func(http.ResponseWriter, *http.Request) {
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

func (srv *Server) jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	hash := mux.Vars(r)["hash"]
	path := strings.Join([]string{hash, "input.json"}, "/")
	reader, err := srv.bucket.Object(path).NewReader(r.Context())
	if err != nil {
		sklog.Warningf("Can't load JSON file %s from GCS: %s", path, err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err = io.Copy(w, reader); err != nil {
		httputils.ReportError(w, err, "Failed to write JSON file.", http.StatusInternalServerError)
		return
	}
}

type UploadRequest struct {
	ParticlesJSON interface{} `json:"json"` // the parsed JSON
	Filename      string      `json:"filename"`
}

type UploadResponse struct {
	Hash string `json:"hash"`
}

func (srv *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract json file.
	defer util.Close(r.Body)
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Error decoding JSON.", http.StatusInternalServerError)
		return
	}
	// Check for maliciously sized input on any field we upload to GCS
	if len(req.Filename) > MAX_FILENAME_SIZE {
		httputils.ReportError(w, nil, "Input file(s) too big", http.StatusInternalServerError)
		return
	}

	// Calculate md5 of UploadRequest (json contents and file name)
	h := md5.New()
	b, err := json.Marshal(req)
	if err != nil {
		httputils.ReportError(w, err, "Can't re-encode request.", http.StatusInternalServerError)
		return
	}
	if _, err = h.Write(b); err != nil {
		httputils.ReportError(w, err, "Failed calculating hash.", http.StatusInternalServerError)
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	if strings.HasSuffix(req.Filename, ".json") {
		if err := srv.createFromJSON(&req, hash, ctx); err != nil {
			httputils.ReportError(w, err, "Failed handing input of JSON.", http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		msg := "Only .json files allowed"
		if _, err := w.Write([]byte(msg)); err != nil {
			sklog.Errorf("Failed to write error response: %s", err)
		}
		return
	}

	resp := UploadResponse{
		Hash: hash,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func (srv *Server) createFromJSON(req *UploadRequest, hash string, ctx context.Context) error {
	b, err := json.Marshal(req.ParticlesJSON)
	if err != nil {
		return skerr.Fmt("Can't re-encode json file: %s", err)
	}
	if len(b) > MAX_JSON_SIZE {
		return skerr.Fmt("Particles JSON is too big (%d bytes): %s", len(b), err)
	}

	return srv.uploadState(req, hash, ctx)
}

func (srv *Server) uploadState(req *UploadRequest, hash string, ctx context.Context) error {
	// Write JSON file, containing the state (filename, json, etc)
	bytesToUpload, err := json.Marshal(req)
	if err != nil {
		return skerr.Fmt("Can't re-encode request: %s", err)
	}

	path := strings.Join([]string{hash, "input.json"}, "/")
	obj := srv.bucket.Object(path)
	wr := obj.NewWriter(ctx)
	wr.ObjectAttrs.ContentEncoding = "application/json"
	if _, err := wr.Write(bytesToUpload); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS: %s", err)
	}
	if err := wr.Close(); err != nil {
		return skerr.Fmt("Failed writing JSON to GCS on close: %s", err)
	}
	return nil
}

func main() {
	common.InitWithMust(
		"particles",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	if *lockedDown && *local {
		sklog.Fatalf("Can't be run as both --locked_down and --local.")
	}

	srv, err := New()
	if err != nil {
		sklog.Fatalf("Failed to start: %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/{hash:[0-9A-Za-z]*}", srv.templateHandler("index.html")).Methods("GET")
	r.HandleFunc("/_/j/{hash:[0-9A-Za-z]+}", srv.jsonHandler).Methods("GET")
	r.HandleFunc("/_/upload", srv.uploadHandler).Methods("POST")

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.CorsHandler(resourceHandler(*resourcesDir))))).Methods("GET")

	// TODO(jcgregorio) Implement CSRF.
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		if *lockedDown {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
		}
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
