/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_analysis"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/ctfe/task_types"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/webhook"
)

var (
	// flags
	host                   = flag.String("host", "localhost", "HTTP service host")
	promPort               = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port                   = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local                  = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir                = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir           = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	tasksSchedulerWaitTime = flag.Duration("tasks_scheduler_wait_time", 5*time.Minute, "How often the repeated tasks scheduler should run.")

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
	chromium_analysis.ReloadTemplates(*resourcesDir)
	chromium_perf.ReloadTemplates(*resourcesDir)
	pixel_diff.ReloadTemplates(*resourcesDir)
	capture_skps.ReloadTemplates(*resourcesDir)
	lua_scripts.ReloadTemplates(*resourcesDir)
	chromium_builds.ReloadTemplates(*resourcesDir)
	admin_tasks.ReloadTemplates(*resourcesDir)
	pending_tasks.ReloadTemplates(*resourcesDir)
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

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	chromium_analysis.AddHandlers(r)
	chromium_perf.AddHandlers(r) // Note: chromium_perf adds a handler for "/".
	pixel_diff.AddHandlers(r)
	capture_skps.AddHandlers(r)
	lua_scripts.AddHandlers(r)
	chromium_builds.AddHandlers(r)
	admin_tasks.AddHandlers(r)
	pending_tasks.AddHandlers(r)
	task_common.AddHandlers(r)

	// Handler for proxying to results stored in Google Storage.
	r.PathPrefix(ctfeutil.RESULTS_URI).HandlerFunc(resultsProxyHandler)

	// Common handlers used by different pages.
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(ctfeutil.OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// startCtfeMetrics registers metrics which indicate CT is running healthily
// and starts a goroutine to update them periodically.
func startCtfeMetrics() {
	pendingTasksGauge := metrics2.GetInt64Metric("num_pending_tasks")
	oldestPendingTaskAgeGauge := metrics2.GetFloat64Metric("oldest_pending_task_age")
	// 0=no tasks pending; 1=started; 2=not started
	oldestPendingTaskStatusGauge := metrics2.GetInt64Metric("oldest_pending_task_status")
	go func() {
		for range time.Tick(common.SAMPLE_PERIOD) {
			pendingTaskCount, err := pending_tasks.GetPendingTaskCount()
			if err != nil {
				sklog.Error(err)
			} else {
				pendingTasksGauge.Update(pendingTaskCount)
			}

			oldestPendingTask, err := pending_tasks.GetOldestPendingTask()
			if err != nil {
				sklog.Error(err)
			} else if oldestPendingTask == nil {
				oldestPendingTaskAgeGauge.Update(0)
				oldestPendingTaskStatusGauge.Update(0)
			} else {
				addedTime := ctutil.GetTimeFromTs(strconv.FormatInt(oldestPendingTask.GetCommonCols().TsAdded.Int64, 10))
				oldestPendingTaskAgeGauge.Update(time.Since(addedTime).Seconds())
				if oldestPendingTask.GetCommonCols().TsStarted.Valid {
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
func repeatedTasksScheduler() {

	for range time.Tick(*tasksSchedulerWaitTime) {
		// Loop over all tasks to find tasks which need to be scheduled.
		for _, prototype := range task_types.Prototypes() {

			query, args := task_common.DBTaskQuery(prototype,
				task_common.QueryParams{
					FutureRunsOnly: true,
					Offset:         0,
					Size:           task_common.MAX_PAGE_SIZE,
				})
			sklog.Infof("Running %s", query)
			data, err := prototype.Select(query, args...)
			if err != nil {
				sklog.Errorf("Failed to query %s tasks: %v", prototype.GetTaskName(), err)
				continue
			}

			tasks := task_common.AsTaskSlice(data)
			for _, task := range tasks {
				addedTime := ctutil.GetTimeFromTs(strconv.FormatInt(task.GetCommonCols().TsAdded.Int64, 10))
				scheduledTime := addedTime.Add(time.Duration(task.GetCommonCols().RepeatAfterDays) * time.Hour * 24)

				cutOffTime := time.Now().UTC().Add(*tasksSchedulerWaitTime)
				if scheduledTime.Before(cutOffTime) {
					addTaskVars := task.GetPopulatedAddTaskVars()
					if _, err := task_common.AddTask(addTaskVars); err != nil {
						sklog.Errorf("Failed to add task %v: %v", task, err)
						continue
					}

					taskVars := task.GetUpdateTaskVars()
					taskVars.GetUpdateTaskCommonVars().Id = task.GetCommonCols().Id
					taskVars.GetUpdateTaskCommonVars().ClearRepeatAfterDays()
					if err := task_common.UpdateTask(taskVars, task.TableName()); err != nil {
						sklog.Errorf("Failed to update task %v: %v", task, err)
						continue
					}
				}
			}
		}
	}
}

func resultsProxyHandler(w http.ResponseWriter, r *http.Request) {
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

	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.Host = "storage.googleapis.com"
		req.URL.Host = req.Host
		req.URL.Path = strings.TrimLeft(req.URL.Path, ctfeutil.RESULTS_URI)
	}

	reverseProxy := httputil.ReverseProxy{
		Director:  director,
		Transport: client.Transport,
	}
	reverseProxy.ServeHTTP(w, r)
}

func main() {
	defer common.LogPanic()
	// Setup flags.
	dbConf := db.DBConfigFromFlags()

	ctfeutil.PreExecuteTemplateHook = func() {
		// Don't use cached templates in local mode.
		if *local {
			reloadTemplates()
		}
	}

	common.InitWithMust("ctfe", common.PrometheusOpt(promPort), common.CloudLoggingOpt())
	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	usr, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}
	ctutil.MailInit(filepath.Join(usr.HomeDir, "email.data"))

	redirectURL := serverURL + ctfeutil.OAUTH2_CALLBACK_PATH
	if err := login.Init(redirectURL, strings.Join(ctfeutil.DomainsWithViewAccess, " ")); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata()
	}

	sklog.Info("CloneOrUpdate complete")

	// Initialize the ctfe database.
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			sklog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		sklog.Fatal(err)
	}

	// Create authenticated HTTP client.
	client, err = auth.NewClient(*local, ctutil.GCSTokenPath, auth.SCOPE_READ_ONLY)
	if err != nil {
		sklog.Fatal(err)
	}

	startCtfeMetrics()

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler()

	runServer(serverURL)
}
