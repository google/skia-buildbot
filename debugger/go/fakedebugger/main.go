package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
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

const (
	INFO_JSON = `{
        "version": 1,
        "width": 100,
        "height": 100
      }`

	CMD_JSON = `{
        "version": 1,
        "commands": [
          {
            "command": "Save"
          },
          {
            "command": "Matrix",
            "matrix": [
              [ 1, 0, 20 ],
              [ 0, 1, 20 ],
              [ 0, 0, 1 ]
            ]
          },
          {
            "command": "Save"
          },
          {
            "command": "Matrix",
            "matrix": [
              [ 1, 0, 20 ],
              [ 0, 1, 20 ],
              [ 0, 0, 1 ]
            ]
          },
          {
            "command": "Restore"
          },
          {
            "command": "Restore"
          }
        ]
      }`
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

func imgHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")

	b, err := ioutil.ReadFile(filepath.Join(*resourcesDir, "image.png"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load image.")
	}
	if _, err := w.Write(b); err != nil {
		glog.Errorf("Failed to write image: %s", err)
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(INFO_JSON)); err != nil {
		glog.Errorf("Failed to write response: %s", err)
	}
}

func cmdHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(CMD_JSON)); err != nil {
		glog.Errorf("Failed to write response: %s", err)
	}
}

func skpHandler(w http.ResponseWriter, r *http.Request) {
	// We get an SKP posted here. Drop it on the floor.
	http.Redirect(w, r, "/", 303)
	util.Close(r.Body)
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
	router.HandleFunc("/cmd", cmdHandler)
	router.HandleFunc("/img", imgHandler)
	router.HandleFunc("/info", infoHandler)
	router.HandleFunc("/new", skpHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
