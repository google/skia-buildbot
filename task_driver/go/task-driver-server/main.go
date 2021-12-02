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
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/pubsub"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/gorilla/mux"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/db"
	bigtable_db "go.skia.org/infra/task_driver/go/db/bigtable"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/handlers"
	"go.skia.org/infra/task_driver/go/logs"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	subscriptionName                   = "td_server_log_collector"
	subscriptionGoroutines             = 10
	subscriptionMaxOutstandingMessages = 1000
)

var (
	// Flags.
	btInstance   = flag.String("bigtable_instance", "", "BigTable instance to use.")
	btProject    = flag.String("bigtable_project", "", "GCE project to use for BigTable.")
	host         = flag.String("host", "localhost", "HTTP service host")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	project      = flag.String("project_id", "", "GCE Project ID")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the \"dist\" subdirectory of the current directory will be used.")
	hang         = flag.Bool("hang", false, "hang")

	// Database used for storing and retrieving Task Drivers.
	d db.DB

	// BigTable connector used for storing and retrieving logs.
	lm *logs.LogsManager

	// HTML templates.
	tdTemplate *template.Template = nil
)

// logsHandler reads log entries from BigTable and writes them to the ResponseWriter.
func logsHandler(w http.ResponseWriter, r *http.Request, taskId, stepId, logId string) {
	// TODO(borenet): If we had access to the Task Driver DB, we could first
	// retrieve the run and then limit our search to its duration. That
	// might speed up the search quite a bit.
	w.Header().Set("Content-Type", "text/plain")
	entries, err := lm.Search(r.Context(), taskId, stepId, logId)
	if err != nil {
		httputils.ReportError(w, err, "Failed to search log entries.", http.StatusInternalServerError)
		return
	}
	if len(entries) == 0 {
		// TODO(borenet): Maybe an empty log is not the same as a
		// missing log?
		http.Error(w, fmt.Sprintf("No matching log entries were found."), http.StatusNotFound)
		return
	}
	for _, e := range entries {
		line := e.TextPayload
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		if _, err := w.Write([]byte(line)); err != nil {
			httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	}
}

// getVar returns the variable which should be present in the request path.
// It returns "" if it is not found, in which case it also writes an error to
// the ResponseWriter.
func getVar(w http.ResponseWriter, r *http.Request, key string) string {
	val, ok := mux.Vars(r)[key]
	if !ok {
		http.Error(w, fmt.Sprintf("No %s in request path.", key), http.StatusBadRequest)
		return ""
	}
	return val
}

// taskLogsHandler is a handler which serves logs for a given task.
func taskLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getVar(w, r, "taskId")
	if taskId == "" {
		return
	}
	logsHandler(w, r, taskId, "", "")
}

// stepLogsHandler is a handler which serves logs for a given step.
func stepLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getVar(w, r, "taskId")
	stepId := getVar(w, r, "stepId")
	if taskId == "" || stepId == "" {
		return
	}
	logsHandler(w, r, taskId, stepId, "")
}

// singleLogHandler is a handler which serves logs for a single log ID.
func singleLogHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getVar(w, r, "taskId")
	stepId := getVar(w, r, "stepId")
	logId := getVar(w, r, "logId")
	if taskId == "" || stepId == "" || logId == "" {
		return
	}
	logsHandler(w, r, taskId, stepId, logId)
}

// getTaskDriver returns a db.TaskDriverRun instance for the given request. If
// anything went wrong, returns nil and writes an error to the ResponseWriter.
func getTaskDriver(ctx context.Context, w http.ResponseWriter, r *http.Request) *db.TaskDriverRun {
	ctx, span := trace.StartSpan(ctx, "getTaskDriverDisplay")
	defer span.End()
	id := getVar(w, r, "taskId")
	if id == "" {
		return nil
	}
	td, err := d.GetTaskDriver(ctx, id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve task driver.", http.StatusInternalServerError)
		return nil
	}
	if td == nil {
		// The requested task driver doesn't exist. This is a temporary
		// measure while some tasks are running task drivers and others
		// are not: if the client provided a redirect URL, redirect the
		// client, otherwise give a 404.
		if redirect := r.FormValue("ifNotFound"); redirect != "" {
			http.Redirect(w, r, redirect, http.StatusSeeOther)
			return nil
		}
		http.Error(w, "No task driver exists with the given ID.", http.StatusNotFound)
		return nil
	}
	return td
}

// getTaskDriverDisplay returns a display.TaskDriverRunDisplay instance for the
// given request. If anything went wrong, returns nil and writes an error to the
// ResponseWriter.
func getTaskDriverDisplay(ctx context.Context, w http.ResponseWriter, r *http.Request) *display.TaskDriverRunDisplay {
	ctx, span := trace.StartSpan(ctx, "getTaskDriverDisplay")
	defer span.End()
	td := getTaskDriver(ctx, w, r)
	if td == nil {
		// Any error was handled by getTaskDriver.
		return nil
	}
	disp, err := display.TaskDriverForDisplay(td)
	if err != nil {
		httputils.ReportError(w, err, "Failed to format task driver for response.", http.StatusInternalServerError)
		return nil
	}
	return disp
}

// taskDriverHandler handles requests for an individual Task Driver.
func taskDriverHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "taskdriverserver_taskDriverHandler")
	defer span.End()
	disp := getTaskDriverDisplay(ctx, w, r)
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
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
	// When data is missing, we may not have the "name" field.
	taskName := "unknown"
	if disp.StepDisplay != nil {
		taskName = disp.StepDisplay.Name
	}
	page := struct {
		TaskName string
		TaskJson string
	}{
		TaskName: taskName,
		TaskJson: string(b),
	}
	w.Header().Set("Content-Type", "text/html")
	if err := tdTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Server could not load page", http.StatusInternalServerError)
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
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	handlers.AddTaskDriverHandlers(r, d, lm)
	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// handleMessage decodes and inserts an update
func handleMessage(ctx context.Context, msg *pubsub.Message) error {
	ctx, span := trace.StartSpan(ctx, "taskdriverserver_handleMessage")
	defer span.End()
	var e logs.Entry

	_, jspan := trace.StartSpan(ctx, "processJSON")
	if err := json.Unmarshal(msg.Data, &e); err != nil {
		// If the message has badly-formatted data,
		// we'll never be able to parse it, so go ahead
		// and ack it to get it out of the queue.
		msg.Ack()
		jspan.End()
		return err
	}
	jspan.End()
	if e.JsonPayload != nil {
		if err := e.JsonPayload.Validate(); err != nil {
			// If the message has badly-formatted data,
			// we'll never be able to use it, so go ahead
			// and ack it to get it out of the queue.
			msg.Ack()
			return err
		}
		if err := d.UpdateTaskDriver(ctx, e.JsonPayload.TaskId, e.JsonPayload); err != nil {
			// This may be a transient error, so nack the message and hope
			// that we'll be able to handle it on redelivery.
			msg.Nack()
			return fmt.Errorf("Failed to insert task driver update: %s", err)
		}
	} else if e.TextPayload != "" {
		if err := lm.Insert(ctx, &e); err != nil {
			// This may be a transient error, so nack the message and hope
			// that we'll be able to handle it on redelivery.
			msg.Nack()
			return fmt.Errorf("Failed to insert log entry: %s", err)
		}
	} else {
		msg.Ack()
		return fmt.Errorf("Message has no payload: %+v", msg)
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
	if *hang {
		select {}
	}

	// We ingest a lot of log entries, so we only really should trace a small percentage of them
	// by default. If this results in too much data, we can turn it down.
	if err := initializeTracing(0.01); err != nil {
		sklog.Fatalf("Could not set up tracing: %s", err)
	}

	// Setup pubsub.
	ctx := context.Background()
	sub, err := sub.NewWithSubName(ctx, *local, *project, td.PubsubTopicLogs, subscriptionName, subscriptionGoroutines)
	if err != nil {
		sklog.Fatal(err)
	}
	sub.ReceiveSettings.MaxOutstandingMessages = subscriptionMaxOutstandingMessages

	// Create the TaskDriver DB.
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope)
	if err != nil {
		sklog.Fatal(err)
	}
	d, err = bigtable_db.NewBigTableDB(ctx, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	lm, err = logs.NewLogsManager(ctx, *btProject, *btInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Launch a goroutine to listen for pubsub messages.
	go func() {
		sklog.Infof("Waiting for messages.")
		for {
			if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				if err := handleMessage(ctx, msg); err != nil {
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

func initializeTracing(traceSampleProportion float64) error {
	// TODO(kjlubick) deduplicate with Gold and Perf
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		// Use 10 times the default (because that's what perf does).
		TraceSpansBufferMaxBytes: 80_000_000,
		// It is not clear what the default interval is. One minute seems to be a good value since
		// that is the same as our Prometheus metrics are reported.
		ReportingInterval: time.Minute,
		DefaultTraceAttributes: map[string]interface{}{
			// This environment variable should be set in the k8s templates.
			"podName": os.Getenv("K8S_POD_NAME"),
		},
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	trace.RegisterExporter(exporter)
	sampler := trace.ProbabilitySampler(traceSampleProportion)
	trace.ApplyConfig(trace.Config{DefaultSampler: sampler})
	return nil
}
