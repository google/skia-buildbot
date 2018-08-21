/*
	The Cluster Telemetry Frontend.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/api/option"

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
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	// flags
	host         = flag.String("host", "localhost", "HTTP service host")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	internalPort = flag.String("port", ":8010", "HTTP service intenral port (e.g., ':8010')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	// UNUSED????
	workdir                = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir           = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	tasksSchedulerWaitTime = flag.Duration("tasks_scheduler_wait_time", 5*time.Minute, "How often the repeated tasks scheduler should run.")
	emailClientSecretFile  = flag.String("email_client_secret_file", "/etc/ct-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile    = flag.String("email_token_cache_file", "/etc/ct-email-secrets/client_token.json", "OAuth token cache file for sending email.")

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

// NEEDS TO BE USED SOMEWHERE??
func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func runServer(serverURL string) {

	// For updates by the CT master!
	/*
			internalRouter := mux.NewRouter()
			 internalRouter.HandleFunc("/debug/pprof/", netpprof.Index)
		             internalRouter.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
		                internalRouter.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		                internalRouter.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		                internalRouter.HandleFunc("/debug/pprof/trace", netpprof.Trace)

		                // Add the rest of the application.
		                internalRouter.PathPrefix("/").Handler(appRouter)

		                go func() {
		                        sklog.Infof("Internal server on  http://127.0.0.1" + *internalPort)
		                        sklog.Fatal(http.ListenAndServe(*internalPort, internalRouter))
		                }()
	*/
	internalRouter := mux.NewRouter()

	// Router without login stuff.
	// Handler for displaying results stored in Google Storage.
	// I THINK KEEP THIS BACK! IT SHOULD BE FINE!
	http.HandleFunc("/res/", httputils.MakeResourceHandler(*resourcesDir))
	//http.HandleFunc(ctfeutil.RESULTS_URI, resultsHandler)

	// Do a new one for all of these!
	r := mux.NewRouter()
	admin_tasks.AddHandlers(r, internalRouter)
	capture_skps.AddHandlers(r, internalRouter)
	chromium_analysis.AddHandlers(r, internalRouter)
	chromium_builds.AddHandlers(r, internalRouter)
	chromium_perf.AddHandlers(r, internalRouter) // Note: chromium_perf adds a handler for "/".
	lua_scripts.AddHandlers(r, internalRouter)
	metrics_analysis.AddHandlers(r, internalRouter)
	pending_tasks.AddHandlers(r, internalRouter)
	pixel_diff.AddHandlers(r, internalRouter)

	task_common.AddHandlers(r, internalRouter)

	// Common handlers used by different pages.
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.PathPrefix(ctfeutil.RESULTS_URI).HandlerFunc(resultsHandler)
	//http.Handle("/", httputils.LoggingGzipRequestResponse(r))

	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = iap.None(h)
	}
	h = login.RestrictViewer(h)
	h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)
	fmt.Println("HHHHHHHHHHH")
	h = httputils.ForceHTTPS(h)

	http.Handle("/", h)

	go func() {
		sklog.Infof("Internal server on  http://127.0.0.1" + *internalPort)
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

// TODO(rmistry): Make sure you test this!
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

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Installed struct {
	Installed ClientConfig `json:"installed"`
}

func main() {

	ctfeutil.PreExecuteTemplateHook = func() {
		// Don't use cached templates in local mode.
		if *local {
			reloadTemplates()
		}
	}

	common.InitWithMust("ctfe", common.PrometheusOpt(promPort))
	skiaversion.MustLogVersion()

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	if !*local {
		// Initialize mailing library.
		var cfg Installed
		err := skutil.WithReadFile(*emailClientSecretFile, func(f io.Reader) error {
			return json.NewDecoder(f).Decode(&cfg)
		})
		if err != nil {
			sklog.Fatalf("Failed to read client secrets from %q: %s", *emailClientSecretFile, err)
		}
		// Create a copy of the token cache file since mounted secrets are read-only
		// and the access token will need to be updated for the oauth2 flow.
		if !*local {
			fout, err := ioutil.TempFile("", "")
			if err != nil {
				sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
			}
			err = skutil.WithReadFile(*emailTokenCacheFile, func(fin io.Reader) error {
				_, err := io.Copy(fout, fin)
				if err != nil {
					err = fout.Close()
				}
				return err
			})
			if err != nil {
				sklog.Fatalf("Failed to write token cache file from %q to %q: %s", *emailTokenCacheFile, fout.Name(), err)
			}
			*emailTokenCacheFile = fout.Name()
		}
		ctutil.MailInit(cfg.Installed.ClientID, cfg.Installed.ClientSecret, *emailTokenCacheFile)
	}

	var allow allowed.Allow
	if !*local {
		allow = allowed.NewAllowedFromList(ctfeutil.DomainsWithViewAccess)
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	fmt.Println(allow)
	// rmistry: CHANGE THIS! allow at th eend
	login.InitWithAllow(*port, *local, nil, nil, nil)

	sklog.Info("CloneOrUpdate complete")

	// Initialize the datastore.
	dsTokenSource, err := auth.NewDefaultTokenSource(*local, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	if err := ds.InitWithOpt(*projectName, *namespace, option.WithTokenSource(dsTokenSource)); err != nil {
		sklog.Fatalf("Could not init datastore: %s", err)
	}

	// Create authenticated HTTP client.
	storageTokenSource, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	client = auth.ClientFromTokenSource(storageTokenSource)

	ctx := context.Background()

	startCtfeMetrics(ctx)

	// Start the repeated tasks scheduler.
	go repeatedTasksScheduler(ctx)

	runServer(serverURL)
}
