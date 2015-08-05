/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	skutil "go.skia.org/infra/go/util"
)

var (
	dbClient *influxdb.Client = nil
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	reloadChromiumPerfTemplates()
	reloadCaptureSkpsTemplates()
	reloadLuaScriptTemplates()
	reloadChromiumBuildTemplates()
	reloadAdminTaskTemplates()
	reloadTaskCommonTemplates()
}

func Init() {
	reloadTemplates()
}

func userHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com") || strings.HasSuffix(login.LoggedInAs(r), "@chromium.org")
}

func userHasAdminRights(r *http.Request) bool {
	// TODO(benjaminwagner): Add this list to GCE project level metadata and retrieve from there.
	admins := map[string]bool{
		"benjaminwagner@google.com": true,
		"borenet@google.com":        true,
		"jcgregorio@google.com":     true,
		"rmistry@google.com":        true,
		"stephana@google.com":       true,
	}
	return userHasEditRights(r) && admins[login.LoggedInAs(r)]
}

func getIntParam(name string, r *http.Request) (*int, error) {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return nil, nil
	}
	v64, err := strconv.ParseInt(raw[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Invalid value for parameter %q: %s -- %v", name, raw, err)
	}
	v32 := int(v64)
	return &v32, nil
}

func executeSimpleTemplate(template *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	if err := template.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(skutil.MakeResourceHandler(*resourcesDir))

	// Chromium Perf handlers.
	r.HandleFunc("/", chromiumPerfView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_URI, chromiumPerfView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_RUNS_URI, chromiumPerfRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_PERF_PARAMETERS_POST_URI, chromiumPerfHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_CHROMIUM_PERF_TASK_POST_URI, addChromiumPerfTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CHROMIUM_PERF_TASKS_POST_URI, getChromiumPerfTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CHROMIUM_PERF_TASK_POST_URI, updateChromiumPerfTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_CHROMIUM_PERF_TASK_POST_URI, deleteChromiumPerfTaskHandler).Methods("POST")

	// Capture SKPs handlers.
	r.HandleFunc("/"+api.CAPTURE_SKPS_URI, captureSkpsView).Methods("GET")
	r.HandleFunc("/"+api.CAPTURE_SKPS_RUNS_URI, captureSkpRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_CAPTURE_SKPS_TASK_POST_URI, addCaptureSkpsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CAPTURE_SKPS_TASKS_POST_URI, getCaptureSkpTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CAPTURE_SKPS_TASK_POST_URI, updateCaptureSkpsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_CAPTURE_SKPS_TASK_POST_URI, deleteCaptureSkpsTaskHandler).Methods("POST")

	// Lua Script handlers.
	r.HandleFunc("/"+api.LUA_SCRIPT_URI, luaScriptsView).Methods("GET")
	r.HandleFunc("/"+api.LUA_SCRIPT_RUNS_URI, luaScriptRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_LUA_SCRIPT_TASK_POST_URI, addLuaScriptTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_LUA_SCRIPT_TASKS_POST_URI, getLuaScriptTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_LUA_SCRIPT_TASK_POST_URI, updateLuaScriptTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_LUA_SCRIPT_TASK_POST_URI, deleteLuaScriptTaskHandler).Methods("POST")

	// Chromium Build handlers.
	r.HandleFunc("/"+api.CHROMIUM_BUILD_URI, chromiumBuildsView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_BUILD_RUNS_URI, chromiumBuildRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.CHROMIUM_REV_DATA_POST_URI, getChromiumRevDataHandler).Methods("POST")
	r.HandleFunc("/"+api.SKIA_REV_DATA_POST_URI, getSkiaRevDataHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_CHROMIUM_BUILD_TASK_POST_URI, addChromiumBuildTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_CHROMIUM_BUILD_TASKS_POST_URI, getChromiumBuildTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_CHROMIUM_BUILD_TASK_POST_URI, updateChromiumBuildTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_CHROMIUM_BUILD_TASK_POST_URI, deleteChromiumBuildTaskHandler).Methods("POST")

	// Admin Tasks handlers.
	r.HandleFunc("/"+api.ADMIN_TASK_URI, adminTasksView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_PAGE_SETS_RUNS_URI, recreatePageSetsRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.RECREATE_WEBPAGE_ARCHIVES_RUNS_URI, recreateWebpageArchivesRunsHistoryView).Methods("GET")
	r.HandleFunc("/"+api.ADD_RECREATE_PAGE_SETS_TASK_POST_URI, addRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, addRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_PAGE_SETS_TASKS_POST_URI, getRecreatePageSetsTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI, getRecreateWebpageArchivesTasksHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_PAGE_SETS_TASK_POST_URI, updateRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.UPDATE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, updateRecreateWebpageArchivesTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_RECREATE_PAGE_SETS_TASK_POST_URI, deleteRecreatePageSetsTaskHandler).Methods("POST")
	r.HandleFunc("/"+api.DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI, deleteRecreateWebpageArchivesTaskHandler).Methods("POST")

	// Runs history handlers.
	r.HandleFunc("/"+api.RUNS_HISTORY_URI, runsHistoryView).Methods("GET")

	// Task Queue handlers.
	r.HandleFunc("/"+api.PENDING_TASKS_URI, pendingTasksView).Methods("GET")
	r.HandleFunc("/"+api.GET_OLDEST_PENDING_TASK_URI, getOldestPendingTaskHandler).Methods("GET")

	// Common handlers used by different pages.
	r.HandleFunc("/"+api.PAGE_SETS_PARAMETERS_POST_URI, pageSetsHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", skutil.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

// repeatedTasksScheduler looks for all tasks that contain repeat_after_days
// set to > 0 and schedules them when the specified time comes.
func repeatedTasksScheduler() {
	for _ = range time.Tick(5 * time.Minute) {
		glog.Info("Checking for repeated tasks that need to be scheduled..")

		// TODO(rmistry): Complete this implementation.
		// This function needs to do the following:
		// 1. Look for tasks that need to be scheduled in the next 5 minutes.
		// 2. Loop over these tasks.
		//   2.1 Update the task and set repeat_after_days to 0.
		//   2.2 Schedule the task again and set repeat_after_days to what it
		//       originally was.
	}
}

func main() {
	// Setup flags.
	dbConf := db.DBConfigFromFlags()
	influxdb.SetupFlags()

	common.InitWithMetrics("ctfe", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Setup InfluxDB client.
	dbClient, err = influxdb.NewClientFromFlagsAndMetadata(*local)
	if err != nil {
		glog.Fatal(err)
	}

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + "/oauth2callback/"
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, *local)

	glog.Info("CloneOrUpdate complete")

	// Initialize the ctfe database.
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler()

	runServer(serverURL)
}
