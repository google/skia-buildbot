package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "/usr/local/share/debugger-assets", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	v2AtRoot     = flag.Bool("v2_at_root", false, "Serves /res/v2.html as / (the new dubugger)")
	versionFile  = flag.String("version_file", "/etc/skia-prod/VERSION", "The full path of the Skia VERSION file.")
)

var (
	//
	templates *template.Template

	// version is the version of Skia we are running.
	version string
)

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "/res/v2.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	// Serve new debugger at root when configured to do so.
	if *v2AtRoot {
		context := map[string]string{
			"Version":      version,
			"VersionShort": version[0:7],
		}
		if err := templates.ExecuteTemplate(w, "v2.html", context); err != nil {
			sklog.Errorf("Failed to expand template: %s", err)
		}
	} else {
		http.Redirect(w, r, "https://legacy-debugger.skia.org", http.StatusPermanentRedirect)
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}

	b, err := ioutil.ReadFile(*versionFile)
	if err != nil {
		// Note that version info will be unavailable when running locally
		sklog.Debugf("Did not find Skia version file: %s", err)
		b = []byte("unknown")
	}
	version = strings.TrimSpace(string(b))

	loadTemplates()
}

func main() {
	common.InitWithMust(
		"debugger-assets",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	Init()

	router := mux.NewRouter()
	router.PathPrefix("/res/").HandlerFunc(autogzip.HandleFunc(makeResourceHandler()))
	router.HandleFunc("/", mainHandler)

	http.Handle("/", httputils.HealthzAndHTTPS(httputils.LoggingRequestResponse(router)))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
