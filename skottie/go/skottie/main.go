package main

import (
	"flag"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"skottie",
		common.PrometheusOpt(promPort),
	)
	Init()

	r := mux.NewRouter()
	r.PathPrefix("/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))

	// TODO csrf
	h := httputils.LoggingGzipRequestResponse(router)
	if !*local {
		h = iap.None(h)
	}

	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
