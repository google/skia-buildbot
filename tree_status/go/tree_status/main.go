package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// defaultSkiaRepo is the repo to default to when no repos have been specified.
// It is the main "skia.git" repo.
const defaultSkiaRepo = "skia"

// Flags
var (
	host               = flag.String("host", "tree-status.skia.org", "HTTP service host")
	modifyGroup        = flag.String("modify_group", "project-skia-committers", "The chrome infra auth group to use for who is allowed to change tree status.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "Path to a local file, or name of a GCP secret, containing the JWT key for the service account that has access to chrome infra auth.")
	namespace          = flag.String("namespace", "tree-status-staging", "The Cloud Datastore namespace.")
	dsProject          = flag.String("ds-project", "skia-public", "Name of the GCP project used for Datastore.")
	repos              = common.NewMultiStringFlag("repo", nil, "These repos will have tree status endpoints.")
	secretProject      = flag.String("secret-project", "skia-infra-public", "Name of the GCP project used for secret management.")
	internalPort       = flag.String("internal_port", "", "HTTP internal service address (eg: ':8001' for unauthenticated in-cluster requests.")
)

var (
	// dsClient is the Cloud Datastore client to access tree statuses.
	dsClient *datastore.Client

	// repoNameRegex matches the format of supported repo names.
	repoNameRegex = "{repo:[0-9a-zA-Z._-]+}"
)

// Server is the state of the server.
type Server struct {
	templates  *template.Template
	modify     allowed.Allow // Who is allowed to modify tree status.
	autorollDB status.DB

	// skiaRepoSpecified is set to true when the main skia has been specified.
	// This boolean is used because the main skia repo requires support for
	// non-repo specified URLs (for backwards compatibility) and for watching
	// autorollers.
	skiaRepoSpecified bool
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}

	dsClient, err = datastore.NewClient(context.Background(), *dsProject, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to initialize Cloud Datastore for tree status")
	}

	// Check to see if the main skia repo has been specified. If it has been
	// specified then it will require special handling.
	skiaRepoSpecified := IsRepoSupported(defaultSkiaRepo)

	var autorollDB status.DB
	if skiaRepoSpecified {
		// Start watching for statuses with autorollers specified. Only supported for
		// the default repo (skia).
		autorollDB, err = AutorollersInit(ctx, defaultSkiaRepo, ts)
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not init autorollers")
		}

		// Load the last status and whether autorollers need to be watched.
		s, err := GetLatestStatus(defaultSkiaRepo)
		if err != nil {
			return nil, skerr.Wrapf(err, "Could not find latest status")
		}
		if s.Rollers != "" {
			sklog.Infof("Last status has rollers that need to be watched: %s", s.Rollers)
			StartWatchingAutorollers(s.Rollers)
		}
	}

	var modify allowed.Allow
	if !*baseapp.Local {
		ts, err := auth.NewJWTServiceAccountTokenSource(ctx, "", *chromeInfraAuthJWT, *secretProject, *chromeInfraAuthJWT, auth.ScopeUserinfoEmail)
		if err != nil {
			return nil, err
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		modify, err = allowed.NewAllowedFromChromeInfraAuth(client, *modifyGroup)
		if err != nil {
			return nil, err
		}
	} else {
		modify = allowed.NewAllowedFromList([]string{"barney@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil /* Admins not needed */, modify, nil /* Everyone is allowed to access */)

	srv := &Server{
		modify:            modify,
		autorollDB:        autorollDB,
		skiaRepoSpecified: skiaRepoSpecified,
	}
	srv.loadTemplates()
	liveness := metrics2.NewLiveness("alive", map[string]string{})
	fmt.Println(liveness)

	return srv, nil
}

func (srv *Server) loadTemplates() {
	blah := *baseapp.ResourcesDir
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(blah, "index.html"),
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

	// All endpoints that require authentication should be added to this router. The
	// rest of endpoints are left unauthenticated because they are accessed from various
	// places like: Skia infra apps, Gerrit plugin, Chrome extensions, presubmits, etc.
	appRouter := mux.NewRouter()

	if srv.skiaRepoSpecified {
		// If the main skia repo has been specified then leave default repo
		// handlers around for backwards compatibility.
		appRouter.HandleFunc("/", srv.treeStateDefaultRepoHandler).Methods("GET")
		r.HandleFunc("/current", httputils.CorsHandler(srv.bannerStatusHandler)).Methods("GET")
	}
	appRouter.HandleFunc("/_/get_autorollers", srv.autorollersHandler).Methods("POST")

	// Add repo-specific endpoints.
	appRouter.HandleFunc(fmt.Sprintf("/%s", repoNameRegex), srv.treeStateDefaultRepoHandler).Methods("GET")
	appRouter.HandleFunc(fmt.Sprintf("/%s/_/add_tree_status", repoNameRegex), srv.addStatusHandler).Methods("POST")
	appRouter.HandleFunc(fmt.Sprintf("/%s/_/recent_statuses", repoNameRegex), srv.recentStatusesHandler).Methods("POST")
	r.HandleFunc(fmt.Sprintf("/%s/current", repoNameRegex), httputils.CorsHandler(srv.bannerStatusHandler)).Methods("GET")

	if *internalPort != "" {
		internalRouter := mux.NewRouter()
		internalRouter.HandleFunc(fmt.Sprintf("/%s/current", repoNameRegex), httputils.CorsHandler(srv.bannerStatusHandler)).Methods("GET")
		internalRouter.HandleFunc("/current", srv.bannerStatusHandler).Methods("GET")

		go func() {
			sklog.Infof("Internal server on %q", *internalPort)
			sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		}()
	}

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_OAUTH2_CALLBACK)
	}

	r.PathPrefix("/").Handler(appHandler)
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

// IsRepoSupported is a utility function that returns true if the specified
// repo is a supported repo (i.e. has been specified in the repos flag).
func IsRepoSupported(repo string) bool {
	return util.In(repo, *repos)
}

func main() {
	// Parse flags to be able to send *host to baseapp.Serve
	flag.Parse()
	baseapp.Serve(New, []string{*host})
}
