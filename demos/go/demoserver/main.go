package main

// The webserver for demos.skia.org. It serves a main page and a set of js+html+css demos.

import (
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"sort"

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

func loadDemoList() []string {
	f, err := os.Open(*demosDir)
	defer f.Close()
	if err != nil {
		sklog.Fatalf("Unable to open demos_dir: %s", err)
	}
	list, err := f.Readdirnames(0)
	if err != nil {
		sklog.Fatalf("Unable to read demos_dir contents: %s", err)
	}
	sort.Strings(list)
	return list
}

func demolistHandler() func(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(struct {
		Demos []string
	}{
		loadDemoList(),
	})
	if err != nil {
		sklog.Fatalf("Unable to marshal demolist to json: %s", err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

func main() {
	common.InitWithMust(
		"demos",
	)

	r := mux.NewRouter()
	r.PathPrefix("/demo/").Handler(http.StripPrefix("/demo", http.FileServer(http.Dir(*demosDir))))
	// PathPrefix above needs a slash to make FileServer relative paths work.
	// For cleanliness, make sure users get to the directory listing even without the slash.
	r.Handle("/demo", http.RedirectHandler("/demo/", 301))
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.FileServer(http.Dir(*resourcesDir))))
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(*resourcesDir, "main.html"))
	})
	r.HandleFunc("/demolist", demolistHandler())

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Info("Ready to serve on http://localhost" + *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
