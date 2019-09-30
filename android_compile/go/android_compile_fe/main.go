/*
	Android Compile Server Frontend.
*/

package main

import (
	"context"
	"flag"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"go.skia.org/infra/android_compile/go/util"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
)

const (
	FORCE_SYNC_POST_URL = "/_/force_sync"
)

var (
	// Flags
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	host         = flag.String("host", "localhost", "HTTP service host")
	port         = flag.String("port", ":8000", "HTTP service port.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find compile.sh and template files.  If blank then the directory two directories up from this source file will be used.")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile-staging", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-corp", "The Google Cloud project name.")

	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	serverURL string
)

func reloadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}

	unownedPendingTasks, ownedPendingTasks, err := util.GetPendingCompileTasks("" /* ownedByInstance */)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get unowned/owned compile tasks", http.StatusInternalServerError)
		return
	}
	androidCompileInstances, err := util.GetAllCompileInstances(context.Background())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get android compile instances", http.StatusInternalServerError)
		return
	}

	var info = struct {
		AndroidCompileInstances []*util.AndroidCompileInstance
		UnownedPendingTasks     []*util.CompileTask
		OwnedPendingTasks       []*util.CompileTask
	}{
		AndroidCompileInstances: androidCompileInstances,
		UnownedPendingTasks:     unownedPendingTasks,
		OwnedPendingTasks:       ownedPendingTasks,
	}

	if err := indexTemplate.Execute(w, info); err != nil {
		httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
		return
	}
	return
}

func forceSyncHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}

	if err := util.SetForceMirrorUpdateOnAllInstances(context.Background()); err != nil {
		httputils.ReportError(w, err, "Failed to set force mirror update on all instances", http.StatusInternalServerError)
		return
	}

	sklog.Infof("Force sync button has been pressed by %s", login.LoggedInAs(r))
	return
}

func runServer() {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", indexHandler)
	r.HandleFunc(FORCE_SYNC_POST_URL, forceSyncHandler)

	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	flag.Parse()

	common.InitWithMust("android_compile_fe", common.PrometheusOpt(promPort), common.MetricsLoggingOpt())
	defer common.Defer()
	skiaversion.MustLogVersion()

	reloadTemplates()
	serverURL = "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), nil)

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore)
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

	runServer()
}
