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

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/api"
	"go.skia.org/infra/scrap/go/scrap"
	"google.golang.org/api/option"
)

// flags
var (
	authGroup    = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	internalPort = flag.String("internal_port", ":9000", "HTTP internal service address (e.g., ':9000') for unauthenticated in-cluster requests.")
	bucket       = flag.String("bucket", "", "The Google Cloud Storage bucket that scraps are stored in.")
)

// server is the state of the server.
type server struct {
	apiEndpoints *api.Api
	templates    *template.Template
}

// New implements baseapp.Constructor.
func New() (baseapp.App, error) {
	if *bucket == "" {
		return nil, skerr.Fmt("--bucket is a required flag.")
	}
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, storage.ScopeFullControl, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var admin allowed.Allow
	if !*baseapp.Local {
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		admin, err = allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	} else {
		admin = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, admin, nil, nil)

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	gcsClient := gcsclient.New(storageClient, *bucket)
	scrapExchange, err := scrap.New(gcsClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create ScrapExchange.")
	}

	apiEndpoints := api.New(scrapExchange)

	srv := &server{
		apiEndpoints: apiEndpoints,
	}
	srv.loadTemplates()

	srv.startInternalServer()

	return srv, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// AddHandlers implements baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	var h http.HandlerFunc = srv.mainHandler
	if !*baseapp.Local {
		h = login.RestrictAdminFn(h)
	}
	r.HandleFunc("/", h)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")
	srv.apiEndpoints.AddHandlers(r, api.DoNotAddProtectedEndpoints)
}

// AddMiddleware implements baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func (srv *server) startInternalServer() {
	// Internal endpoints that are only accessible from within the cluster.
	internal := mux.NewRouter()
	srv.apiEndpoints.AddHandlers(internal, api.AddProtectedEndpoints)
	go func() {
		sklog.Fatal(http.ListenAndServe(*internalPort, internal))
	}()
}

func main() {
	baseapp.Serve(New, []string{"scrap.skia.org"})
}
