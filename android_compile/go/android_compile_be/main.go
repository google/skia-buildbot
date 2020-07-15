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
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	ac_util "go.skia.org/infra/android_compile/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// This determines the number of go routines we want to run when receiving pubsub messages.
	// When we receive a pubsub msg we handle it immediately and either Ack it or Nack it,
	// there should be no need for parallel receives. Also we do not expect to get tons of parallel
	// tryjob requests so handling one at a time should be ok.
	MAX_PARALLEL_RECEIVES = 1
)

var (
	// Flags
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service port.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	resourcesDir       = flag.String("resources_dir", "", "The directory to find compile.sh and template files.  If blank then the directory two directories up from this source file will be used.")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	numCheckouts       = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")
	repoUpdateDuration = flag.Duration("repo_update_duration", 1*time.Hour, "How often to update the main Android repository.")
	serviceAccount     = flag.String("service_account", "", "Should be set when running in K8s.")

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

	hostname string
)

// triggerCompileTask runs the specified CompileTask in a goroutine. After
// completion the task is marked as Done and updated in the Datastore.
func triggerCompileTask(ctx context.Context, g *gsFileLocation, task *ac_util.CompileTask) {
	go func() {
		checkoutsReadyMutex.RLock()
		defer checkoutsReadyMutex.RUnlock()
		pathToCompileScript := filepath.Join(*resourcesDir, "compile.sh")
		datastoreKey := ac_util.GetTaskDSKey(task.LunchTarget, task.Issue, task.PatchSet)
		sklog.Infof("Triggering %s", datastoreKey.Name)
		if err := RunCompileTask(ctx, g, task, pathToCompileScript); err != nil {
			task.InfraFailure = true
			task.Error = err.Error()
			sklog.Errorf("Error when compiling task with Key %s: %s", datastoreKey.Name, err)
		}
		task.Done = true
		task.Completed = time.Now()
		if err := UpdateCompileTask(ctx, g, task); err != nil {
			sklog.Errorf("Could not update compile task with Key %s: %s", datastoreKey.Name, err)
		}
	}()
}

// UpdateCompileTask updates the task in both Google storage and in Datastore.
func UpdateCompileTask(ctx context.Context, g *gsFileLocation, task *ac_util.CompileTask) error {
	datastoreKey := ac_util.GetTaskDSKey(task.LunchTarget, task.Issue, task.PatchSet)
	if err := updateTaskInGoogleStorage(ctx, g, *task); err != nil {
		return fmt.Errorf("Could not update in Google storage compile task with Key %s: %s", datastoreKey.Name, err)
	}
	if _, err := ac_util.UpdateTaskInDS(ctx, task); err != nil {
		return fmt.Errorf("Could not update in Datastore compile task with Key %s: %s", datastoreKey.Name, err)
	}
	return nil
}

func updateTaskInGoogleStorage(ctx context.Context, g *gsFileLocation, task ac_util.CompileTask) error {
	// Update the Google storage file with the taskID.
	b, err := json.Marshal(struct {
		ac_util.CompileTask
		TaskID string `json:"task_id"`
	}{
		CompileTask: task,
		TaskID:      ac_util.GetTaskDSKey(task.LunchTarget, task.Issue, task.PatchSet).Name,
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

func initPubSub(ts oauth2.TokenSource, resultCh chan *ac_util.CompileTask, storageClient *storage.Client) error {
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
	sub.ReceiveSettings.MaxOutstandingMessages = MAX_PARALLEL_RECEIVES
	sub.ReceiveSettings.NumGoroutines = MAX_PARALLEL_RECEIVES

	go func() {
		for {
			// The pubsub receive callback method does either an Ack()
			// or Nack() with a comment before all returns.
			//
			//
			// The following cases are Nack'ed:
			// * All checkouts on this instance are busy and cannot
			//   pickup the task yet.
			// * This instance is syncing it's mirror and cannot
			//   pickup the task till it is done.
			// * Unknown errors when trying to claim the task.
			//
			//
			// The followed cases are Ack'ed:
			// * Could not decode the pubsub message body.
			// * When storage object is overwritten (not new file). We
			//   only care about processing new files.
			// * The file no longer exists in Google storage.
			// * Could not read the JSON in the file.
			// * There is a datastore entry for the file that says that
			//   the current instance owns it but is not running it yet.
			//   Something probably went wrong and we will Ack it and run
			//   the task.
			// * There is a datastore entry for the file that says that
			//   another instance owns it. This instance will Ack it and not
			//   run the task to avoid duplicated work.
			// * If none of the above Nack or Ack cases matched it then we
			//   will Ack it and run the task.
			//
			if err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
				var message struct {
					Bucket string `json:"bucket"`
					Name   string `json:"name"`
				}
				if err := json.Unmarshal(m.Data, &message); err != nil {
					sklog.Errorf("Failed to decode pubsub message body: %s", err)
					m.Ack() // We'll never be able to handle this message.
					return
				}

				if m.Attributes["eventType"] != "OBJECT_FINALIZE" {
					// We only care about new files, i.e. OBJECT_FINALIZE events.
					m.Ack()
					return
				}
				// The overwroteGeneration attribute only appears in OBJECT_FINALIZE
				// events in the case of an overwrite.
				// Source: https://cloud.google.com/storage/docs/pubsub-notifications
				// We ignore such messages because we only care about
				// processing new files.
				if m.Attributes["overwroteGeneration"] != "" {
					sklog.Debugf("Override for %s", message.Name)
					m.Ack() // An existing request.
					return
				}

				sklog.Infof("Received storage event: %s / %s\n", message.Bucket, message.Name)
				data, err := storageClient.Bucket(message.Bucket).Object(message.Name).NewReader(ctx)
				if err != nil {
					sklog.Errorf("New reader failed for %s/%s: %s", message.Bucket, message.Name, err)
					m.Ack() // Maybe the file no longer exists.
					return
				}
				task := ac_util.CompileTask{}
				if err := json.NewDecoder(data).Decode(&task); err != nil {
					sklog.Errorf("Failed to parse request: %s", err)
					m.Ack() // We'll probably never be able to handle this message.
					return
				}

				// Is this instance ready to pickup new tasks? Checks for the following to decide:
				// * Is mirror sync going on?
				// * Are all checkouts busy?
				// If either of these cases are true then the message is Nack'ed. Hopefully another instance
				// handles it, else this instance will pick it up when free.
				if getMirrorUpdateRunning() {
					sklog.Debugf("Mirror is being updated right now. Nack'ing %s.", message.Name)
					if err := ac_util.AddUnownedCompileTask(&task); err != nil {
						sklog.Error(err)
					}
					m.Nack()
					return
				} else if len(AvailableCheckoutsChan) == 0 {
					sklog.Debugf("All %d checkouts are busy. Nack'ing %s .", *numCheckouts, message.Name)
					if err := ac_util.AddUnownedCompileTask(&task); err != nil {
						sklog.Error(err)
					}
					m.Nack()
					return
				}

				// Check to see if another instance picked up this task, if not then claim it.
				if err := ac_util.ClaimAndAddCompileTask(&task, hostname /* ownedByInstance */); err != nil {
					if err == ac_util.ErrAnotherInstanceRunningTask || err == ac_util.ErrThisInstanceRunningTask {
						sklog.Info(err.Error())
						m.Ack() // An instance is already running this task.
						return
					} else if err == ac_util.ErrThisInstanceOwnsTaskButNotRunning {
						sklog.Info(err.Error())
						m.Ack() // This instance will eventually run this task. Ack to prevent duplicate runs.
						return
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
	ctx := context.Background()

	var err error
	hostname, err = os.Hostname()
	if err != nil {
		sklog.Fatalf("Could not find hostname: %s", err)
	}

	if *projectID == "" || *topicName == "" || *subscriberName == "" {
		sklog.Fatalf("project_id, topic and subscriber flags must all be set.")
	}

	// Create token source.
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore, pubsub.ScopePubSub)
	if err != nil {
		sklog.Fatalf("Problem setting up default token source: %s", err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate storage client.
	storageClient, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatalf("Failed to create a Google Storage API client: %s", err)
	}

	// Initialize cloud datastore.
	if err := ac_util.DatastoreInit(*projectName, *namespace, ts); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	// Register instance with frontend.
	if err := ac_util.UpdateInstanceInDS(ctx, hostname, time.Now().Format("Mon Jan 2 15:04:05 MST"), *repoUpdateDuration, false); err != nil {
		sklog.Fatalf("Failed to update instance kind in datastore: %s", err)
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

	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
	}

	// Initialize checkouts.
	if err := CheckoutsInit(*numCheckouts, *workdir, *repoUpdateDuration, storageClient, httpClient); err != nil {
		sklog.Fatalf("Failed to init checkouts: %s", err)
	}

	// Init pubsub.
	resultCh := make(chan *ac_util.CompileTask)
	if err := initPubSub(ts, resultCh, storageClient); err != nil {
		sklog.Fatal(err)
	}

	// Start listener for when the mirror should be force synced.
	// Update mirror here and then periodically.
	cleanup.Repeat(time.Minute, func(ctx context.Context) {
		// Check the datastore and if it is true then Update the mirror!
		forceMirror, err := ac_util.GetForceMirrorUpdateBool(ctx, hostname)
		if err != nil {
			sklog.Errorf("Could not get force mirror update bool from datastore: %s", err)
		} else if forceMirror {
			sklog.Info("Gone request to force sync mirror. Starting now.")
			UpdateMirror(ctx)
		}
	}, nil)

	// Find and reschedule all CompileTasks that are owned by this instance but did not
	// run to completion. They likely did not run to completion because the server
	// was restarted in the middle of run(s). Do not block bringing up the server.
	// Any "running" CompileTasks means that the server was restarted in the middle
	// of run(s). Do not block bringing up the server.
	go func() {
		_, ownedPendingTasks, err := ac_util.GetPendingCompileTasks(hostname /* ownedByInstance */)
		if err != nil {
			sklog.Fatalf("Failed to retrieve compile tasks and keys: %s", err)
		}

		for _, t := range ownedPendingTasks {
			taskKey := ac_util.GetTaskDSKey(t.LunchTarget, t.Issue, t.PatchSet)
			sklog.Infof("Found orphaned task %s. Retriggering it...", taskKey.Name)
			// Make sure the file exists in GS first.
			fileName := fmt.Sprintf("%s.json", taskKey.Name)
			_, err := storageClient.Bucket(*storageBucket).Object(fileName).Attrs(ctx)
			if err != nil {
				sklog.Fatalf("Unable to get handle for orphaned task '%s/%s': %s", *storageBucket, fileName, err)
			}

			triggerCompileTask(ctx, newGCSFileLocation(*storageBucket, fileName, storageClient), t)
		}
	}()

	// Wait for compile task requests that come in.
	go func() {
		for true {
			compileTask := <-resultCh
			taskKey := ac_util.GetTaskDSKey(compileTask.LunchTarget, compileTask.Issue, compileTask.PatchSet)
			fileName := fmt.Sprintf("%s.json", taskKey.Name)
			triggerCompileTask(ctx, newGCSFileLocation(*storageBucket, fileName, storageClient), compileTask)
		}
	}()

	httputils.RunHealthCheckServer(*port)
}
