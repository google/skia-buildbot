package main

import (
	"flag"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	host     = flag.String("host", "localhost", "HTTP service host")
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port     = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
}

func main() {
	common.InitWithMust(
		"test-service",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	serverURL := "http://" + *host + *port
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
		serverURL = "https://" + *host
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	go func() {
		for range time.Tick(10 * time.Second) {
			sklog.Infof("Still running...")
		}
	}()
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
