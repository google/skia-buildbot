package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"go.skia.org/infra/debugger/go/instances"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	source       = flag.String("source", "https://debugger-assets.skia.org", "Where to load assets from.")
	versionFile  = flag.String("version_file", "/etc/skia-prod/VERSION", "The full path of the Skia VERSION file.")
)

var (
	templates *template.Template

	// co handles proxying requests to skiaserve instances which is spins up and down.
	co *instances.Instances

	// version is the version of Skia we are running.
	version string
)

func loadTemplates() {
	templates = template.Must(template.New("").ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/admin.html"),
	))
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if *local {
			loadTemplates()
		}
		if err := templates.ExecuteTemplate(w, name, struct{}{}); err != nil {
			sklog.Error("Failed to expand template:", err)
		}
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	context := map[string]string{
		"Version":      version,
		"VersionShort": version[0:7],
	}
	if err := templates.ExecuteTemplate(w, "index.html", context); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := templates.ExecuteTemplate(w, "admin.html", co.DescribeAll()); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// loadHandler allows an SKP available on the open web to be downloaded into
// skiaserve for debugging.
//
// Expects a single query parameter of "url" that contains the URL of the SKP
// to download.
func loadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}

	// Load the SKP from the given query parameter.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(r.FormValue("url"))
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve the SKP.", http.StatusInternalServerError)
		return
	}
	if resp.StatusCode != 200 {
		httputils.ReportError(w, err, "Failed to retrieve the SKP, bad status code.", http.StatusInternalServerError)
		return
	}
	defer util.Close(r.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		httputils.ReportError(w, err, "Failed to read body.", http.StatusInternalServerError)
		return
	}

	// Now package that SKP up in the multipart/form-file that skiaserve expects.
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	formFile, err := multipartWriter.CreateFormFile("file", "file.skp")
	if err != nil {
		httputils.ReportError(w, err, "Failed to create new multipart/form-file object to pass to skiaserve.", http.StatusInternalServerError)
		return
	}
	if _, err := formFile.Write(b); err != nil {
		httputils.ReportError(w, err, "Failed to copy SKP into multipart/form-file object to pass to skiaserve.", http.StatusInternalServerError)
		return
	}
	if err := multipartWriter.Close(); err != nil {
		httputils.ReportError(w, err, "Failed to close new multipart/form-file object to pass to skiaserve.", http.StatusInternalServerError)
		return
	}

	// POST the image down to skiaserve.
	instanceID := instances.NewInstanceID()
	req, err := http.NewRequest("POST", fmt.Sprintf("/%s/new", instanceID), body)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create new request object to pass to skiaserve.", http.StatusInternalServerError)
		return
	}
	// Copy over cookies so the request is authenticated.
	for _, c := range r.Cookies() {
		req.AddCookie(c)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", multipartWriter.Boundary()))
	rec := httptest.NewRecorder()
	co.ServeHTTP(rec, req)
	if rec.Code >= 400 {
		httputils.ReportError(w, fmt.Errorf("Bad status from SKP upload: Status %d Body %q", rec.Code, rec.Body.String()), "Failed to upload SKP.", http.StatusInternalServerError)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/%s/", instanceID), 303)
	}
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}

	b, err := ioutil.ReadFile(*versionFile)
	if err != nil {
		sklog.Fatalf("Failed to read Skia version: %s", err)
	}
	version = strings.TrimSpace(string(b))

	loadTemplates()
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	common.InitWithMust(
		"debugger",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	co = instances.New(*source)

	Init()

	router := mux.NewRouter()
	router.PathPrefix("/res/").HandlerFunc(autogzip.HandleFunc(makeResourceHandler()))
	router.HandleFunc("/", mainHandler)
	router.HandleFunc("/healthz", httputils.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/admin", adminHandler)
	router.HandleFunc("/loadfrom", loadHandler)

	// All URLs that we don't understand will be routed to be handled by
	// skiaserve.
	router.NotFoundHandler = co

	h := httputils.LoggingRequestResponse(router)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)

	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
