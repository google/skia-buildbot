package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	logNameTmpl = "logName=\"projects/%s/logs/task-driver\""
)

var (
	// Flags.
	host     = flag.String("host", "localhost", "HTTP service host")
	internal = flag.Bool("internal", false, "If true, restrict viewers to internal-only.")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port     = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	project  = flag.String("project_id", "skia-swarming-bots", "GCE project.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	// Logs client.
	client *logadmin.Client
)

func logsHandler(w http.ResponseWriter, r *http.Request, filters ...logadmin.EntriesOption) {
	// TODO(borenet): If we had access to the Task Driver DB, we could first
	// retrieve the run and then limit our search to its duration. That
	// might speed up the search quite a bit.
	w.Header().Set("Content-Type", "text/plain")
	opts := append([]logadmin.EntriesOption{
		logadmin.Filter(fmt.Sprintf(logNameTmpl, *project)),
	}, filters...)
	sklog.Infof("Searching for logs...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	iter := client.Entries(ctx, opts...)
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
		} else {
			sklog.Warningf("Got non-string payload: %+v", entry.Payload)
		}
	}
}

// Return a log filter for the task ID, or nil if none is found. Writes an error
// to the ResponseWriter if no task ID is found.
func getTaskIDFilter(w http.ResponseWriter, r *http.Request) logadmin.EntriesOption {
	id, ok := mux.Vars(r)["taskId"]
	if !ok {
		http.Error(w, "No task driver ID in request path.", http.StatusBadRequest)
		return nil
	}
	return logadmin.Filter(fmt.Sprintf("labels.taskId=%s", id))
}

// Return a log filter for the step ID, or nil if none is found. Writes an error
// to the ResponseWriter if no task ID is found.
func getStepIDFilter(w http.ResponseWriter, r *http.Request) logadmin.EntriesOption {
	id, ok := mux.Vars(r)["stepId"]
	if !ok {
		http.Error(w, "No step ID in request path.", http.StatusBadRequest)
		return nil
	}
	return logadmin.Filter(fmt.Sprintf("labels.stepId=%s", id))
}

// Return a log filter for the log ID, or nil if none is found. Writes an error
// to the ResponseWriter if no task ID is found.
func getLogIDFilter(w http.ResponseWriter, r *http.Request) logadmin.EntriesOption {
	id, ok := mux.Vars(r)["logId"]
	if !ok {
		http.Error(w, "No log ID in request path.", http.StatusBadRequest)
		return nil
	}
	return logadmin.Filter(fmt.Sprintf("labels.logId=%s", id))
}

func taskLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getTaskIDFilter(w, r)
	if taskId == nil {
		// Any error was handled by getTaskId.
		return
	}
	logsHandler(w, r, taskId)
}

func stepLogsHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getTaskIDFilter(w, r)
	stepId := getStepIDFilter(w, r)
	if taskId == nil || stepId == nil {
		// Any error was handled above.
		return
	}
	logsHandler(w, r, taskId, stepId)
}

func singleLogHandler(w http.ResponseWriter, r *http.Request) {
	taskId := getTaskIDFilter(w, r)
	stepId := getStepIDFilter(w, r)
	logId := getLogIDFilter(w, r)
	if taskId == nil || stepId == nil || logId == nil {
		// Any error was handled above.
		return
	}
	logsHandler(w, r, taskId, stepId, logId)
}

func main() {
	common.InitWithMust(
		"scribe",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()
	skiaversion.MustLogVersion()

	// Create the logs client.
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, logging.ReadScope)
	if err != nil {
		sklog.Fatal(err)
	}
	client, err = logadmin.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(client)

	// Start the server.
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	var viewAllow allowed.Allow
	if *internal {
		viewAllow = allowed.Googlers()
	}
	login.InitWithAllow(*port, *local, allowed.Googlers(), allowed.Googlers(), viewAllow)

	r := mux.NewRouter()
	r.HandleFunc("/{taskId}", taskLogsHandler)
	r.HandleFunc("/{taskId}/{stepId}", stepLogsHandler)
	r.HandleFunc("/{taskId}/{stepId}/{logId}", singleLogHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		if viewAllow != nil {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, "/oauth2callback/")
		}
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
