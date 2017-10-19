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
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// Put it into this dir with tar xf ~/Downloads/cov.html.tar --strip-components=6 -C /usr/local/google/tmp/coverage/abcdef1234/Test-Some-Config-Release/
	extractDir = flag.String("extract_dir", ".", "The directory that the coverage data should be extracted to.")

	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8080", "HTTP service port")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	flag.Parse()
	defer common.LogPanic()

	if *local {
		common.InitWithMust(
			"coverage",
			common.PrometheusOpt(promPort),
		)
	} else {
		common.InitWithMust(
			"coverage",
			common.PrometheusOpt(promPort),
			common.CloudLoggingOpt(),
		)
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", getIndexHandler())
	r.HandleFunc("/coverage", getPageHandler())
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)

	r.PathPrefix("/cov_html/").Handler(http.StripPrefix("/cov_html/", http.FileServer(http.Dir(*extractDir))))

	rootHandler := httputils.LoggingGzipRequestResponse(r)

	http.Handle("/", rootHandler)
	sklog.Infof("Ready to serve on http://127.0.0.1%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))

}

// getIndexHandler returns a handler that displays the index page, which has no
// real templating. The client side JS will query for more information.
func getIndexHandler() func(http.ResponseWriter, *http.Request) {
	tempFiles := []string{
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	}

	indexTemplate := template.Must(template.ParseFiles(tempFiles...))

	return func(w http.ResponseWriter, r *http.Request) {
		if *local {
			indexTemplate = template.Must(template.ParseFiles(tempFiles...))
		}
		w.Header().Set("Content-Type", "text/html")

		if err := indexTemplate.Execute(w, nil); err != nil {
			sklog.Errorf("Failed to expand template: %v", err)
		}
	}
}

// getPageHandler returns a handler that displays the coverage page, which has no
// real templating. The client side JS will make a request to cov_html for an iframe.
func getPageHandler() func(http.ResponseWriter, *http.Request) {
	tempFiles := []string{
		filepath.Join(*resourcesDir, "templates/coverage-page.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	}

	indexTemplate := template.Must(template.ParseFiles(tempFiles...))

	return func(w http.ResponseWriter, r *http.Request) {
		if *local {
			indexTemplate = template.Must(template.ParseFiles(tempFiles...))
		}
		w.Header().Set("Content-Type", "text/html")

		if err := indexTemplate.Execute(w, nil); err != nil {
			sklog.Errorf("Failed to expand template: %v", err)
		}
	}
}
