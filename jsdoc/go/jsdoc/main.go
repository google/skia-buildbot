// Serves the jsdoc's for both the elements-sk and common libraries.
package main

import (
	"flag"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	common.InitWithMust(
		"jsdocserver",
		common.PrometheusOpt(promPort),
	)
	docsDir := "/usr/local/share/jsdoc/docs"
	elementsDemoDir := "/usr/local/share/jsdoc/elements-sk"
	commonDemoDir := "/usr/local/share/jsdoc/common-sk"
	infraDemoDir := "/usr/local/share/jsdoc/infra-sk"
	r := mux.NewRouter()
	r.PathPrefix("/common-sk/").Handler(http.StripPrefix("/common-sk/", http.HandlerFunc(httputils.MakeResourceHandler(commonDemoDir))))
	r.PathPrefix("/elements-sk/").Handler(http.StripPrefix("/elements-sk/", http.HandlerFunc(httputils.MakeResourceHandler(elementsDemoDir))))
	r.PathPrefix("/infra-sk/").Handler(http.StripPrefix("/infra-sk/", http.HandlerFunc(httputils.MakeResourceHandler(infraDemoDir))))
	r.PathPrefix("/").Handler(http.HandlerFunc(httputils.MakeResourceHandler(docsDir)))

	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
