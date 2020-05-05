package main

// The webserver for demos.skia.org. It serves a main page and a set of js+html+css demos.

import (
	"flag"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	demosDir     = flag.String("demos_dir", "./demos/public", "The directory to find named subdirectories for each demo. If blank ./demos/public")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank ./dist will be used.")
)

func main() {
	common.InitWithMust(
		"demos",
	)

	r := mux.NewRouter()
	r.PathPrefix("/demo/").Handler(http.StripPrefix("/demo", http.FileServer(http.Dir(*demosDir))))
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.FileServer(http.Dir(*resourcesDir))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(*resourcesDir, "main.html"))
	})

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve on http://localhost" + *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
