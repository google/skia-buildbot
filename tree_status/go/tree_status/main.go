package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	//"path/filepath"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	//"go.skia.org/infra/go/ds"
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
	modifyGroup        = flag.String("modify_group", "project-skia-admins", "The chrome infra auth group to use for restricting access.")
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
		modify = allowed.NewAllowedFromList([]string{"betty@example.org", "fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, nil)
	// 	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, access)

	//ctx := context.Background()
	//ts, err := auth.NewDefaultTokenSource(*baseapp.Local, "https://www.googleapis.com/auth/datastore")
	//if err != nil {
	//	return nil, err
	//}

	// if *namespace == "" {
	//	return nil, fmt.Errorf("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	// }
	//if err := ds.InitWithOpt(*project, *namespace, option.WithTokenSource(ts)); err != nil {
	//	return nil, fmt.Errorf("Failed to init Cloud Datastore: %s", err)
	//}

	srv := &Server{
		//treeStore: tree.NewStore,
		// Also add sheriff and the other stuff in here...
		access: access,
		modify: modify,
	}
	srv.loadTemplates()
	liveness := metrics2.NewLiveness("alive", map[string]string{})
	fmt.Println(liveness)

	// NOTE: there is also something called go/audit log.
	// NOTE: Look at am.skia.org in general.

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
	}
}

// TODO(rmistry): Move this stuff over to rotations...
func (srv *Server) sheriffHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("SheriffSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get sheriff rotations.", http.StatusInternalServerError)
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Sheriff",
		RotationsDoc:  "https://skia.org/dev/sheriffing",
		RotationsImg:  "sheriff.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}
func (srv *Server) wranglerHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("GpuSheriffSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get wrangler rotations.", http.StatusInternalServerError)
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Wrangler",
		RotationsDoc:  "https://skia.org/dev/sheriffing/gpu",
		RotationsImg:  "wrangler.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}
func (srv *Server) robocopHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("RobocopSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get robocop rotations.", http.StatusInternalServerError)
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Android Robocop",
		RotationsDoc:  "https://skia.org/dev/sheriffing/android",
		RotationsImg:  "robocop.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}
func (srv *Server) trooperHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("TrooperSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get trooper rotations.", http.StatusInternalServerError)
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Infra Trooper",
		RotationsDoc:  "https://skia.org/dev/sheriffing/trooper",
		RotationsImg:  "trooper.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}

func (srv *Server) commonHandler(w http.ResponseWriter, r *http.Request, templateContext RotationsTemplateContext) error {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "rotations.html", templateContext); err != nil {
		return fmt.Errorf("Failed to expand template: %s", err)
	}
	return nil
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
	r.HandleFunc("/_/add_tree_status", srv.addStatusHandler).Methods("POST")
	r.HandleFunc("/_/get_autorollers", srv.getAutorollersHandler).Methods("POST")

	// For rotations.
	r.HandleFunc("/sheriff", srv.sheriffHandler)
	r.HandleFunc("/robocop", srv.robocopHandler)
	r.HandleFunc("/wrangler", srv.wranglerHandler)
	r.HandleFunc("/trooper", srv.trooperHandler)
	r.HandleFunc("/_/get_rotations", srv.getAutorollersHandler).Methods("POST")
	// Will obviously need more stuff here.
}

func (srv *Server) recentStatusesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statuses, err := GetStatuses(25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) getAutorollersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	as, err := GetAutorollStatuses(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get autoroll statuses.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(as); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) addStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	user := srv.user(r)
	if !srv.access.Member(user) {
		httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
		return
	}

	// Get the message from the request.
	m := struct {
		Message string `json:"message"`
		Rollers string `json:"rollers"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}
	message := m.Message
	rollers := m.Rollers
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXx")
	fmt.Printf("%+v", m)

	// Validate the message. Extract into a function.
	containsOpenState := strings.Contains(strings.ToLower(message), OPEN_STATE)
	containsCautionState := strings.Contains(strings.ToLower(message), CAUTION_STATE)
	containsClosedState := strings.Contains(strings.ToLower(message), CLOSED_STATE)
	fmt.Println(containsOpenState)
	fmt.Println(containsCautionState)
	fmt.Println(containsClosedState)
	if (containsOpenState && containsCautionState) ||
		(containsCautionState && containsClosedState) ||
		(containsClosedState && containsOpenState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Cannot specify two keywords from (%s, %s, %s) in a status message.", OPEN_STATE, CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	} else if !(containsOpenState || containsCautionState || containsClosedState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Must specify either (%s, %s, %s) somewhere in the status message.", OPEN_STATE, CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	} else if containsOpenState && rollers != "" {
		httputils.ReportError(w, nil, fmt.Sprintf("Waiting for rollers should only be used with %s or %s states", CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	}

	// ADD A MUTEX HERE!!!!!!!!!!!!!

	StopWatchingAutorollers()

	if err := AddStatus(message, user, rollers); err != nil {
		httputils.ReportError(w, err, "Failed to add message to the datastore", http.StatusInternalServerError)
		return
	}

	// Start the autorollers goroutine.
	StartWatchingAutorollers(rollers)

	// Return updated list of the most recent tree statuses.
	statuses, err := GetStatuses(25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
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

	//if err := StatusInit(ts, *project, *namespace, *baseapp.Local); err != nil {
	//	sklog.Fatal(skerr.Wrapf(err, "Could not init datastore"))
	//}

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

	//fmt.Println("TESITNG")
	//_, err = GetUpcomingRotations("TrooperSchedules")
	//if err != nil {
	//	sklog.Fatal(err)
	//}
	//for _, r := range rotations {
	//	fmt.Println(r.Username)
	//	fmt.Println(r.ScheduleStart)
	//	fmt.Println(r.ScheduleEnd)
	//	fmt.Println("---------------")
	//}

	baseapp.Serve(New, []string{"tree.skia.org"})
}
