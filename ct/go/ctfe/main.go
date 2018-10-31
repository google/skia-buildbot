/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/metrics_analysis"
	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/ctfe/task_types"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

var (
	// flags
	host         = flag.String("host", "localhost", "HTTP service host")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	internalPort = flag.String("internal_port", ":9000", "HTTP service internal port (e.g., ':9000')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	resourcesDir           = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	tasksSchedulerWaitTime = flag.Duration("tasks_scheduler_wait_time", 5*time.Minute, "How often the repeated tasks scheduler should run.")

	// Email params
	emailClientSecretFile = flag.String("email_client_secret_file", "/etc/ct-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/ct-email-secrets/client_token.json", "OAuth token cache file for sending email.")

	// Datastore params
	namespace   = flag.String("namespace", "cluster-telemetry", "The Cloud Datastore namespace, such as 'cluster-telemetry'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// authenticated http client
	client *http.Client
)

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	admin_tasks.ReloadTemplates(*resourcesDir)
	capture_skps.ReloadTemplates(*resourcesDir)
	chromium_analysis.ReloadTemplates(*resourcesDir)
	chromium_builds.ReloadTemplates(*resourcesDir)
	chromium_perf.ReloadTemplates(*resourcesDir)
	lua_scripts.ReloadTemplates(*resourcesDir)
	metrics_analysis.ReloadTemplates(*resourcesDir)
	pending_tasks.ReloadTemplates(*resourcesDir)
	pixel_diff.ReloadTemplates(*resourcesDir)
}

func Init() {
	reloadTemplates()
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

func runServer(serverURL string) {
	externalRouter := mux.NewRouter()
	externalRouter.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	// Router for URLs that do not need to be exposed externally. Eg:
	// updating tasks, querying for pending tasks, terminating tasks.
	internalRouter := mux.NewRouter()

	admin_tasks.AddHandlers(externalRouter, internalRouter)
	capture_skps.AddHandlers(externalRouter, internalRouter)
	chromium_analysis.AddHandlers(externalRouter, internalRouter)
	chromium_builds.AddHandlers(externalRouter, internalRouter)
	chromium_perf.AddHandlers(externalRouter, internalRouter) // Note: chromium_perf adds a handler for "/".
	lua_scripts.AddHandlers(externalRouter, internalRouter)
	metrics_analysis.AddHandlers(externalRouter, internalRouter)
	pending_tasks.AddHandlers(externalRouter, internalRouter)
	pixel_diff.AddHandlers(externalRouter, internalRouter)

	task_common.AddHandlers(externalRouter, internalRouter)

	// Handler for displaying results stored in Google Storage.
	externalRouter.PathPrefix(ctfeutil.RESULTS_URI).HandlerFunc(resultsHandler)

	// Common handlers used by different pages.
	externalRouter.HandleFunc("/json/version", skiaversion.JsonHandler)
	externalRouter.HandleFunc("/loginstatus/", login.StatusHandler)

	h := httputils.LoggingGzipRequestResponse(externalRouter)
	h = login.RestrictViewer(h)
	if !*local {
		h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
	}
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)

	go func() {
		sklog.Infof("Internal server is accessible via %s", *internalPort)
		sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
	}()

	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// startCtfeMetrics registers metrics which indicate CT is running healthily
// and starts a goroutine to update them periodically.
func startCtfeMetrics(ctx context.Context) {
	pendingTasksGauge := metrics2.GetInt64Metric("num_pending_tasks")
	oldestPendingTaskAgeGauge := metrics2.GetFloat64Metric("oldest_pending_task_age")
	// 0=no tasks pending; 1=started; 2=not started
	oldestPendingTaskStatusGauge := metrics2.GetInt64Metric("oldest_pending_task_status")
	go func() {
		for range time.Tick(common.SAMPLE_PERIOD) {
			pendingTaskCount, err := pending_tasks.GetPendingTaskCount(ctx)
			if err != nil {
				sklog.Error(err)
			} else {
				pendingTasksGauge.Update(pendingTaskCount)
			}

			oldestPendingTask, err := pending_tasks.GetOldestPendingTask(ctx)
			if err != nil {
				sklog.Error(err)
			} else if oldestPendingTask == nil {
				oldestPendingTaskAgeGauge.Update(0)
				oldestPendingTaskStatusGauge.Update(0)
			} else {
				addedTime := ctutil.GetTimeFromTs(strconv.FormatInt(oldestPendingTask.GetCommonCols().TsAdded, 10))
				oldestPendingTaskAgeGauge.Update(time.Since(addedTime).Seconds())
				if oldestPendingTask.GetCommonCols().TsStarted != 0 {
					oldestPendingTaskStatusGauge.Update(1)
				} else {
					oldestPendingTaskStatusGauge.Update(2)
				}
			}
		}
	}()
}

// repeatedTasksScheduler looks for all tasks that contain repeat_after_days
// set to > 0 and schedules them when the specified time comes.
// The function does the following:
// 1. Look for tasks that need to be scheduled in the next 5 minutes.
// 2. Loop over these tasks.
//   2.1 Schedule the task again and set repeat_after_days to what it
//       originally was.
//   2.2 Update the original task and set repeat_after_days to 0 since the
//       newly created task will now replace it.
func repeatedTasksScheduler(ctx context.Context) {

	for range time.Tick(*tasksSchedulerWaitTime) {
		// Loop over all tasks to find tasks which need to be scheduled.
		for _, prototype := range task_types.Prototypes() {

			it := task_common.DatastoreTaskQuery(ctx, prototype,
				task_common.QueryParams{
					FutureRunsOnly: true,
					Offset:         0,
					Size:           task_common.MAX_PAGE_SIZE,
				})
			data, err := prototype.Query(it)
			if err != nil {
				sklog.Errorf("Failed to query %s tasks: %v", prototype.GetTaskName(), err)
				continue
			}

			tasks := task_common.AsTaskSlice(data)
			for _, task := range tasks {
				addedTime := ctutil.GetTimeFromTs(strconv.FormatInt(task.GetCommonCols().TsAdded, 10))
				scheduledTime := addedTime.Add(time.Duration(task.GetCommonCols().RepeatAfterDays) * time.Hour * 24)

				cutOffTime := time.Now().UTC().Add(*tasksSchedulerWaitTime)
				if scheduledTime.Before(cutOffTime) {
					addTaskVars, err := task.GetPopulatedAddTaskVars()
					if err != nil {
						sklog.Errorf("Failed to get populated addTaskVars %v: %s", task, err)
						continue
					}
					if _, err := task_common.AddTask(ctx, addTaskVars); err != nil {
						sklog.Errorf("Failed to add task %v: %s", task, err)
						continue
					}

					taskVars := task.GetUpdateTaskVars()
					taskVars.GetUpdateTaskCommonVars().Id = task.GetCommonCols().DatastoreKey.ID
					taskVars.GetUpdateTaskCommonVars().ClearRepeatAfterDays = true
					if err := task_common.UpdateTask(ctx, taskVars, task); err != nil {
						sklog.Errorf("Failed to update task %v: %s", task, err)
						continue
					}
				}
			}
		}
	}
}

func resultsHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Requesting: %s", r.RequestURI)
	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}
	if !login.IsGoogler(r) {
		sklog.Info("User is not a Googler.")
		http.Error(w, "Only Google accounts are allowed.", http.StatusUnauthorized)
		return
	}

	storageURL := fmt.Sprintf("https://storage.googleapis.com/%s", strings.TrimLeft(r.URL.Path, ctfeutil.RESULTS_URI))
	resp, err := client.Get(storageURL)
	if err != nil {
		sklog.Errorf("resultsHandler: Unable to get data from %s: %s", storageURL, err)
		httputils.ReportError(w, r, err, "Unable to get data from google storage")
		return
	}
	defer skutil.Close(resp.Body)
	if resp.StatusCode != 200 {
		sklog.Errorf("resultsHandler: %s returned %d", storageURL, resp.StatusCode)
		httputils.ReportError(w, r, nil, fmt.Sprintf("Google storage request returned %d", resp.StatusCode))
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if _, err := io.Copy(w, resp.Body); err != nil {
		sklog.Errorf("Error when copying response from %s: %s", storageURL, err)
		httputils.ReportError(w, r, err, "Error when copying response from google storage")
		return
	}
}

func main() {

	ctfeutil.PreExecuteTemplateHook = func() {
		// Don't use cached templates in local mode.
		if *local {
			reloadTemplates()
		}
	}

	common.InitWithMust(
		"ctfe",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	skiaversion.MustLogVersion()

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	if !*local {
		// Initialize mailing library.
		if err := ctutil.MailInit(*emailClientSecretFile, *emailTokenCacheFile); err != nil {
			sklog.Fatalf("Could not initialize mailing library: %s", err)
		}
	}

	if *local {
		login.InitWithAllow(*port, *local, nil, nil, nil)
	} else {
		admins := allowed.NewAllowedFromList(ctutil.CtAdmins)
		allow := allowed.NewAllowedFromList(ctfeutil.DomainsWithViewAccess)
		login.InitWithAllow(*port, *local, admins, nil, allow)
	}

	// Initialize the datastore.
	dsTokenSource, err := auth.NewDefaultTokenSource(*local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	if err := ds.InitWithOpt(*projectName, *namespace, option.WithTokenSource(dsTokenSource)); err != nil {
		sklog.Fatalf("Could not init datastore: %s", err)
	}

	// Create authenticated HTTP client.
	storageTokenSource, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_ONLY)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	client = httputils.DefaultClientConfig().WithTokenSource(storageTokenSource).With2xxOnly().Client()

	ctx := context.Background()

	startCtfeMetrics(ctx)

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler(ctx)

	runServer(serverURL)
}
