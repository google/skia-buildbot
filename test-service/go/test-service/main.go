package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
	log "go.skia.org/infra/go/sklog/structuredlogging"
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
	)
	defer common.Defer()
	if !*local {
		sklogimpl.SetLogger(log.New(os.Stderr))
	}

	r := chi.NewRouter()
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
	ctx := log.WithContext(context.Background(), log.Context{
		Labels: map[string]string{
			"key": "value",
		},
	})
	go func() {
		for range time.Tick(10 * time.Second) {
			sklog.Infof("Still running...")
			sklog.Infof(`some
multiline

			string`)
			log.Infof(ctx, "blah %d", 284)
		}
	}()
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
