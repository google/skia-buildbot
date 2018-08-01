/*
	Android Compile Server for Skia Bots.
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
	"runtime"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/webhook"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	REGISTER_RUN_POST_URI = "/_/register"
	GET_TASK_STATUS_URI   = "/get_task_status"

	PROD_URI = "https://android-compile.skia.org"
)

var (
	// Flags
	host               = flag.String("host", "localhost", "HTTP service host")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port               = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find compile.sh, templates, JS, and CSS files.  If blank then the directory two directories up from this source file will be used.")
	numCheckouts       = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")
	repoUpdateDuration = flag.Duration("repo_update_duration", 15*time.Minute, "How often to update the main Android repository.")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://leasing.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	serverURL string

	// Used to signal when checkouts are ready to serve requests.
	checkoutsReadyMutex sync.RWMutex
)

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
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

	waitingTasks, runningTasks, err := GetCompileTasks()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get compile tasks")
		return
	}

	var templateTasks = struct {
		WaitingTasks []*CompileTask
		RunningTasks []*CompileTask
	}{
		WaitingTasks: waitingTasks,
		RunningTasks: runningTasks,
	}

	if err := indexTemplate.Execute(w, templateTasks); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
		return
	}
	return
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	_, err := webhook.AuthenticateRequest(r)
	if err != nil {
		httputils.ReportError(w, r, err, "Authentication failure")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	taskParam := r.FormValue("task")
	if taskParam == "" {
		httputils.ReportError(w, r, nil, "Missing task parameter")
		return
	}
	taskID, err := strconv.ParseInt(taskParam, 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid task parameter")
		return
	}

	_, t, err := GetDSTask(taskID)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find task")
		return
	}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return

	}

	return
}

func registerRunHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		httputils.ReportError(w, r, err, "Authentication failure")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	task := CompileTask{}
	if err := json.Unmarshal(data, &task); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse request.")
		return
	}

	// Either hash or (issue & patchset) must be specified.
	if task.Hash == "" && (task.Issue == 0 || task.PatchSet == 0) {
		httputils.ReportError(w, r, nil, "Either hash or (issue & patchset) must be specified")
		return
	}

	// Check to see if this task has already been requested and is currently
	// waiting/running. If it is then return the existing ID without triggering
	// a new task. This is done to avoid creating unnecessary duplicate tasks.
	waitingTasksAndKeys, runningTasksAndKeys, err := GetCompileTasksAndKeys()
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to retrieve currently waiting/running compile tasks and keys: %s", err))
		return
	}
	for _, existingTaskAndKey := range append(waitingTasksAndKeys, runningTasksAndKeys...) {
		if (task.Hash != "" && task.Hash == existingTaskAndKey.task.Hash) ||
			(task.Hash == "" && task.Issue == existingTaskAndKey.task.Issue && task.PatchSet == existingTaskAndKey.task.PatchSet) {
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"taskID": existingTaskAndKey.key.ID}); err != nil {
				httputils.ReportError(w, r, err, "Failed to encode JSON")
				return
			}
			sklog.Infof("Got request for already existing task [hash: %s, issue: %d, patchset: %d]. Returning existing ID: %d", task.Hash, task.Issue, task.PatchSet, existingTaskAndKey.key.ID)
			return
		}
	}

	key := GetNewDSKey()
	task.Created = time.Now()
	ctx := context.Background()
	datastoreKey, err := PutDSTask(ctx, key, &task)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Error putting task in datastore: %v", err))
		return
	}

	// Kick off the task and return the task ID.
	triggerCompileTask(ctx, &task, datastoreKey)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"taskID": datastoreKey.ID}); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

// triggerCompileTask runs the specified CompileTask in a goroutine. After
// completion the task is marked as Done and updated in the Datastore.
func triggerCompileTask(ctx context.Context, task *CompileTask, datastoreKey *datastore.Key) {
	go func() {
		checkoutsReadyMutex.RLock()
		defer checkoutsReadyMutex.RUnlock()
		pathToCompileScript := filepath.Join(*resourcesDir, "compile.sh")
		if err := RunCompileTask(ctx, task, datastoreKey, pathToCompileScript); err != nil {
			task.InfraFailure = true
			sklog.Errorf("Error when compiling task with ID %d: %s", datastoreKey.ID, err)
		}
		updateInfraFailureMetric(task.InfraFailure)
		task.Done = true
		task.Completed = time.Now()
		if _, err := UpdateDSTask(ctx, datastoreKey, task); err != nil {
			sklog.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
		}
	}()
}

func runServer() {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", indexHandler)
	r.HandleFunc(REGISTER_RUN_POST_URI, registerRunHandler).Methods("POST")
	r.HandleFunc(GET_TASK_STATUS_URI, statusHandler)

	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.AddLogsRedirect(r)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	flag.Parse()

	if *local {
		// Dont log to cloud in local mode.
		common.InitWithMust(
			"android_compile",
			common.PrometheusOpt(promPort),
		)
		reloadTemplates()
	} else {
		common.InitWithMust(
			"android_compile",
			common.PrometheusOpt(promPort),
			common.CloudLoggingOpt(),
		)
	}
	defer common.Defer()
	skiaversion.MustLogVersion()

	reloadTemplates()
	serverURL = "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	useRedirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.Init(useRedirectURL, *authWhiteList, ""); err != nil {
		sklog.Fatal(fmt.Errorf("Problem setting up server OAuth: %s", err))
	}

	// Initialize cloud datastore.
	if err := DatastoreInit(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	// Initialize checkouts but do not block bringing up the server.
	go func() {
		checkoutsReadyMutex.Lock()
		defer checkoutsReadyMutex.Unlock()
		if err := CheckoutsInit(*numCheckouts, *workdir, *repoUpdateDuration); err != nil {
			sklog.Fatalf("Failed to init checkouts: %s", err)
		}
	}()

	// Initialize webhooks.
	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata("ac_webhook_request_salt")
	}

	// Reset metrics on server startup.
	resetMetrics()

	// Find and reschedule all CompileTasks that are in "running" state. Any
	// "running" CompileTasks means that the server was restarted in the middle
	// of run(s).
	ctx := context.Background()
	_, runningTasksAndKeys, err := GetCompileTasksAndKeys()
	if err != nil {
		sklog.Fatalf("Failed to retrieve compile tasks and keys: %s", err)
	}
	for _, taskAndKey := range runningTasksAndKeys {
		sklog.Infof("Found orphaned task %d. Retriggering it...", taskAndKey.key.ID)
		triggerCompileTask(ctx, taskAndKey.task, taskAndKey.key)
	}

	runServer()
}
