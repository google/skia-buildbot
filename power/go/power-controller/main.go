package main

import (
	"flag"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil
)

var (
	// web server params
	host         = flag.String("host", "localhost", "HTTP service host")
	port         = flag.String("port", ":8001", "HTTP service port (e.g., ':8002')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	defer common.LogPanic()
	// Calls flag.Parse()
	common.InitWithMust(
		"power-controller",
		common.PrometheusOpt(promPort),
		//common.CloudLoggingOpt(),
	)

	loadTemplates()

	runServer()
}

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

// indexHandler displays the index page, which has no real templating. The client side JS will
// query for more information.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
}

func runServer() {
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", indexHandler)

	rootHandler := httputils.LoggingGzipRequestResponse(r)

	http.Handle("/", rootHandler)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
