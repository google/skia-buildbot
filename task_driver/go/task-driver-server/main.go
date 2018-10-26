package main

/*
	This is a server which collects and serves information about Task Drivers.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/db"
	bigtable_db "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/logsmanager"
	"go.skia.org/infra/task_driver/go/td"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	LOG_NAME_TMPL     = "logName=\"projects/%s/logs/task-driver\""
	SUBSCRIPTION_NAME = "td_server_log_collector"
)

var (
	// Flags.
	host         = flag.String("host", "localhost", "HTTP service host")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	project      = flag.String("project_id", "", "GCE Project ID")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the \"dist\" subdirectory of the current directory will be used.")

	// Database used for storing and retrieving Task Drivers.
	d db.DB

	// BigTable connector used for storing and retrieving logs.
	logs *logsmanager.LogsManager

	// Logs client.
	logsClient *logadmin.Client

	// HTML templates.
	tdTemplate *template.Template = nil
)

// logsHandler reads log entries from Cloud Logging using the given filters and
// writes them to the ResponseWriter.
func logsHandler(w http.ResponseWriter, r *http.Request, filters ...logadmin.EntriesOption) {
	// TODO(borenet): If we had access to the Task Driver DB, we could first
	// retrieve the run and then limit our search to its duration. That
	// might speed up the search quite a bit.
	w.Header().Set("Content-Type", "text/plain")
	opts := append([]logadmin.EntriesOption{
		logadmin.Filter(fmt.Sprintf(LOG_NAME_TMPL, *project)),
	}, filters...)
	sklog.Infof("Searching for logs:")
	for _, filter := range filters {
		sklog.Infof("  filter: %s", filter)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	iter := logsClient.Entries(ctx, opts...)
	for {
		entry, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to iterate log entries.")
			return
		}
		payload, ok := entry.Payload.(string)
		if ok {
			if _, err := w.Write([]byte(payload)); err != nil {
				httputils.ReportError(w, r, err, "Failed to write response.")
				return
			}
		}
	}
}

// taskLogsHandler is a handler which serves logs for a given task.
func taskLogsHandler(w http.ResponseWriter, r *http.Request) {
	t := getTaskDriver(w, r)
	if t == nil {
		// Any error was handled by getTaskDriver.
		return
	}
	opts := []logadmin.EntriesOption{
		logadmin.Filter(fmt.Sprintf("labels.taskId=%s", t.TaskId)),
	}
	root, ok := t.Steps[td.STEP_ID_ROOT]
	if ok {
		if !util.TimeIsZero(root.Started) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp >= "%s"`, root.Started.Format(time.RFC3339))))
		}
		if !util.TimeIsZero(root.Finished) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp <= "%s"`, root.Finished.Format(time.RFC3339))))
		}
	}
	logsHandler(w, r, opts...)
}

// stepLogsHandler is a handler which serves logs for a given step.
func stepLogsHandler(w http.ResponseWriter, r *http.Request) {
	t := getTaskDriver(w, r)
	if t == nil {
		// Any error was handled by getTaskDriver.
		return
	}
	opts := []logadmin.EntriesOption{
		logadmin.Filter(fmt.Sprintf("labels.taskId=%s", t.TaskId)),
	}
	stepId, ok := mux.Vars(r)["stepId"]
	if !ok {
		http.Error(w, "No step ID in request path.", http.StatusBadRequest)
		return
	}
	opts = append(opts, logadmin.Filter(fmt.Sprintf("labels.stepId=%s", stepId)))
	step, ok := t.Steps[stepId]
	if ok {
		if !util.TimeIsZero(step.Started) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp >= "%s"`, step.Started.Format(time.RFC3339))))
		}
		if !util.TimeIsZero(step.Finished) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp <= "%s"`, step.Finished.Format(time.RFC3339))))
		}
	}
	logsHandler(w, r, opts...)
}

// singleLogHandler is a handler which serves logs for a single log ID.
func singleLogHandler(w http.ResponseWriter, r *http.Request) {
	t := getTaskDriver(w, r)
	if t == nil {
		// Any error was handled by getTaskDriver.
		return
	}
	opts := []logadmin.EntriesOption{
		logadmin.Filter(fmt.Sprintf("labels.taskId=%s", t.TaskId)),
	}
	stepId, ok := mux.Vars(r)["stepId"]
	if !ok {
		http.Error(w, "No step ID in request path.", http.StatusBadRequest)
		return
	}
	opts = append(opts, logadmin.Filter(fmt.Sprintf("labels.stepId=%s", stepId)))
	step, ok := t.Steps[stepId]
	if ok {
		if !util.TimeIsZero(step.Started) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp >= "%s"`, step.Started.Format(time.RFC3339))))
		}
		if !util.TimeIsZero(step.Finished) {
			opts = append(opts, logadmin.Filter(fmt.Sprintf(`timestamp <= "%s"`, step.Finished.Format(time.RFC3339))))
		}
	}
	id, ok := mux.Vars(r)["logId"]
	if !ok {
		http.Error(w, "No log ID in request path.", http.StatusBadRequest)
		return
	}
	opts = append(opts, logadmin.Filter(fmt.Sprintf("labels.logId=%s", id)))
	logsHandler(w, r, opts...)
}

// getTaskDriver returns a db.TaskDriverRun instance for the given request. If
// anything went wrong, returns nil and writes an error to the ResponseWriter.
func getTaskDriver(w http.ResponseWriter, r *http.Request) *db.TaskDriverRun {
	id, ok := mux.Vars(r)["taskId"]
	if !ok {
		http.Error(w, "No task driver ID in request path.", http.StatusBadRequest)
		return nil
	}
	td, err := d.GetTaskDriver(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve task driver.")
		return nil
	}
	if td == nil {
		http.Error(w, "No task driver exists with the given ID.", http.StatusNotFound)
		return nil
	}
	return td
}

// getTaskDriverDisplay returns a display.TaskDriverRunDisplay instance for the
// given request. If anything went wrong, returns nil and writes an error to the
// ResponseWriter.
func getTaskDriverDisplay(w http.ResponseWriter, r *http.Request) *display.TaskDriverRunDisplay {
	td := getTaskDriver(w, r)
	if td == nil {
		// Any error was handled by getTaskDriver.
		return nil
	}
	disp, err := display.TaskDriverForDisplay(td)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to format task driver for response.")
		return nil
	}
	return disp
}

// taskDriverHandler handles requests for an individual Task Driver.
func taskDriverHandler(w http.ResponseWriter, r *http.Request) {
	disp := getTaskDriverDisplay(w, r)
	if disp == nil {
		// Any error was handled by getTaskDriverDisplay.
		return
	}

	if *local {
		// reload during local development
		loadTemplates()
	}
	b, err := json.Marshal(disp)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
	page := struct {
		TaskName string
		TaskJson string
	}{
		TaskName: disp.Name,
		TaskJson: string(b),
	}
	w.Header().Set("Content-Type", "text/html")
	if err := tdTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Server could not load page")
		return
	}
}

// jsonTaskDriverHandler returns the JSON representation of the requested Task Driver.
func jsonTaskDriverHandler(w http.ResponseWriter, r *http.Request) {
	disp := getTaskDriverDisplay(w, r)
	if disp == nil {
		// Any error was handled by getTaskDriverDisplay.
		return
	}

	if err := json.NewEncoder(w).Encode(disp); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

// Load the HTML pages.
func loadTemplates() {
	tdTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "task-driver-index.html"),
	))
}

// Run the web server.
func runServer(ctx context.Context, serverURL string) {
	loadTemplates()
	r := mux.NewRouter()
	r.HandleFunc("/td/{taskId}", taskDriverHandler)
	r.HandleFunc("/json/td/{taskId}", jsonTaskDriverHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc("/log/{taskId}", taskLogsHandler)
	r.HandleFunc("/log/{taskId}/{stepId}", stepLogsHandler)
	r.HandleFunc("/log/{taskId}/{stepId}/{logId}", singleLogHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	sklog.AddLogsRedirect(r)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// Entry mimics logging.Entry, which for some reason does not include the
// jsonPayload field, and is not parsable via json.Unmarshal due to the Severity
// type.
type Entry struct {
	Labels      map[string]string `json:"labels"`
	JsonPayload td.Message        `json:"jsonPayload"`
}

// handleMessage decodes and inserts an update
func handleMessage(msg *pubsub.Message) error {
	sklog.Infof("Got message: %+v", msg)
	var e logging.Entry
	if err := json.Unmarshal(msg.Data, &e); err != nil {
		// If the message has badly-formatted data,
		// we'll never be able to parse it, so go ahead
		// and ack it to get it out of the queue.
		msg.Ack()
		return err
	}
	sklog.Infof("Decoded entry: %+v", e)
	if _, ok := e.Payload.(string); ok {
		if err := logs.Insert(&e); err != nil {
			msg.Nack()
			return err
		}
	} else {

		/*if err := d.UpdateTaskDriver(e.JsonPayload.TaskId, &e.JsonPayload); err != nil {
			// This may be a transient error, so nack the message and hope
			// that we'll be able to handle it on redelivery.
			msg.Nack()
			return err
		}*/
	}
	msg.Ack()

	return nil
}

func main() {
	common.InitWithMust(
		"task-driver-server",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()
	if *project == "" {
		sklog.Fatal("--project_id is required.")
	}
	skiaversion.MustLogVersion()

	// Setup pubsub.
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(td.PUBSUB_TOPIC_LOGS)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		topic, err = client.CreateTopic(ctx, td.PUBSUB_TOPIC_LOGS)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	sub := client.Subscription(SUBSCRIPTION_NAME)
	if exists, err := sub.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		sub, err = client.CreateSubscription(ctx, SUBSCRIPTION_NAME, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the logs client.
	ts, err := auth.NewDefaultTokenSource(*local, logging.ReadScope)
	if err != nil {
		sklog.Fatal(err)
	}
	logsClient, err = logadmin.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(logsClient)

	// Create the TaskDriver DB.

	// We read TaskDrivers from *project, but the BigTable instance is
	// actually in skia-public.
	btProject := "skia-public"
	d, err = bigtable_db.NewBigTableDB(ctx, btProject, bigtable_db.BT_INSTANCE, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	logs, err = logsmanager.NewLogsManager(ctx, btProject, logsmanager.BT_INSTANCE, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Launch a goroutine to listen for pubsub messages.
	go func() {
		sklog.Infof("Waiting for messages.")
		for {
			if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				if err := handleMessage(msg); err != nil {
					sklog.Errorf("Failed to handle pubsub message: %s", err)
				}
			}); err != nil {
				sklog.Fatal(err)
			}
		}
	}()

	// Run the web server.
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	runServer(ctx, serverURL)
}
