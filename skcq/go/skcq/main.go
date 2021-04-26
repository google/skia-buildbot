/*
	Skia Commit Queue server
*/

package main

import (
	"context"
	"flag"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	// "go.skia.org/infra/bugs-central/go/db"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/poller"
)

var (
	// Flags
	host        = flag.String("host", "skcq.skia.org", "HTTP service host")
	workdir     = flag.String("workdir", ".", "Directory to use for scratch work.")
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'skcq'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	serviceAccountFile = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")

	// Keep this really really fast.
	pollInterval = flag.Duration("poll_interval", 30*time.Second, "How often the server will poll Gerrit for CR+1 and CQ+1/CQ+2 changes.")
)

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	// Create workdir if it does not exist.
	if err := os.MkdirAll(*workdir, 0755); err != nil {
		sklog.Fatalf("Could not create %s: %s", *workdir, err)
	}

	// Note: Everything is nil over here. For private instance might need something else? or not.
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, nil)

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, datastore.ScopeDatastore)
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	// if err != nil {
	// 	sklog.Fatalf("Could not init DB: %s", err)
	// }

	// Instantiate poller and turn it on.
	// pollerClient, err := poller.New(ctx, ts, *serviceAccountFile, dbClient)
	// if err != nil {
	// 	sklog.Fatalf("Could not init poller: %s", err)
	// }
	// if err := pollerClient.Start(ctx, *pollInterval); err != nil {
	// 	sklog.Fatalf("Could not start poller: %s", err)
	// }

	// Instantiate codereview.
	g, err := codereview.NewGerrit(httpClient)
	if err != nil {
		sklog.Fatalf("Could not init gerrit client: %s", err)
	}

	// Instantiate poller and turn it on.
	if err := poller.Start(ctx, *pollInterval, g); err != nil {
		sklog.Fatalf("Could not init poller: %s", err)
	}

	srv := &Server{
		// pollerClient: pollerClient,
		// dbClient:     dbClient,
	}
	srv.loadTemplates()

	return srv, nil
}

// Server is the state of the server.
type Server struct {
	// pollerClient *poller.IssuesPoller
	// dbClient     *db.FirestoreDB
	templates *template.Template
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// All endpoints that require authentication should be added to this router.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/", srv.indexHandler)

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(New, []string{*host})
}
