package main

/*
   Receive pub/sub messages from Swarming, download task stdout, and push to Cloud logging.
*/

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/logging"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

const (
	LOG_ID = "swarming_tasks"

	PUBSUB_SUBSCRIBER_NAME = "skia-swarming-logger"
)

var (
	local          = flag.Bool("local", false, "Use when running locally as opposed to in production.")
	host           = flag.String("host", "localhost", "HTTP server")
	port           = flag.String("port", ":8000", "HTTP service port")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	swarmingServer = flag.String("swarming", "https://chromium-swarm.appspot.com", "Swarming server URL")
	workdir        = flag.String("workdir", ".", "Working directory")

	tl *taskLogger
)

// taskLogger is a struct used for pushing task stdout to Cloud Logging.
type taskLogger struct {
	l *logging.Client
	s swarming.ApiClient
}

func (tl *taskLogger) HandleSwarmingPubSub(taskId string) bool {
	// Obtain the Swarming task data.
	t, err := tl.s.GetTask(taskId, false)
	if err != nil {
		sklog.Errorf("pubsub: Failed to retrieve task from Swarming: %s", err)
		return false
	}
	// Skip unfinished tasks.
	if t.CompletedTs == "" {
		return true
	}

	// Get the task stdout.
	sklog.Infof("Got task %s", taskId)
	output, err := tl.s.SwarmingService().Task.Stdout(taskId).Do()
	if err != nil {
		sklog.Errorf("Failed to obtain Swarming task output: %s", err)
		return false
	}

	// Log the lines from the task.
	logger := tl.l.Logger(LOG_ID, logging.CommonLabels(map[string]string{
		"id": taskId,
	}))
	for i, line := range strings.Split(output.Output, "\n") {
		if err := logger.LogSync(context.Background(), logging.Entry{
			Payload:  line,
			InsertID: fmt.Sprintf("%s_%d", taskId, i),
		}); err != nil {
			sklog.Errorf("Failed to log Swarming task stdout: %s", err)
			return false
		}
	}
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
	defer common.LogPanic()
	common.InitWithMust(
		"swarming_logger",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	logClient, err := logging.NewClient(context.Background(), common.PROJECT_ID)
	if err != nil {
		sklog.Fatal(err)
	}
	tp := httputils.NewBackOffTransport().(*httputils.BackOffTransport)
	tp.Transport.Dial = func(network, addr string) (net.Conn, error) {
		return net.DialTimeout(network, addr, 3*time.Minute)
	}
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}
	oauthCacheFile := path.Join(wdAbs, "google_storage_token.data")
	c, err := auth.NewClientWithTransport(*local, oauthCacheFile, "", tp, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
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
	runServer(serverURL)
}
