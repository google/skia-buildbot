/*
	Android Compile Server for Skia Bots.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/webhook"
)

const (
	// ALL THIS NEEDS TO BE CONVERTED TO USE PUBSUB INSTEAD.
	REGISTER_RUN_POST_URI = "/_/register"
	GET_TASK_STATUS_URI   = "/get_task_status"
)

var (
	// Flags
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	// NEEDED?
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find compile.sh.  If blank then the directory two directories up from this source file will be used.")
	numCheckouts       = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")
	repoUpdateDuration = flag.Duration("repo_update_duration", 1*time.Hour, "How often to update the main Android repository.")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile-staging", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// Used to signal when checkouts are ready to serve requests.
	checkoutsReadyMutex sync.RWMutex
)

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

func main() {
	flag.Parse()

	common.InitWithMust("android_compile", common.PrometheusOpt(promPort))
	defer common.Defer()
	skiaversion.MustLogVersion()

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
}
