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
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/fiorix/go-web/autogzip"
	"github.com/gorilla/mux"
	"go.skia.org/infra/debugger/go/containers"
	"go.skia.org/infra/debugger/go/runner"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	depotTools        = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	hosted            = flag.Bool("hosted", false, "True if skdebugger should build and run local skiaserve instances itself.")
	influxDatabase    = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost        = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword    = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser        = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
	workRoot          = flag.String("work_root", "", "Directory location where all the work is done.")
	imageDir          = flag.String("image_dir", "", "Directory location of the container.")
)

var (
	templates *template.Template

	// repo is the Skia checkout.
	repo *gitinfo.GitInfo

	// build is responsible to building the LKGR of skiaserve periodically.
	build *buildskia.ContinuousBuilder

	// co handles proxying requests to skiaserve instances which is spins up and down.
	co *containers.Containers
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
			sklog.Errorln("Failed to expand template:", err)
		}
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if !*hosted || login.LoggedInAs(r) == "" {
		if err := templates.ExecuteTemplate(w, "index.html", nil); err != nil {
			sklog.Errorf("Failed to expand template: %s", err)
		}
	} else {
		co.ServeHTTP(w, r)
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if *hosted && !login.IsAdmin(r) {
		http.Error(w, "You must be an administrator to visit this page.", 500)
		return
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
	if !*hosted {
		http.NotFound(w, r)
	}
	if login.LoggedInAs(r) == "" {
		if err := templates.ExecuteTemplate(w, "index.html", nil); err != nil {
			sklog.Errorf("Failed to expand template: %s", err)
		}
		return
	}

	// Load the SKP from the given query parameter.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(r.FormValue("url"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve the SKP.")
		return
	}
	if resp.StatusCode != 200 {
		httputils.ReportError(w, r, err, "Failed to retrieve the SKP, bad status code.")
		return
	}
	defer util.Close(r.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to read body.")
		return
	}

	// Now package that SKP up in the multipart/form-file that skiaserve expects.
	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	formFile, err := multipartWriter.CreateFormFile("file", "file.skp")
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create new multipart/form-file object to pass to skiaserve.")
		return
	}
	if _, err := formFile.Write(b); err != nil {
		httputils.ReportError(w, r, err, "Failed to copy SKP into multipart/form-file object to pass to skiaserve.")
		return
	}
	if err := multipartWriter.Close(); err != nil {
		httputils.ReportError(w, r, err, "Failed to close new multipart/form-file object to pass to skiaserve.")
		return
	}

	// POST the image down to skiaserve.
	req, err := http.NewRequest("POST", "/new", body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create new request object to pass to skiaserve.")
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
		httputils.ReportError(w, r, fmt.Errorf("Bad status from SKP upload: Status %d Body %q", rec.Code, rec.Body.String()), "Failed to upload SKP.")
	} else {
		http.Redirect(w, r, "/", 303)
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func buildSkiaServe(checkout, depotTools string) error {
	sklog.Info("Starting GNGen")
	if err := buildskia.GNGen(checkout, depotTools, "Release", []string{"is_official_build=true"}); err != nil {
		return fmt.Errorf("Failed GN gen: %s", err)
	}

	sklog.Info("Building skiaserve")
	if msg, err := buildskia.GNNinjaBuild(checkout, depotTools, "Release", "skiaserve", true); err != nil {
		return fmt.Errorf("Failed ninja build of skiaserve: %q %s", msg, err)
	}

	return nil
}

// cleanShutdown listens for SIGTERM and then shuts down every container in an
// orderly manner before exiting. If we don't do this then we get systemd
// .scope files left behind which block starting new containers, and the only
// solution is to reboot the instance.
//
// See https://github.com/docker/docker/issues/7015 for more details.
func cleanShutdown() {
	c := make(chan os.Signal, 1)
	// We listen for SIGTERM, which is the first signal that systemd sends when
	// trying to stop a service. It will later follow-up with SIGKILL if the
	// process fails to stop.
	signal.Notify(c, syscall.SIGTERM)
	s := <-c
	sklog.Infof("Orderly shutdown after receiving signal: %s", s)
	co.StopAll()
	// In theory all the containers should be exiting by now, but let's wait a
	// little before exiting ourselves.
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("debugger", influxHost, influxUser, influxPassword, influxDatabase, local)
	if *hosted {
		if *workRoot == "" {
			sklog.Fatal("The --work_root flag is required.")
		}
		if *depotTools == "" {
			sklog.Fatal("The --depot_tools flag is required.")
		}
	}
	Init()

	if *hosted {
		redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
		if !*local {
			redirectURL = "https://debugger.skia.org/oauth2callback/"
		}
		if err := login.Init(redirectURL, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
			sklog.Fatalf("Failed to initialize the login system: %s", err)
		}

		var err error
		repo, err = gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*workRoot, "skia"), true)
		if err != nil {
			sklog.Fatalf("Failed to clone Skia: %s", err)
		}
		build = buildskia.New(*workRoot, *depotTools, repo, buildSkiaServe, 64, *timeBetweenBuilds, true)
		build.Start()

		getHash := func() string {
			return build.Current().Hash
		}

		run := runner.New(*workRoot, *imageDir, getHash, *local)
		co = containers.New(run)

		go cleanShutdown()
	}

	router := mux.NewRouter()
	router.PathPrefix("/res/").HandlerFunc(autogzip.HandleFunc(makeResourceHandler()))
	router.HandleFunc("/", mainHandler)
	router.HandleFunc("/admin", adminHandler)
	if *hosted {
		router.HandleFunc("/loadfrom", loadHandler)
		router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
		router.HandleFunc("/logout/", login.LogoutHandler)
		router.HandleFunc("/loginstatus/", login.StatusHandler)

		// All URLs that we don't understand will be routed to be handled by
		// skiaserve, with the one exception of "/instanceStatus" which will be
		// handled by 'co' itself.
		router.NotFoundHandler = co
	}

	http.Handle("/", httputils.LoggingRequestResponse(router))

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
