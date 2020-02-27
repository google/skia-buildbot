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

// Flags
var (
	modifyGroup        = flag.String("modify_group", "googlers", "The chrome infra auth group to use for who is allowed to change tree status.")
	adminGroup         = flag.String("admin_group", "google/skia-staff@google.com", "The chrome infra auth group to use for who is allowed to update rotations.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	namespace          = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'tree-status-staging'.")
	project            = flag.String("project", "skia-tree-status-staging", "The Google Cloud project name.")
)

var (
	// dsClient is the Cloud Datastore client to access tree statuses and rotations.
	dsClient *datastore.Client
)

// Server is the state of the server.
type Server struct {
	templates *template.Template
	modify    allowed.Allow // Who is allowed to modify tree status.
	admin     allowed.Allow // Who is allowed to modify rotations.
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	var modify allowed.Allow
	var admin allowed.Allow
	if !*baseapp.Local {
		ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, err
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		modify, err = allowed.NewAllowedFromChromeInfraAuth(client, *modifyGroup)
		if err != nil {
			return nil, err
		}
		admin, err = allowed.NewAllowedFromChromeInfraAuth(client, *adminGroup)
		if err != nil {
			return nil, err
		}
	} else {
		modify = allowed.NewAllowedFromList([]string{"barney@example.org"})
		admin = allowed.NewAllowedFromList([]string{"barney@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, admin, modify, nil /* Everyone is allowed to access */)

	srv := &Server{
		modify: modify,
		admin:  admin,
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

	// For tree status.
	r.HandleFunc("/", srv.treeStateHandler)
	r.HandleFunc("/current", srv.bannerStatusHandler)
	r.HandleFunc("/_/add_tree_status", srv.addStatusHandler).Methods("POST")
	r.HandleFunc("/_/recent_statuses", srv.recentStatusesHandler).Methods("POST")

	// For rotations.
	r.HandleFunc("/sheriff", srv.sheriffHandler)
	r.HandleFunc("/robocop", srv.robocopHandler)
	r.HandleFunc("/wrangler", srv.wranglerHandler)
	r.HandleFunc("/trooper", srv.trooperHandler)

	r.HandleFunc("/current-sheriff", httputils.CorsHandler(srv.currentSheriffHandler))
	r.HandleFunc("/current-robocop", httputils.CorsHandler(srv.currentRobocopHandler))
	r.HandleFunc("/current-wrangler", httputils.CorsHandler(srv.currentWranglerHandler))
	r.HandleFunc("/current-trooper", httputils.CorsHandler(srv.currentTrooperHandler))

	r.HandleFunc("/next-sheriff", httputils.CorsHandler(srv.nextSheriffHandler))
	r.HandleFunc("/next-robocop", httputils.CorsHandler(srv.nextRobocopHandler))
	r.HandleFunc("/next-wrangler", httputils.CorsHandler(srv.nextWranglerHandler))
	r.HandleFunc("/next-trooper", httputils.CorsHandler(srv.nextTrooperHandler))

	r.HandleFunc("/update_sheriff_rotations", srv.updateSheriffRotationsHandler)
	r.HandleFunc("/update_robocop_rotations", srv.updateRobocopRotationsHandler)
	r.HandleFunc("/update_wrangler_rotations", srv.updateWranglerRotationsHandler)
	r.HandleFunc("/update_trooper_rotations", srv.updateTrooperRotationsHandler)
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
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		sklog.Fatal(fmt.Errorf("Problem setting up default token source: %s", err))
	}

	dsClient, err = datastore.NewClient(context.Background(), *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(skerr.Wrapf(err, "Failed to initialize Cloud Datastore for tree status"))
	}

	baseapp.Serve(New, []string{"tree.skia.org"})
}
