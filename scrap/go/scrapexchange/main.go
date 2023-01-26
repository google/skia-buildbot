// scrapexchange is the web admin and API server for the scrap exchange service.
//
// See http://go/scrap-exchange for more details.
package main

import (
	"context"
	"flag"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/api"
	"go.skia.org/infra/scrap/go/scrap"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

type flags struct {
	internalPort string
	bucket       string
	local        bool
	port         string
	promPort     string
	resourcesDir string
}

func (f *flags) Register(fs *flag.FlagSet) {
	fs.StringVar(&f.internalPort, "internal_port", ":9000", "HTTP internal service address (e.g., ':9000') for unauthenticated in-cluster requests.")
	fs.StringVar(&f.bucket, "bucket", "", "The Google Cloud Storage bucket that scraps are stored in.")
	fs.BoolVar(&f.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&f.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&f.promPort, "prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.StringVar(&f.resourcesDir, "resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
}

// server is the state of the server.
type server struct {
	flags        *flags
	apiEndpoints *api.Api
	templates    *template.Template
	login        alogin.Login
}

func new() (*server, error) {
	// Register and parse flags.
	flags := &flags{}
	flagSet := flag.NewFlagSet("scrapexchange", flag.ExitOnError)
	flags.Register(flagSet)

	common.InitWithMust(
		"scrapexchange",
		common.PrometheusOpt(&flags.promPort),
		common.MetricsLoggingOpt(),
		common.FlagSetOpt(flagSet),
	)

	if flags.bucket == "" {
		return nil, skerr.Fmt("--bucket is a required flag.")
	}

	// Fix up flag values.
	if flags.resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(1)
		flags.resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeFullControl, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gcsClient := gcsclient.New(storageClient, flags.bucket)
	scrapExchange, err := scrap.New(gcsClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create ScrapExchange.")
	}

	apiEndpoints := api.New(scrapExchange)

	srv := &server{
		flags:        flags,
		apiEndpoints: apiEndpoints,
		login:        proxylogin.NewWithDefaults(),
	}
	srv.loadTemplates()
	srv.startInternalServer()

	return srv, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(srv.flags.resourcesDir, "index.html"),
	))
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if srv.flags.local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// gzip applies gzip to HTTP responses.
func gzip(h http.Handler) http.Handler {
	return httputils.GzipRequestResponse(h)
}

func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler).Methods("GET")
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", gzip(http.HandlerFunc(httputils.MakeResourceHandler(srv.flags.resourcesDir))))).Methods("GET")
	srv.apiEndpoints.AddHandlers(r, api.DoNotAddProtectedEndpoints)
}

func (srv *server) startInternalServer() {
	// Internal endpoints that are only accessible from within the cluster.
	internal := mux.NewRouter()
	srv.apiEndpoints.AddHandlers(internal, api.AddProtectedEndpoints)
	go func() {
		sklog.Fatal(http.ListenAndServe(srv.flags.internalPort, internal))
	}()
}

func main() {
	s, err := new()
	if err != nil {
		sklog.Fatal(err)
	}

	// Add HTTP handlers.
	r := mux.NewRouter()
	s.AddHandlers(r)

	// Do not wrap http.Handler with security or authentication middleware if we
	// are running locally.
	var h http.Handler = r
	if !s.flags.local {
		h = baseapp.SecurityMiddleware([]string{"scrap.skia.org"}, s.flags.local, nil)(h)
		h = proxylogin.ForceRoleMiddleware(s.login, roles.Viewer)(h)
	}

	sklog.Infof("Ready to serve at: %q", s.flags.port)
	server := &http.Server{
		Addr:           s.flags.port,
		Handler:        h,
		MaxHeaderBytes: 1 << 20,
	}
	sklog.Fatal(server.ListenAndServe())
}
