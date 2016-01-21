package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

// flags
var (
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	source       = flag.String("source", "https://debugger.skia.org", "The domain that the Polymer code is served from.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var (
	templates *template.Template
)

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "faketemplates/index.html"),
	))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	loadTemplates()
	if err := templates.ExecuteTemplate(w, "index.html", map[string]string{"source": *source}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	data := struct {
		CommandLine string `json:"command_line"`
		Version     string `json:"version"`
	}{
		CommandLine: "skdebugger --foo --bar",
		Version:     "1.0",
	}
	if err := enc.Encode(data); err != nil {
		glog.Errorln("Failed to encode response: %s", err)
	}
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	common.Init()
	defer common.LogPanic()
	Init()

	router := mux.NewRouter()
	router.HandleFunc("/", indexHandler)
	router.HandleFunc("/_/info", infoHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(router))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
