package main

/*
   Receive pub/sub messages from Swarming, download task stdout, and push to Cloud logging.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/swarming_logger/go/process"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
	logging "google.golang.org/api/logging/v2"
	"google.golang.org/api/option"
)

const (
	LOG_ID                 = "swarming_tasks"
	LOG_NAME_TMPL          = "projects/%s/logs/swarming_%s_%s"
	LOG_RESOURCE_NAME_TMPL = "swarming_%s"

	PUBSUB_SUBSCRIBER_NAME = "skia-swarming-logger"

	// Maximum sizes for logging requests. Give ourselves 20% headroom,
	// just in case our estimates are incorrect.
	MAX_BYTES_ENTRY   = int(112640.0 * 0.80)
	MAX_BYTES_REQUEST = int(10485760.0 * 0.80)
)

var (
	local              = flag.Bool("local", false, "Use when running locally as opposed to in production.")
	host               = flag.String("host", "localhost", "HTTP server")
	port               = flag.String("port", ":8000", "HTTP service port")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	swarmingServer     = flag.String("swarming", swarming.SWARMING_SERVER, "Swarming server URL")
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
	workdir            = flag.String("workdir", ".", "Working directory")

	tl *taskLogger
)

// makeLogName returns the log name for the given Swarming task.
func makeLogName(taskId string) string {
	return fmt.Sprintf(LOG_NAME_TMPL, common.PROJECT_ID, *swarmingServer, taskId)
}

// makeLogResourceName returns a logging.MonitoredResource name for this set of
// logs.
func makeLogResourceName() string {
	return fmt.Sprintf(LOG_RESOURCE_NAME_TMPL, *swarmingServer)
}

// makeLogEntryId returns a LogEntry InsertId based on the given taskId and
// index.
func makeLogEntryId(taskId string, idx int) string {
	return fmt.Sprintf("%s_%d", taskId, idx)
}

// entrySize returns the size in bytes of the given LogEntry.
func entrySize(e *logging.LogEntry) (int, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// requestSize returns the size in bytes of the given WriteLogEntriesRequest.
func requestSize(req *logging.WriteLogEntriesRequest) (int, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// splitOutput splits the output into appropriately-sized LogEntrys. Returns a
// slice of LogEntrys, each of which is shorter than MAX_BYTES_ENTRY, a slice
// of ints representing the sizes in bytes of each LogEntry, or an error if any.
// Splits on newlines first, then splits lines if necessary.
func splitOutput(taskId, output string, startedTs time.Time) ([]*logging.LogEntry, []int, error) {
	lr := &logging.MonitoredResource{
		Type: "logging_log",
		Labels: map[string]string{
			"name": makeLogResourceName(),
		},
	}
	baseEntry := logging.LogEntry{
		LogName:     makeLogName(taskId),
		TextPayload: " ",
		InsertId:    makeLogEntryId(taskId, math.MaxInt32),
		Resource:    lr,
	}
	baseSize, err := entrySize(&baseEntry)
	if err != nil {
		return nil, nil, err
	}
	maxLineLength := MAX_BYTES_ENTRY - baseSize
	lines := strings.Split(output, "\n")
	sklog.Infof("Log has %d lines.", len(lines))
	strs := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		for len(line) > maxLineLength {
			strs = append(strs, line[:maxLineLength])
			line = line[maxLineLength:]
		}
		strs = append(strs, line)
	}
	rv := make([]*logging.LogEntry, 0, len(strs))
	sizes := make([]int, 0, len(strs))
	for i, line := range strs {
		rv = append(rv, &logging.LogEntry{
			LogName:     makeLogName(taskId),
			TextPayload: line,
			InsertId:    makeLogEntryId(taskId, i),
			Resource:    lr,
			Timestamp:   startedTs.Add(time.Duration(i) * time.Nanosecond).Format(util.RFC3339NanoZeroPad),
		})
		sizes = append(sizes, baseSize+len(line)) // Roughly, anyway.
	}
	sklog.Infof("Got %d entries.", len(rv))
	return rv, sizes, nil
}

// taskLogger is a struct used for pushing task stdout to Cloud Logging.
type taskLogger struct {
	l *logging.Service
	s swarming.ApiClient
}

func (tl *taskLogger) HandleSwarmingPubSub(msg *swarming.PubSubTaskMessage) bool {
	// Obtain the Swarming task data.
	t, err := tl.s.GetTask(msg.SwarmingTaskId, false)
	if err != nil {
		sklog.Errorf("pubsub: Failed to retrieve task from Swarming: %s", err)
		return false
	}
	// Skip unfinished tasks.
	if t.CompletedTs == "" {
		return true
	}

	// Get the task stdout.
	sklog.Infof("Got task %s", msg.SwarmingTaskId)
	output, err := tl.s.SwarmingService().Task.Stdout(msg.SwarmingTaskId).Do()
	if err != nil {
		sklog.Errorf("Failed to obtain Swarming task output: %s", err)
		return false
	}

	// Create log entries for log lines.
	startedTs, err := swarming.ParseTimestamp(t.StartedTs)
	if err != nil {
		sklog.Errorf("Failed to parse timestamp: %s", err)
		return false
	}
	entries, sizes, err := splitOutput(msg.SwarmingTaskId, output.Output, startedTs)
	if err != nil {
		sklog.Errorf("Failed to create log entries: %s", err)
		return false
	}
	reqLabels := map[string]string{
		"id": msg.SwarmingTaskId,
	}

	// Get the base request size without any entries.
	baseReqSize, err := requestSize(&logging.WriteLogEntriesRequest{
		Entries: []*logging.LogEntry{},
		Labels:  reqLabels,
	})
	if err != nil {
		sklog.Errorf("Failed to marshal JSON: %s", err)
	}

	reqs := []*logging.WriteLogEntriesRequest{}
	batch := []*logging.LogEntry{}
	reqSize := baseReqSize
	for i, entry := range entries {
		size := sizes[i]
		if reqSize+size > MAX_BYTES_REQUEST {
			reqs = append(reqs, &logging.WriteLogEntriesRequest{
				Entries: batch,
				Labels:  reqLabels,
			})
			sklog.Infof("Req size: %d / %d", reqSize, MAX_BYTES_REQUEST)
			batch = []*logging.LogEntry{}
			reqSize = baseReqSize
		}
		batch = append(batch, entry)
		reqSize += size
	}
	if len(batch) > 0 {
		sklog.Infof("Req size: %d / %d", reqSize, MAX_BYTES_REQUEST)
		reqs = append(reqs, &logging.WriteLogEntriesRequest{
			Entries: batch,
			Labels:  reqLabels,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	var mtx sync.Mutex
	errs := []error{}
	for _, req := range reqs {
		wg.Add(1)
		go func(req *logging.WriteLogEntriesRequest) {
			defer wg.Done()
			if resp, err := tl.l.Entries.Write(req).Context(ctx).Do(); err != nil {
				mtx.Lock()
				defer mtx.Unlock()
				errs = append(errs, fmt.Errorf("Failed to write logs: %s\n\n%+v", err, resp))
				return
			} else if resp.HTTPStatusCode != http.StatusOK {
				mtx.Lock()
				defer mtx.Unlock()
				errs = append(errs, fmt.Errorf("Failed to write logs: resp status code %d", resp.HTTPStatusCode))
				return
			}
		}(req)
	}
	wg.Wait()
	if len(errs) > 0 {
		for _, err := range errs {
			sklog.Error(err)
		}
		return false
	}
	sklog.Infof("Successfully uploaded logs for %s", msg.SwarmingTaskId)
	return true
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	swarming.RegisterPubSubServer(tl, r)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"swarming_logger",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	logTs, err := auth.NewJWTServiceAccountTokenSource("", "", logging.LoggingWriteScope)
	if err != nil {
		sklog.Fatal(err)
	}
	logHttpClient := httputils.DefaultClientConfig().WithTokenSource(logTs).WithDialTimeout(httputils.FAST_DIAL_TIMEOUT).Client()

	logClient, err := logging.New(logHttpClient)
	if err != nil {
		sklog.Fatal(err)
	}
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	oauthCacheFile := path.Join(wdAbs, "google_storage_token.data")
	swarmTs, err := auth.NewLegacyTokenSource(*local, oauthCacheFile, "", swarming.AUTH_SCOPE, storage.ScopeReadWrite)
	if err != nil {
		sklog.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(swarmTs).WithDialTimeout(3 * time.Minute).With2xxOnly().Client()
	swarmClient, err := swarming.NewApiClient(c, *swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}
	tl = &taskLogger{
		l: logClient,
		s: swarmClient,
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	if err := swarming.InitPubSub(serverURL, swarming.PUBSUB_TOPIC_SWARMING_TASKS, PUBSUB_SUBSCRIBER_NAME); err != nil {
		sklog.Fatal(err)
	}
	taskDb, err := remote_db.NewClient(*taskSchedulerDbUrl, httputils.NewTimeoutClient())
	if err != nil {
		sklog.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gcs, err := storage.NewClient(ctx, option.WithHTTPClient(c))
	if err != nil {
		sklog.Fatal(err)
	}
	process.IngestLogsPeriodically(context.Background(), taskDb, gcs, swarmClient)
	runServer(serverURL)
}
