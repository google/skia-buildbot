package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/am/go/alertclient"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/power/go/decider"
	"go.skia.org/infra/power/go/gatherer"
	"go.skia.org/infra/power/go/recorder"
)

var downBots gatherer.Gatherer = nil
var fixRecorder recorder.Recorder = nil

var (
	// web server params
	port           = flag.String("port", ":8080", "HTTP service port")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	alertsEndpoint = flag.String("alerts_endpoint", "alert-manager:9000", "The alert manager GCE name and port")

	// OAUTH params
	powercycleConfigs = common.NewMultiStringFlag("powercycle_config", nil, "JSON5 file with powercycle bot/device configuration. Same as used for powercycle.")
	updatePeriod      = flag.Duration("update_period", time.Minute, "How often to update the list of down bots.")
	authorizedEmails  = common.NewMultiStringFlag("authorized_email", nil, "Email addresses of users who are authorized to post to this web service.")
)

func main() {
	flag.Parse()

	ctx := context.Background()

	if *local {
		common.InitWithMust(
			"power-controller",
			common.PrometheusOpt(promPort),
		)
	} else {
		common.InitWithMust(
			"power-controller",
			common.PrometheusOpt(promPort),
			common.MetricsLoggingOpt(),
		)
	}

	if err := setupGatherer(ctx); err != nil {
		sklog.Fatalf("Could not set up down bot gatherer: %s", err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/down_bots", downBotsHandler)
	allow := allowed.NewAllowedFromList(*authorizedEmails)
	r.HandleFunc("/powercycled_bots", login.RestrictFn(powercycledBotsHandler, allow))
	r.PathPrefix("/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	rootHandler := httputils.LoggingGzipRequestResponse(r)
	rootHandler = httputils.HealthzAndHTTPS(rootHandler)
	http.Handle("/", rootHandler)
	sklog.Infof("Ready to serve on http://127.0.0.1%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

type downBotsResponse struct {
	List []gatherer.DownBot `json:"list"`
}

func downBotsHandler(w http.ResponseWriter, r *http.Request) {
	if downBots == nil {
		http.Error(w, "The power-controller isn't finished booting up.  Try again later", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	response := downBotsResponse{List: downBots.DownBots()}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// powercycledBotsHandler is the way that the powercycle daemons can talk
// to the server and report that they have powercycled.
func powercycledBotsHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PowercycledBots []string `json:"powercycled_bots"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
		return
	}
	email := login.LoggedInAs(r)
	sklog.Infof("%s reported they powercycled %q", email, input.PowercycledBots)
	fixRecorder.PowercycledBots(email, input.PowercycledBots)
	w.WriteHeader(http.StatusAccepted)
}

func setupGatherer(ctx context.Context) error {
	ts, err := auth.NewDefaultTokenSource(*local, skswarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	authedClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	es, err := skswarming.NewApiClient(authedClient, "chromium-swarm.appspot.com")
	if err != nil {
		return err
	}
	is, err := skswarming.NewApiClient(authedClient, "chrome-swarming.appspot.com")
	if err != nil {
		return fmt.Errorf("Could not get ApiClient for chrome-swarming: %s", err)
	}
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	ac := alertclient.New(c, *alertsEndpoint)
	d, hostMap, err := decider.New(*powercycleConfigs)
	if err != nil {
		return fmt.Errorf("Could not initialize down bot decider: %s", err)
	}

	fixRecorder = recorder.NewCloudLoggingRecorder()
	downBots = gatherer.NewPollingGatherer(ctx, es, is, ac, d, fixRecorder, hostMap, *updatePeriod)

	return nil
}
