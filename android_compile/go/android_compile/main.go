/*
	Android Compile Server for Skia Bots.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

var (
	// Flags
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service port.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find compile.sh.  If blank then the directory two directories up from this source file will be used.")
	numCheckouts       = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")
	repoUpdateDuration = flag.Duration("repo_update_duration", 1*time.Hour, "How often to update the main Android repository.")
	serviceAccount     = flag.String("service_account", "", "Should be set when running in K8s.")

	// Pubsub for storage flags.
	projectID      = flag.String("project_id", "google.com:skia-corp", "Project ID of the Cloud project where the PubSub topic and GS bucket lives.")
	storageBucket  = flag.String("bucket", "android-compile-tasks", "Storage bucket where android compile task JSON files will be kept.")
	subscriberName = flag.String("subscriber", "android-compile-tasks", "ID of the pubsub subscriber.")
	topic          = flag.String("topic", "android-compile-tasks", "Google Cloud PubSub topic of the eventbus.")

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

func readGSAndTriggerCompileTask(ctx context.Context, g *gsFileLocation) error {
	data, err := g.storageClient.Bucket(g.bucket).Object(g.name).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("New reader failed for %s/%s: %s", g.bucket, g.name, err)
	}

	task := CompileTask{}
	if err := json.NewDecoder(data).Decode(&task); err != nil {
		return fmt.Errorf("Failed to parse request: %s", err)
	}

	// Either hash or (issue & patchset) must be specified.
	if task.Hash == "" && (task.Issue == 0 || task.PatchSet == 0) {
		return errors.New("Either hash or (issue & patchset) must be specified")
	}

	// Check to see if this task has already been requested and is currently
	// waiting/running. If it is then do not trigger a new task. This is done
	// to avoid creating unnecessary duplicate tasks.
	waitingTasksAndKeys, runningTasksAndKeys, err := GetCompileTasksAndKeys()
	if err != nil {
		return fmt.Errorf("Failed to retrieve currently waiting/running compile tasks and keys: %s", err)
	}
	for _, existingTaskAndKey := range append(waitingTasksAndKeys, runningTasksAndKeys...) {
		if (task.Hash != "" && task.Hash == existingTaskAndKey.task.Hash) ||
			(task.Hash == "" && task.Issue == existingTaskAndKey.task.Issue && task.PatchSet == existingTaskAndKey.task.PatchSet) {
			sklog.Infof("Got request for already existing task [hash: %s, issue: %d, patchset: %d, id: %d]", task.Hash, task.Issue, task.PatchSet, existingTaskAndKey.key.ID)
			return nil
		}
	}

	key := GetNewDSKey()
	task.Created = time.Now()
	datastoreKey, err := PutDSTask(ctx, key, &task)
	if err != nil {
		return fmt.Errorf("Error putting task in datastore: %s", err)
	}

	// Kick off the task and return the task ID.
	triggerCompileTask(ctx, g, &task, datastoreKey)

	// Update the Google storage file.
	if err := updateTaskInGoogleStorage(ctx, g, task, datastoreKey.ID); err != nil {
		return fmt.Errorf("Could not update task in Google storage: %s", err)
	}

	return nil
}

// triggerCompileTask runs the specified CompileTask in a goroutine. After
// completion the task is marked as Done and updated in the Datastore.
func triggerCompileTask(ctx context.Context, g *gsFileLocation, task *CompileTask, datastoreKey *datastore.Key) {
	go func() {
		checkoutsReadyMutex.RLock()
		defer checkoutsReadyMutex.RUnlock()
		pathToCompileScript := filepath.Join(*resourcesDir, "compile.sh")
		if err := RunCompileTask(ctx, g, task, datastoreKey, pathToCompileScript); err != nil {
			task.InfraFailure = true
			sklog.Errorf("Error when compiling task with ID %d: %s", datastoreKey.ID, err)
		}
		updateInfraFailureMetric(task.InfraFailure)
		task.Done = true
		task.Completed = time.Now()
		if err := UpdateCompileTask(ctx, g, datastoreKey, task); err != nil {
			sklog.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
		}
	}()
}

// UpdateCompileTask updates the task in both Google storage and in Datastore.
func UpdateCompileTask(ctx context.Context, g *gsFileLocation, datastoreKey *datastore.Key, task *CompileTask) error {
	if err := updateTaskInGoogleStorage(ctx, g, *task, datastoreKey.ID); err != nil {
		return fmt.Errorf("Could not update in Google storage compile task in with ID %d: %s", datastoreKey.ID, err)
	}
	if _, err := UpdateDSTask(ctx, datastoreKey, task); err != nil {
		return fmt.Errorf("Could not update in Datastore compile task with ID %d: %s", datastoreKey.ID, err)
	}
	return nil
}

func updateTaskInGoogleStorage(ctx context.Context, g *gsFileLocation, task CompileTask, taskID int64) error {
	// Update the Google storage file with the taskID.
	b, err := json.Marshal(struct {
		CompileTask
		TaskID int64 `json:"task_id"`
	}{
		CompileTask: task,
		TaskID:      taskID,
	})
	if err != nil {
		return fmt.Errorf("Could not re-encode compile task: %s", err)
	}
	wr := g.storageClient.Bucket(g.bucket).Object(g.name).NewWriter(ctx)
	defer util.Close(wr)
	wr.ObjectAttrs.ContentEncoding = "application/json"
	if _, err := wr.Write(b); err != nil {
		return fmt.Errorf("Failed writing JSON to GCS: %s", err)
	}
	return nil
}

type gsFileLocation struct {
	bucket        string
	name          string
	storageClient *storage.Client
}

func newGCSFileLocation(result *storage.ObjectAttrs, storageClient *storage.Client) *gsFileLocation {
	return &gsFileLocation{
		bucket:        result.Bucket,
		name:          result.Name,
		storageClient: storageClient,
	}
}

func main() {
	flag.Parse()

	common.InitWithMust("android_compile", common.PrometheusOpt(promPort), common.MetricsLoggingOpt())
	defer common.Defer()
	skiaversion.MustLogVersion()
	ctx := context.Background()

	if *projectID == "" || *topic == "" || *subscriberName == "" {
		sklog.Fatalf("project_id, topic and subscriber flags must all be set.")
	}

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}

	// Instantiate storage client.
	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatalf("Failed to create a Google Storage API client: %s", err)
	}

	// Initialize cloud datastore.
	if err := DatastoreInit(*projectName, *namespace, ts); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	if !*local {
		// Use the gitcookie created by gitauth package.
		user, err := user.Current()
		if err != nil {
			sklog.Fatal(err)
		}
		gitcookiesPath := filepath.Join(user.HomeDir, ".gitcookies")
		if _, err := gitauth.New(ts, gitcookiesPath, true, *serviceAccount); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
	}

	// Initialize checkouts but do not block bringing up the server.
	go func() {
		checkoutsReadyMutex.Lock()
		defer checkoutsReadyMutex.Unlock()
		if err := CheckoutsInit(*numCheckouts, *workdir, *repoUpdateDuration, storageClient); err != nil {
			sklog.Fatalf("Failed to init checkouts: %s", err)
		}
	}()

	// Subscribe to storage pubsub events.
	eventBus, err := gevent.New(*projectID, *topic, *subscriberName)
	if err != nil {
		sklog.Fatalf("Error creating event bus: %s", err)
	}
	eventType, err := eventBus.RegisterStorageEvents(*storageBucket, "", nil, storageClient)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	resultCh := make(chan *gsFileLocation)
	sklog.Infof("Registered storage events. Eventtype: %s", eventType)
	eventBus.SubscribeAsync(eventType, func(evt interface{}) {
		file := evt.(*eventbus.StorageEvent)
		if file.OverwroteGeneration != "" {
			return
		}
		sklog.Infof("Received storage event: %s / %s\n", file.BucketID, file.ObjectID)

		// Fetch the object attributes.
		objAttr, err := storageClient.Bucket(file.BucketID).Object(file.ObjectID).Attrs(ctx)
		if err != nil {
			sklog.Errorf("Unable to get handle for '%s/%s': %s", file.BucketID, file.ObjectID, err)
			return
		}

		resultCh <- newGCSFileLocation(objAttr, storageClient)
	})

	// Reset metrics on server startup.
	resetMetrics()

	// Find and reschedule all CompileTasks that are in "running" state. Any
	// "running" CompileTasks means that the server was restarted in the middle
	// of run(s). Do not block bringing up the server.
	go func() {
		_, runningTasksAndKeys, err := GetCompileTasksAndKeys()
		if err != nil {
			sklog.Fatalf("Failed to retrieve compile tasks and keys: %s", err)
		}

		for _, taskAndKey := range runningTasksAndKeys {
			sklog.Infof("Found orphaned task %d. Retriggering it...", taskAndKey.key.ID)
			// Fetch the object attributes.
			fileName := fmt.Sprintf("%d-%d.json", taskAndKey.task.Issue, taskAndKey.task.PatchSet)
			objAttr, err := storageClient.Bucket(*storageBucket).Object(fileName).Attrs(ctx)
			if err != nil {
				sklog.Fatalf("Unable to get handle for orphaned task '%s/%s': %s", *storageBucket, fileName, err)
			}

			triggerCompileTask(ctx, newGCSFileLocation(objAttr, storageClient), taskAndKey.task, taskAndKey.key)
		}
	}()

	// Wait for compile task requests that come in.
	go func() {
		for true {
			fileLocation := <-resultCh
			if err = readGSAndTriggerCompileTask(ctx, fileLocation); err != nil {
				sklog.Errorf("Error when reading from GS and triggering compile task: %s", err)
				continue
			}
		}
	}()

	httputils.RunHealthCheckServer(*port)
}
