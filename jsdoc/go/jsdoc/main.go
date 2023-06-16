// Serves the jsdoc's for both the elements-sk and common libraries.
package main

import (
	"flag"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "/usr/local/share/jsdoc/", "Root directory of resources to serve.")
)

func main() {
	common.InitWithMust(
		"jsdocserver",
		common.PrometheusOpt(promPort),
	)
	r := chi.NewRouter()
	r.Handle("/*", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir)))

	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
