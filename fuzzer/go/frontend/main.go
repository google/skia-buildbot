package main

import (
	"flag"
	htemplate "html/template"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fiorix/go-web/autogzip"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/common"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *htemplate.Template = nil

	requestsCounter = metrics.NewRegisteredCounter("requests", metrics.DefaultRegistry)
)

// Command line flags.
var (
	configFilename = flag.String("config", "fuzzer.toml", "Configuration filename")
)

func Init() {
	rand.Seed(time.Now().UnixNano())

	common.InitWithMetricsCB("fuzzer", func() string {
		if _, err := toml.DecodeFile(*configFilename, &config.Config); err != nil {
			glog.Fatalf("Failed to decode config file: %s", err)
		}
		return config.Config.FrontEnd.GraphiteServer
	})

	if config.Config.Common.ResourcePath == "" {
		_, filename, _, _ := runtime.Caller(0)
		config.Config.Common.ResourcePath = filepath.Join(filepath.Dir(filename), "../..")
	}

	path, err := filepath.Abs(config.Config.Common.ResourcePath)
	if err != nil {
		glog.Fatalf("Couldn't get absolute path to fuzzer resources: %s", err)
	}
	if err := os.Chdir(path); err != nil {
		glog.Fatal(err)
	}

	indexTemplate = htemplate.Must(htemplate.ParseFiles(
		filepath.Join(path, "templates/index.html"),
		filepath.Join(path, "templates/header.html"),
		filepath.Join(path, "templates/footer.html"),
	))

}

// mainHandler handles the GET and POST of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)
	requestsCounter.Inc(1)
	if r.Method == "GET" {
		// Expand the template.
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.Execute(w, struct{}{}); err != nil {
			glog.Errorf("Failed to expand template: %q\n", err)
		}
	}
}

func main() {
	flag.Parse()
	Init()
	// Resources are served directly
	http.Handle("/res/", autogzip.Handle(http.FileServer(http.Dir("./"))))
	http.HandleFunc("/", autogzip.HandleFunc(mainHandler))
	glog.Fatal(http.ListenAndServe(config.Config.FrontEnd.Port, nil))
}
