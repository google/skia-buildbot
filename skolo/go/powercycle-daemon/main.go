package main

// The powercycle daemon is a continuously running process that helps manage
// aspects of powercycling. It is authenticated to power.skia.org, so it can
// report what was powercycled. In the future, it will be able to monitor
// power.skia.org and automatically trigger reboots, and/or be a web interface
// for swarming bots to powercycle devices.

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	port        = flag.String("port", ":9210", "HTTP service address (e.g., ':8000')")
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promPort    = flag.String("prom_port", ":20004", "Metrics service address (e.g., ':10110')")
	powerServer = flag.String("power_server", "https://power.skia.org", "")
)

var authedClient *http.Client

// powercycledHandler essentially forwards the POST request from
// the powercycle-cli to the powerServer, using the credentials
// from serviceAccountPath to authenticate with the server.
func powercycledHandler(w http.ResponseWriter, r *http.Request) {
	if authedClient != nil {
		url := *powerServer + "/powercycled_bots"
		sklog.Infof("Posting to %s", url)
		defer util.Close(r.Body)
		resp, err := authedClient.Post(url, "application/json", r.Body)
		sklog.Infof("Error: %v", err)
		if resp != nil {
			sklog.Infof("Response: %s", httputils.ReadAndClose(resp.Body))
		} else {
			sklog.Infof("Response was nil")
		}

		w.WriteHeader(http.StatusAccepted)
	} else {
		http.Error(w, "Does not have authenticated client", http.StatusServiceUnavailable)
	}
}

func main() {
	flag.Parse()
	if *local {
		common.InitWithMust(
			"powercycle-daemon",
			common.PrometheusOpt(promPort),
		)
	} else {
		common.InitWithMust(
			"powercycle-daemon",
			common.PrometheusOpt(promPort),
			common.CloudLogging(local, "google.com:skia-buildbots"),
		)
	}

	tokenSource, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail)
	if err != nil {
		sklog.Fatal(err)
	}
	authedClient = httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()
	sklog.Info("Got authenticated client.")

	r := mux.NewRouter()
	r.HandleFunc("/powercycled_bots", powercycledHandler).Methods("POST")

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
