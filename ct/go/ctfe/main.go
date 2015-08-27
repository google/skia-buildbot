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
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	metrics "github.com/rcrowley/go-metrics"
	"go.skia.org/infra/ct/go/ctfe/admin_tasks"
	"go.skia.org/infra/ct/go/ctfe/capture_skps"
	"go.skia.org/infra/ct/go/ctfe/chromium_builds"
	"go.skia.org/infra/ct/go/ctfe/chromium_perf"
	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/ctfe/pending_tasks"
	"go.skia.org/infra/ct/go/ctfe/task_common"
	"go.skia.org/infra/ct/go/ctfe/task_types"
	ctfeutil "go.skia.org/infra/ct/go/ctfe/util"
	"go.skia.org/infra/ct/go/db"
	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	skutil "go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

var (
	dbClient *influxdb.Client = nil
)

// flags
var (
	graphiteServer         = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host                   = flag.String("host", "localhost", "HTTP service host")
	port                   = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local                  = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir                = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir           = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	tasksSchedulerWaitTime = flag.Duration("tasks_scheduler_wait_time", 5*time.Minute, "How often the repeated tasks scheduler should run.")
)

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	chromium_perf.ReloadTemplates(*resourcesDir)
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
	r.PathPrefix("/res/").HandlerFunc(skutil.MakeResourceHandler(*resourcesDir))

	chromium_perf.AddHandlers(r)
	capture_skps.AddHandlers(r)
	lua_scripts.AddHandlers(r)
	chromium_builds.AddHandlers(r)
	admin_tasks.AddHandlers(r)
	pending_tasks.AddHandlers(r)
	task_common.AddHandlers(r)

	// Common handlers used by different pages.
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", skutil.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

// startCtfeMetrics registers gauges with the graphite server that indicate CT is running healthily
// and starts a goroutine to update them periodically.
func startCtfeMetrics() {
	pendingTasksGauge := metrics.GetOrRegisterGauge("num-pending-tasks", metrics.DefaultRegistry)
	oldestPendingTaskAgeGauge := metrics.GetOrRegisterGaugeFloat64("oldest-pending-task-age", metrics.DefaultRegistry)
	// 0=no tasks pending; 1=started; 2=not started
	oldestPendingTaskStatusGauge := metrics.GetOrRegisterGauge("oldest-pending-task-status", metrics.DefaultRegistry)
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			pendingTaskCount, err := pending_tasks.GetPendingTaskCount()
			if err != nil {
				glog.Error(err)
			} else {
				pendingTasksGauge.Update(pendingTaskCount)
			}

			oldestPendingTask, err := pending_tasks.GetOldestPendingTask()
			if err != nil {
				glog.Error(err)
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

	for _ = range time.Tick(*tasksSchedulerWaitTime) {
		// Loop over all tasks to find tasks which need to be scheduled.
		for _, prototype := range task_types.Prototypes() {

			query, args := task_common.DBTaskQuery(prototype,
				task_common.QueryParams{
					FutureRunsOnly: true,
					Offset:         0,
					Size:           task_common.MAX_PAGE_SIZE,
				})
			glog.Infof("Running %s", query)
			data, err := prototype.Select(query, args...)
			if err != nil {
				glog.Errorf("Failed to query %s tasks: %v", prototype.GetTaskName(), err)
				continue
			}

			tasks := task_common.AsTaskSlice(data)
			for _, task := range tasks {
				addedTime := ctutil.GetTimeFromTs(strconv.FormatInt(task.GetCommonCols().TsAdded.Int64, 10))
				scheduledTime := addedTime.Add(time.Duration(task.GetCommonCols().RepeatAfterDays) * time.Hour * 24)

				cutOffTime := time.Now().UTC().Add(*tasksSchedulerWaitTime)
				if scheduledTime.Before(cutOffTime) {
					addTaskVars := task.GetPopulatedAddTaskVars()
					if err := task_common.AddTask(addTaskVars); err != nil {
						glog.Errorf("Failed to add task %v: %v", task, err)
						continue
					}

					taskVars := task.GetUpdateTaskVars()
					taskVars.GetUpdateTaskCommonVars().Id = task.GetCommonCols().Id
					taskVars.GetUpdateTaskCommonVars().ClearRepeatAfterDays()
					if err := task_common.UpdateTask(taskVars, task.TableName()); err != nil {
						glog.Errorf("Failed to update task %v: %v", task, err)
						continue
					}
				}
			}
		}
	}
}

func main() {
	defer common.LogPanic()
	// Setup flags.
	dbConf := db.DBConfigFromFlags()
	influxdb.SetupFlags()

	ctfeutil.PreExecuteTemplateHook = func() {
		// Don't use cached templates in local mode.
		if *local {
			reloadTemplates()
		}
	}

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

	if *local {
		webhook.InitRequestSaltForTesting()
	} else {
		webhook.MustInitRequestSaltFromMetadata()
	}

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

	startCtfeMetrics()

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler()

	runServer(serverURL)
}
