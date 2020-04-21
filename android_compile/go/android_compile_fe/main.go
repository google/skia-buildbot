/*
	Android Compile Server Frontend.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/android_compile/go/util"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

const (
	forceSyncPostUrl        = "/_/force_sync"
	pendingTasksPostUrl     = "/_/pending_tasks"
	compileInstancesPostUrl = "/_/compile_instances"
)

var (
	// Flags
	host = flag.String("host", "skia-android-compile.corp.goog", "HTTP service host")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile-staging", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-corp", "The Google Cloud project name.")

	serverURL string
)

// Server is the state of the server.
type Server struct {
	templates *template.Template
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) compileInstancesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	compileInstances, err := util.GetAllCompileInstances(context.Background())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get android compile instances", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(compileInstances); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) pendingTasksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	unownedPendingTasks, ownedPendingTasks, err := util.GetPendingCompileTasks("" /* ownedByInstance */)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get unowned/owned compile tasks", http.StatusInternalServerError)
		return
	}

	var pendingTasks = struct {
		UnOwnedPendingTasks []*util.CompileTask `json:"unowned_pending_tasks"`
		OwnedPendingTasks   []*util.CompileTask `json:"owned_pending_tasks"`
	}{
		UnOwnedPendingTasks: unownedPendingTasks,
		OwnedPendingTasks:   ownedPendingTasks,
	}

	if err := json.NewEncoder(w).Encode(pendingTasks); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (srv *Server) forceSyncHandler(w http.ResponseWriter, r *http.Request) {
	if err := util.SetForceMirrorUpdateOnAllInstances(context.Background()); err != nil {
		httputils.ReportError(w, err, "Failed to set force mirror update on all instances", http.StatusInternalServerError)
		return
	}
	sklog.Infof("Force sync button has been pressed by %s", login.LoggedInAs(r))

	compileInstances, err := util.GetAllCompileInstances(context.Background())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get android compile instances", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(compileInstances); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
	return
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	// The main page.
	r.HandleFunc("/", srv.indexHandler).Methods("GET")
	// POST handlers.
	r.HandleFunc(forceSyncPostUrl, srv.forceSyncHandler).Methods("POST")
	r.HandleFunc(pendingTasksPostUrl, srv.pendingTasksHandler).Methods("POST")
	r.HandleFunc(compileInstancesPostUrl, srv.compileInstancesHandler).Methods("POST")
}

// See baseapp.Constructor
func New() (baseapp.App, error) {
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, allowed.Googlers(), allowed.Googlers(), nil)

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_READ_WRITE, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}

	// Initialize cloud datastore.
	if err := util.DatastoreInit(*projectName, *namespace, ts); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	// Start updater for the queue length metrics.
	cleanup.Repeat(time.Minute, func(ctx context.Context) {
		unownedPendingTasks, ownedPendingTasks, err := util.GetPendingCompileTasks("" /* ownedByInstance */)
		if err != nil {
			sklog.Errorf("Failed to get unowned/owned compile tasks: %s", err)
		} else {
			util.QueueLengthMetric.Update(int64(len(unownedPendingTasks)))
			util.RunningLengthMetric.Update(int64(len(ownedPendingTasks)))
		}
	}, nil)

	srv := &Server{}
	srv.loadTemplates()

	return srv, nil
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
	baseapp.Serve(New, []string{*host})
}
