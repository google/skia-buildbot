package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"text/template"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/codesize/go/bloaty"
	"go.skia.org/infra/codesize/go/codesizeserver/rpc"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const gcsBucket = "skia-codesize"

type server struct {
	templates *template.Template
	gcsClient *gcsclient.StorageClient

	// bloatyFile holds the contents of a single Bloaty file loaded from GCS, and will soon be
	// replaced with a more appropriate data structure to support multiple artifacts and metadata
	// (from JSON files with build parameters, Bloaty command-line arguments, etc.).
	//
	// TODO(lovisolo): Replace with a more definitive in-memory cache with the above information.
	bloatyFile string
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	ctx := context.Background()
	srv := &server{}

	// Set up GCS client.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, storage.ScopeFullControl)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get token source")
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create storage client")
	}
	srv.gcsClient = gcsclient.New(storageClient, gcsBucket)

	// Preload the latest Bloaty outputs.
	if err := srv.preloadBloatyFiles(); err != nil {
		return nil, skerr.Wrapf(err, "failed to preload Bloaty outputs from GCS")
	}

	// TODO(lovisolo): Subscribe to GCS pubsub events to detect new file uploads.

	srv.loadTemplates()

	return srv, nil
}

// preloadBloatyFiles preloads the latest Bloaty outputs for each supported build artifact.
func (s *server) preloadBloatyFiles() error {
	// For now, this reads a single known Bloaty output file. Soon, we will replace this with a
	// directory structure of the form /<artifact name>/YYYY/MM/DD/<git hash>.{tsv/json}, where
	// <artifact name> is the name of the binary plus information about how it was built (e.g.
	// "dm-debug", "dm-release", etc.), the TSV file is the corresponding Bloaty output, and the JSON
	// file is a file with metadata such as the exact build parameters, Bloaty version and
	// command-line arguments, etc.
	//
	// TODO(lovisolo): Implement and test.

	contents, err := s.gcsClient.GetFileContents(context.Background(), "dm.tsv")
	if err != nil {
		return skerr.Wrap(err)
	}

	s.bloatyFile = string(contents[:])
	return nil
}

func (s *server) loadTemplates() {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(
		filepath.Join(*baseapp.ResourcesDir, "*.html"),
	))
}

// sendJSONResponse sends a JSON representation of any data structure as an HTTP response. If the
// conversion to JSON has an error, the error is logged.
func sendJSONResponse(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// sendHTMLResponse renders the given template, passing it the current context's CSP nonce. If
// template rendering fails, it logs an error.
func (s *server) sendHTMLResponse(templateName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := s.templates.ExecuteTemplate(w, templateName, map[string]string{
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (s *server) machinesPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("index.html", w, r)
}

func (s *server) bloatyHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lovisolo): Parameterize this RPC and read the Bloaty output for the given artifact from
	//                 an in-memory cache.
	outputItems, err := bloaty.ParseTSVOutput(s.bloatyFile)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse dm.tsv.", http.StatusInternalServerError)
		return
	}

	res := rpc.BloatyRPCResponse{
		Rows: bloaty.GenTreeMapDataTableRows(outputItems),
	}
	sendJSONResponse(res, w)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.machinesPageHandler).Methods("GET")
	r.HandleFunc("/rpc/bloaty/v1", s.bloatyHandler).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"codesize.skia.org"})
}
