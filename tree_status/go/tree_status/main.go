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
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	// TODO(rmistry): Maybe this should be everyone?
	accessGroup        = flag.String("access_group", "googlers", "The chrome infra auth group to use for users incidents can be assigned to.")
	modifyGroup        = flag.String("modify_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	namespace          = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'tree-status-staging'.")
	project            = flag.String("project", "skia-tree-status-staging", "The Google Cloud project name.")
)

const (
	OPEN_STATE    = "open"
	CAUTION_STATE = "caution"
	CLOSED_STATE  = "closed"
)

var (
	// DS is the Cloud Datastore client to access tree statuses and rotations.
	DS *datastore.Client
)

// Server is the state of the server.
type Server struct {
	templates *template.Template
	access    allowed.Allow // Who is allowed to use the site.
	modify    allowed.Allow // Who is allowed to modify data on the site.
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	var access allowed.Allow
	var modify allowed.Allow
	if !*baseapp.Local {
		ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, err
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		access, err = allowed.NewAllowedFromChromeInfraAuth(client, *accessGroup)
		if err != nil {
			return nil, err
		}
		modify, err = allowed.NewAllowedFromChromeInfraAuth(client, *modifyGroup)
		if err != nil {
			return nil, err
		}
	} else {
		access = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
		modify = allowed.NewAllowedFromList([]string{"barney@example.org"})
		//modify = allowed.NewAllowedFromList([]string{"betty@example.org", "fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, modify, access)

	srv := &Server{
		access: access,
		modify: modify,
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
		filepath.Join(blah, "rotations.html"),
	))
}

func (srv *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
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
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// For tree status.
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/_/recent_statuses", srv.recentStatusesHandler).Methods("POST")
	r.HandleFunc("/_/add_tree_status", login.RestrictViewerFn(srv.addStatusHandler)).Methods("POST")
	r.HandleFunc("/_/get_autorollers", srv.getAutorollersHandler).Methods("POST")

	// ADD HANDLERS TO THE SPECIFIC FILES!!!!!!!!! I think this is better in the INITs...
	// For rotations.
	r.HandleFunc("/sheriff", srv.sheriffHandler)
	r.HandleFunc("/robocop", srv.robocopHandler)
	r.HandleFunc("/wrangler", srv.wranglerHandler)
	r.HandleFunc("/trooper", srv.trooperHandler)

	// REMOVE "get"...
	r.HandleFunc("/current-sheriff", httputils.CorsHandler(srv.getCurrentSheriffHandler))
	r.HandleFunc("/current-trooper", httputils.CorsHandler(srv.getCurrentTrooperHandler))
	r.HandleFunc("/current-robocop", httputils.CorsHandler(srv.getCurrentRobocopHandler))
	r.HandleFunc("/current-wrangler", httputils.CorsHandler(srv.getCurrentWranglerHandler))

	r.HandleFunc("/next-sheriff", httputils.CorsHandler(srv.getNextSheriffHandler))
	r.HandleFunc("/next-trooper", httputils.CorsHandler(srv.getNextTrooperHandler))
	r.HandleFunc("/next-robocop", httputils.CorsHandler(srv.getNextRobocopHandler))
	r.HandleFunc("/next-wrangler", httputils.CorsHandler(srv.getNextWranglerHandler))

	r.HandleFunc("/update_sheriff_rotations", login.RestrictEditorFn(srv.updateSheriffRotationsHandler))
	r.HandleFunc("/update_wrangler_rotations", login.RestrictEditorFn(srv.updateWranglerRotationsHandler))
	r.HandleFunc("/update_robocop_rotations", login.RestrictEditorFn(srv.updateRobocopRotationsHandler))
	r.HandleFunc("/update_trooper_rotations", login.RestrictEditorFn(srv.updateTrooperRotationsHandler))

	r.HandleFunc("/_/get_rotations", srv.getAutorollersHandler).Methods("POST")
	// Will obviously need more stuff here.
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func main() {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		sklog.Fatal(fmt.Errorf("Problem setting up default token source: %s", err))
	}

	if err := AutorollersInit(ctx, ts); err != nil {
		sklog.Fatal(skerr.Wrapf(err, "Could not init autorollers"))
	}

	DS, err = datastore.NewClient(context.Background(), *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(skerr.Wrapf(err, "Failed to initialize Cloud Datastore for tree status"))
	}

	// Load the last status and whether autorollers need to be watched.
	s, err := GetLatestStatus()
	if err != nil {
		sklog.Fatal(skerr.Wrapf(err, "Could not find latest status"))
	}
	if s.Rollers != "" {
		sklog.Infof("Last status has rollers that need to be watched: %s", s.Rollers)
		StartWatchingAutorollers(s.Rollers)
	}

	baseapp.Serve(New, []string{"tree.skia.org"})
}
