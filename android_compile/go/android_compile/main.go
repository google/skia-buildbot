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
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	FORCE_SYNC_POST_URL = "/_/force_sync"

	// MAX_PARALLEL_RECEIVES is the number of Go routines we want to run. Determined experimentally.
	MAX_PARALLEL_RECEIVES = 1
)

var (
	// Flags
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	host               = flag.String("host", "localhost", "HTTP service host")
	port               = flag.String("port", ":8000", "HTTP service port.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find compile.sh and template files.  If blank then the directory two directories up from this source file will be used.")
	numCheckouts       = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")
	repoUpdateDuration = flag.Duration("repo_update_duration", 1*time.Hour, "How often to update the main Android repository.")
	serviceAccount     = flag.String("service_account", "", "Should be set when running in K8s.")
	authWhiteList      = flag.String("auth_whitelist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	// Useful for debugging.
	hang = flag.Bool("hang", false, "If true, just hang and do nothing.")

	// Pubsub for storage flags.
	projectID      = flag.String("project_id", "google.com:skia-corp", "Project ID of the Cloud project where the PubSub topic and GS bucket lives.")
	storageBucket  = flag.String("bucket", "android-compile-tasks-staging", "Storage bucket where android compile task JSON files will be kept.")
	subscriberName = flag.String("subscriber", "android-compile-tasks-staging", "ID of the pubsub subscriber.")
	topicName      = flag.String("topic", "android-compile-tasks-staging", "Google Cloud PubSub topic of the eventbus.")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile-staging", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-corp", "The Google Cloud project name.")

	// Used to signal when checkouts are ready to serve requests.
	checkoutsReadyMutex sync.RWMutex

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

	waitingTasks, runningTasks, err := GetCompileTasks()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to get compile tasks")
		return
	}

	var info = struct {
		WaitingTasks         []*CompileTask
		RunningTasks         []*CompileTask
		MirrorLastSynced     string
		MirrorUpdateDuration time.Duration
		MirrorUpdateRunning  bool
	}{
		WaitingTasks:         waitingTasks,
		RunningTasks:         runningTasks,
		MirrorLastSynced:     MirrorLastSynced.Format("Mon Jan 2 15:04:05 MST"),
		MirrorUpdateDuration: *repoUpdateDuration,
		MirrorUpdateRunning:  getMirrorUpdateRunning(),
	}

	if err := indexTemplate.Execute(w, info); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
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

	if getMirrorUpdateRunning() {
		httputils.ReportError(w, r, nil, "Checkout sync is currently in progress")
		return
	}

	sklog.Infof("Force sync button has been pressed by %s", login.LoggedInAs(r))
	UpdateMirror(context.Background())
	return
}

func readGSAndTriggerCompileTask(ctx context.Context, t *CompileTask) error {
	//data, err := g.storageClient.Bucket(g.bucket).Object(g.name).NewReader(ctx)
	//if err != nil {
	//	return fmt.Errorf("New reader failed for %s/%s: %s", g.bucket, g.name, err)
	//}

	//task := CompileTask{}
	//if err := json.NewDecoder(data).Decode(&task); err != nil {
	//	return fmt.Errorf("Failed to parse request: %s", err)
	//}

	//// Either hash or (issue & patchset) must be specified.
	//if task.Hash == "" && (task.Issue == 0 || task.PatchSet == 0) {
	//	return errors.New("Either hash or (issue & patchset) must be specified")
	//}

	//// Set default values if LunchTarget and MMMATargets are not specified.
	//// This is done for backwards compatibility.
	//if task.LunchTarget == "" {
	//	task.LunchTarget = DEFAULT_LUNCH_TARGET
	//}
	//if task.MMMATargets == "" {
	//	task.MMMATargets = DEFAULT_MMMA_TARGETS
	//}

	//// Check to see if this task has already been requested and is currently
	//// waiting/running. If it is then do not trigger a new task. This is done
	//// to avoid creating unnecessary duplicate tasks.
	//waitingTasksAndKeys, runningTasksAndKeys, err := GetCompileTasksAndKeys()
	//if err != nil {
	//	return fmt.Errorf("Failed to retrieve currently waiting/running compile tasks and keys: %s", err)
	//}
	//for _, existingTaskAndKey := range append(waitingTasksAndKeys, runningTasksAndKeys...) {
	//	if task.LunchTarget == existingTaskAndKey.task.LunchTarget &&
	//		((task.Hash != "" && task.Hash == existingTaskAndKey.task.Hash) ||
	//			(task.Hash == "" && task.Issue == existingTaskAndKey.task.Issue && task.PatchSet == existingTaskAndKey.task.PatchSet)) {
	//		sklog.Infof("Got request for already existing task [lunch_target: %s, hash: %s, issue: %d, patchset: %d, id: %d]", task.LunchTarget, task.Hash, task.Issue, task.PatchSet, existingTaskAndKey.key.ID)
	//		return nil
	//	}
	//}

	key := GetNewDSKey(t.LunchTarget, t.Issue, t.PatchSet)
	t.Created = time.Now()
	t.CompileServerInstance = serverURL
	datastoreKey, err := PutDSTask(ctx, key, t)
	if err != nil {
		return fmt.Errorf("Error putting task in datastore: %s", err)
	}

	fmt.Println("SKIPPING TRIGGERING COMPILE TASK")
	fmt.Println(datastoreKey)

	//// Kick off the task and return the task ID.
	//triggerCompileTask(ctx, g, &task, datastoreKey)

	//// Update the Google storage file.
	//if err := updateTaskInGoogleStorage(ctx, g, task, datastoreKey.ID); err != nil {
	//	return fmt.Errorf("Could not update task in Google storage: %s", err)
	//}

	return nil
}

// triggerCompileTask runs the specified CompileTask in a goroutine. After
// completion the task is marked as Done and updated in the Datastore.
func triggerCompileTask(ctx context.Context, g *gsFileLocation, task *CompileTask, datastoreKey *datastore.Key) {
	go func() {
		checkoutsReadyMutex.RLock()
		defer checkoutsReadyMutex.RUnlock()
		pathToCompileScript := filepath.Join(*resourcesDir, "compile.sh")
		// TODO(rmistry): Should also move the available checkouts thing here from RunCompileTask.
		// after it goes here maybe sure nothing else picked it up.
		if err := RunCompileTask(ctx, g, task, datastoreKey, pathToCompileScript); err != nil {
			task.InfraFailure = true
			task.Error = err.Error()
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

func newGCSFileLocation(bucket, name string, storageClient *storage.Client) *gsFileLocation {
	return &gsFileLocation{
		bucket:        bucket,
		name:          name,
		storageClient: storageClient,
	}
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

func initPubSub(ts oauth2.TokenSource, resultCh chan *CompileTask, storageClient *storage.Client) error {
	ctx := context.Background()

	// Create a client.
	client, err := pubsub.NewClient(ctx, *projectID, option.WithTokenSource(ts))
	if err != nil {
		return err
	}

	// Create topic and subscription if necessary.

	// Topic.
	topic := client.Topic(*topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := client.CreateTopic(ctx, *topicName); err != nil {
			return err
		}
	}

	// Subscription.
	subName := fmt.Sprintf("%s+%s", *subscriberName, *topicName)
	sub := client.Subscription(subName)
	exists, err = sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 10 * time.Second,
		}); err != nil {
			return err
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = MAX_PARALLEL_RECEIVES
	sub.ReceiveSettings.NumGoroutines = MAX_PARALLEL_RECEIVES

	AvailableCheckoutsChan = make(chan string, 3)
	AvailableCheckoutsChan <- "checkout1"
	AvailableCheckoutsChan <- "checkout2"
	go func() {
		for {
			// The pubsub receive callback method does the following:
			// * Does either an Ack() or Nack() with a comment before all returns.
			if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
				fmt.Println("IN RECEIVE")
				fmt.Println(len(AvailableCheckoutsChan))

				var message struct {
					Bucket string `json:"bucket"`
					Name   string `json:"name"`
				}
				if err := json.Unmarshal(m.Data, &message); err != nil {
					sklog.Errorf("Failed to decode pubsub message body: %s", err)
					m.Ack() // We'll never be able to handle this message.
					return
				}

				if m.Attributes["overwroteGeneration"] != "" {
					sklog.Infof("Override for %s", message.Name)
					m.Ack() // An existing request.
					return
				}

				sklog.Infof("Received storage event: %s / %s\n", message.Bucket, message.Name)

				if len(AvailableCheckoutsChan) == 0 {
					sklog.Infof("All %d checkouts are busy. Nack'ing %s.", *numCheckouts, message.Name)
					m.Nack() // Hopefully another instance handles it, else this instance will pick it up when free.
					return
				}

				// Check to see if another instance picked up this task, if not then claim it.
				data, err := storageClient.Bucket(message.Bucket).Object(message.Name).NewReader(ctx)
				if err != nil {
					sklog.Errorf("New reader failed for %s/%s: %s", message.Bucket, message.Name, err)
					m.Ack() // Maybe the file no longer exists.
					return
				}
				task := CompileTask{}
				if err := json.NewDecoder(data).Decode(&task); err != nil {
					sklog.Errorf("Failed to parse request: %s", err)
					m.Ack() // We'll probably never be able to handle this message.
					return
				}
				if err := ClaimCompileTask(task.Issue, task.PatchSet, task.LunchTarget); err != nil {
					if err == ErrAnotherInstanceRunningTask || err == ErrThisInstanceRunningTask {
						sklog.Info(err.Error())
						m.Ack() // An instance is already running this task.
						return
					} else if err == ErrThisInstanceOwnsTaskButNotRunning {
						sklog.Info(err.Error())
						// This instance should run this task so continue..
						// NOT 100% SURE this path is correct.
					} else {
						sklog.Errorf("Could not claim %s: %s", message.Name, err)
						m.Nack() // Failed due to unknown reason. Let's try again.
						return
					}
				}

				// Send the new task to the results channel and Ack the message afterwards.
				resultCh <- &task
				m.Ack()
				return
			}); err != nil {
				sklog.Errorf("Failed to receive pubsub messages: %s", err)
			}
		}
	}()
	return nil
}

func main() {
	flag.Parse()

	common.InitWithMust("android_compile", common.PrometheusOpt(promPort), common.MetricsLoggingOpt())
	defer common.Defer()
	skiaversion.MustLogVersion()
	ctx := context.Background()

	if *projectID == "" || *topicName == "" || *subscriberName == "" {
		sklog.Fatalf("project_id, topic and subscriber flags must all be set.")
	}

	reloadTemplates()
	serverURL = "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, allowed.Googlers(), allowed.Googlers(), nil)

	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
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
	//go func() {
	//	checkoutsReadyMutex.Lock()
	//	defer checkoutsReadyMutex.Unlock()
	//	if err := CheckoutsInit(*numCheckouts, *workdir, *repoUpdateDuration, storageClient); err != nil {
	//		sklog.Fatalf("Failed to init checkouts: %s", err)
	//	}
	//}()

	// Init pubsub.
	resultCh := make(chan *CompileTask)
	if err := initPubSub(ts, resultCh, storageClient); err != nil {
		sklog.Fatal(err)
	}

	// Reset metrics on server startup.
	resetMetrics()

	//Find and reschedule all CompileTasks that are owned by this instance.
	//Any "running" CompileTasks means that the server was restarted in the middle
	//of run(s). Do not block bringing up the server.
	//NOT YET
	go func() {
		// HERE HERE needs to be ONLY according to the instance, because the instance will always have
		// capacity to satisfy it's own tasks. It things are movied to waiting then it is totally up for
		// grabs by other instance(s).
		ownedTasksAndKeys, err := GetOwnedCompileTasksAndKeys()
		if err != nil {
			sklog.Fatalf("Failed to retrieve compile tasks and keys: %s", err)
		}

		for _, taskAndKey := range ownedTasksAndKeys {
			sklog.Infof("Found orphaned task %d. Retriggering it...", taskAndKey.key.ID)
			// Fetch the object attributes.
			fileName := fmt.Sprintf("%s-%d-%d.json", taskAndKey.task.LunchTarget, taskAndKey.task.Issue, taskAndKey.task.PatchSet)
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
			compileTask := <-resultCh
			if err = readGSAndTriggerCompileTask(ctx, compileTask); err != nil {
				sklog.Errorf("Error when reading from GS and triggering compile task: %s", err)
				continue
			}
		}
	}()

	runServer()
}
