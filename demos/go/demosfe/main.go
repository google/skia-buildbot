package main

// The webserver for demos.skia.org. It serves the main page to navigate among JS demos.

import (
	//	"encoding/json"
	"flag"
	//"fmt"
	//"io/ioutil"
	//"mime"
	"net/http"
	"path/filepath"
	//"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	//"go.skia.org/infra/go/util"
)

var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)
func main() {
	common.InitWithMust(
		"demos",
	)
	r := mux.NewRouter()
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.FileServer(http.Dir(*resourcesDir))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(*resourcesDir, "main.html"))
	})

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve on http://localhost"+ *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
