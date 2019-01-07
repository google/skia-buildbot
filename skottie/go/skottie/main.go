package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// BUCKET is the Cloud Storage bucket we store files in.
	BUCKET          = "skottie-renderer"
	BUCKET_INTERNAL = "skottie-renderer-internal"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	lockedDown   = flag.Bool("locked_down", false, "Restricted to only @google.com accounts.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	skottieTool  = flag.String("skottie_tool", "", "[deprecated/unused]Absolute path to the skottie_tool executable.")
	versionFile  = flag.String("version_file", "[deprecated/unused]/etc/skia-prod/VERSION", "The full path of the Skia VERSION file.")
)

var (
	invalidRequestErr = errors.New("")
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
		login.InitWithAllow(*port, *local, nil, nil, allow)
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
		filepath.Join(*resourcesDir, "embed.html"),
	))
}

func (srv *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *Server) embedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "embed.html", nil); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *Server) jsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	hash := mux.Vars(r)["hash"]
	path := strings.Join([]string{hash, "lottie.json"}, "/")
	reader, err := srv.bucket.Object(path).NewReader(r.Context())
	if err != nil {
		httputils.ReportError(w, r, err, "Can't load file from GCS")
		return
	}
	if _, err = io.Copy(w, reader); err != nil {
		httputils.ReportError(w, r, err, "Failed to write JSON file.")
		return
	}
}

type UploadRequest struct {
	Lottie   interface{} `json:"lottie"`
	Width    int         `json:"width"`
	Height   int         `json:"height"`
	FPS      float32     `json:"fps"`
	Filename string      `json:"filename"`
}

type UploadResponse struct {
	Hash string `json:"hash"`
}

func (req *UploadRequest) validate(w http.ResponseWriter) error {
	if req.FPS < 1 || req.FPS > 120 {
		http.Error(w, "FPS must be between 1 and 120.", http.StatusBadRequest)
		return invalidRequestErr
	}
	if req.Width < 1 || req.Width > 2048 {
		http.Error(w, "Width must be between 1 and 2048.", http.StatusBadRequest)
		return invalidRequestErr
	}
	if req.Height < 1 || req.Height > 2048 {
		http.Error(w, "Height must be between 1 and 2048.", http.StatusBadRequest)
		return invalidRequestErr
	}
	return nil
}

func (srv *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract json file.
	defer util.Close(r.Body)
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Error decoding JSON.")
		return
	}
	if err := req.validate(w); err != nil {
		return
	}

	b, err := json.Marshal(req.Lottie)
	if err != nil {
		httputils.ReportError(w, r, err, "Can't re-encode lottie file.")
		return
	}

	// Calculate md5 of file.
	// TODO(jcgregorio) include options in md5 calculation once they're added to the UI.
	h := md5.New()
	b, err = json.Marshal(req)
	if err != nil {
		httputils.ReportError(w, r, err, "Can't re-encode request.")
		return
	}
	if _, err = h.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed calculating hash.")
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	// Write JSON file.
	path := strings.Join([]string{hash, "lottie.json"}, "/")
	obj := srv.bucket.Object(path)
	wr := obj.NewWriter(ctx)
	wr.ObjectAttrs.ContentEncoding = "application/json"
	if _, err := wr.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed writing JSON to GCS.")
		return
	}
	if err := wr.Close(); err != nil {
		httputils.ReportError(w, r, err, "Failed writing JSON to GCS on close.")
		return
	}
	if !*lockedDown {
		if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			sklog.Errorf("Failed to make JSON public: %s", err)
		}
	}

	resp := UploadResponse{
		Hash: hash,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func main() {
	common.InitWithMust(
		"skottie",
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
	r.HandleFunc("/{hash:[0-9A-Za-z]*}", srv.mainHandler)
	r.HandleFunc("/e/{hash:[0-9A-Za-z]*}", srv.embedHandler)

	r.HandleFunc("/_/j/{hash:[0-9A-Za-z]+}", srv.jsonHandler)
	r.HandleFunc("/_/upload", srv.uploadHandler)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))

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
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
