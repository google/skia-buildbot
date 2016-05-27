package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"html/template"
	ttemplate "html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"google.golang.org/cloud/storage"

	"github.com/golang/groupcache/lru"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
	"go.skia.org/infra/imageinfo/go/store"
)

const (
	NUM_CACHED_RESULT_IMAGES = 1000

	MAX_BODY_SIZE = 50 * 1024 * 1024
)

// flags
var (
	depotTools        = flag.String("depot_tools", "", "Directory location where depot_tools is installed.")
	influxDatabase    = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost        = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword    = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser        = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir      = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	timeBetweenBuilds = flag.Duration("time_between_builds", time.Hour, "How long to wait between building LKGR of Skia.")
	verbose           = flag.Bool("verbose", false, "Verbose logging.")
	workRoot          = flag.String("work_root", "", "Directory location where all the work is done.")
)

// Context is the structure we use for the expanding the info.html template.
type Context struct {
	Source string `json:"source"` // URL of the source image.
	Output string `json:"output"` // Name of the output image file. A relative URL to /vis/.
	StdOut string `json:"stdout"` // The text output of running the app.
}

var (
	templates *template.Template

	// Will be used later when we start reporting the git hash of the version of Skia we've built.
	funcMap = ttemplate.FuncMap{
		"chop": func(s string) string {
			if len(s) > 6 {
				return s[:6]
			}
			return s
		},
	}

	// cache is a cache of the generated gamut images.
	cache *lru.Cache

	// repo is the Skia checkout.
	repo *gitinfo.GitInfo

	// build is responsible to building visualize_color_gamut.
	build *buildskia.ContinuousBuilder

	// imageStore is a wrapper around Google Storage.
	imageStore *store.Store

	repoSyncFailures = metrics2.GetCounter("repo-sync-failed", nil)
	buildFailures    = metrics2.GetCounter("builds-failed", nil)
	buildLiveness    = metrics2.NewLiveness("build")
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").Funcs(funcMap).ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/info.html"),
		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	context := struct {
		LoggedIn bool
	}{
		LoggedIn: login.LoggedInAs(r) != "",
	}
	if err := templates.ExecuteTemplate(w, "index.html", context); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
	}
}

// uploadHandler handles POSTs of images to be analyzed.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to upload an image.", 403)
		return
	}
	if *local {
		loadTemplates()
	}
	if err := r.ParseMultipartForm(MAX_BODY_SIZE); err != nil {
		httputils.ReportError(w, r, err, "Could not parse POST request body.")
		return
	}
	f, fh, err := r.FormFile("upload")
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find image in POST request body.")
		return
	}
	defer util.Close(f)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not read image in POST request body.")
		return
	}

	// Checksum image.
	hash := fmt.Sprintf("%x", md5.Sum(b))

	// Store to Google Storage.
	if err := imageStore.Put(b, hash, fh.Header.Get("Content-Type"), user); err != nil {
		httputils.ReportError(w, r, err, "Could not write image into storage.")
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/info?hash=%s", hash), 303)
}

// imageHandler serves up the image for the identifying hash.
func imageHandler(w http.ResponseWriter, r *http.Request) {
	hash := mux.Vars(r)["hash"]
	b, contentType, err := imageStore.Get(hash)
	if err == storage.ErrObjectNotExist {
		http.NotFound(w, r)
		return
	} else if err != nil {
		httputils.ReportError(w, r, err, "Unable to load image.")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Add("Cache-Control", "max-age=360000")
	if _, err := w.Write(b); err != nil {
		glog.Errorf("Unable to write image: %s", err)
		return
	}
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	url := r.FormValue("url")
	hash := r.FormValue("hash")
	if url == "" && hash == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing required parameter."), "Missing required parameter.")
		return
	}

	// Make a tmp dir to do our work in.
	dir, err := ioutil.TempDir(filepath.Join(*workRoot, "tmp"), "imageinfo_")
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create temp dir to work in.")
		return
	}

	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			glog.Errorf("Failed to clean up tmp dir: %s", err)
		}
	}()

	var data []byte
	if url != "" {
		resp, err := http.Get(url)
		defer util.Close(resp.Body)

		// Copy the body out to a file.
		// But limit the total size of the file.
		lr := &io.LimitedReader{
			R: resp.Body,
			N: MAX_BODY_SIZE,
		}
		data, err = ioutil.ReadAll(lr)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to download image from the web.")
			return
		}
	} else {
		data, _, err = imageStore.Get(hash)
		if err == storage.ErrObjectNotExist {
			http.NotFound(w, r)
			return
		} else if err != nil {
			httputils.ReportError(w, r, err, "Failed to download image from storage.")
			return
		}
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "input"), data, 0666); err != nil {
		httputils.ReportError(w, r, err, "Failed to write image into temp dir.")
		return
	}

	// Find the current build of Skia.
	current := build.Current()
	exe := filepath.Join(*workRoot, "versions", current.Hash, "out", "Release", "visualize_color_gamut")
	resources := filepath.Join(*workRoot, "versions", current.Hash, "resources")

	// buf is for the stdout/stderr output of running visualize_color_gamut.
	buf := bytes.Buffer{}
	comb := limitwriter.New(&buf, 64*1024)

	// Run visualize_color_gamut.
	visCmd := &exec.Command{
		Name: exe,
		Args: []string{
			"--input", filepath.Join(dir, "input"),
			"--output", filepath.Join(dir, "output.png"),
			"--resourcePath", resources,
		},
		Dir:            "/tmp",
		CombinedOutput: comb,
		InheritPath:    true,
		LogStderr:      true,
		LogStdout:      *verbose,
	}
	glog.Infof("About to run: %#v", *visCmd)
	if err := exec.SimpleRun(visCmd); err != nil {
		glog.Infof("Combined Output %s", buf.String())
		httputils.ReportError(w, r, err, "Failed to execute visualize_color_gamut.")
		return
	}
	output, err := ioutil.ReadFile(filepath.Join(dir, "output.png"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find output file.")
		return
	}
	key := fmt.Sprintf("%x", md5.Sum(output))
	cache.Add(key, output)
	source := url
	if hash != "" {
		source = fmt.Sprintf("/img/%s", hash)
	}
	cp := &Context{
		Source: source,
		Output: key,
		StdOut: buf.String(),
	}
	if err := templates.ExecuteTemplate(w, "info.html", cp); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
		return
	}
}

func visHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	bytes, ok := cache.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Add("Cache-Control", "max-age=36000")
	w.Header().Set("Content-Type", "image/png")
	if _, err := w.Write(bytes.([]byte)); err != nil {
		glog.Errorf("Failed to write image: %s", err)
		return
	}
}

func singleBuildLatest() {
	if err := repo.Update(true, true); err != nil {
		glog.Errorf("Failed to update skia repo used to look up git hashes: %s", err)
		repoSyncFailures.Inc(1)
	}
	repoSyncFailures.Reset()
	ci, err := build.BuildLatestSkia(false, false, false)
	if err != nil {
		glog.Errorf("Failed to build LKGR: %s", err)
		// Only measure real build failures, not a failure if LKGR hasn't updated.
		if err != buildskia.AlreadyExistsErr {
			buildFailures.Inc(1)
		}
		return
	}
	buildFailures.Reset()
	buildLiveness.Reset()
	glog.Infof("Successfully built: %s %s", ci.Hash, ci.Subject)
}

// StartBuilding starts a Go routine that periodically tries to
// download the Skia LKGR and build it.
func StartBuilding() {
	go func() {
		singleBuildLatest()
		for _ = range time.Tick(*timeBetweenBuilds) {
			singleBuildLatest()
		}
	}()
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

// buildVisualize, given a directory that Skia is checked out into,
// builds the specific targets we need for this application.
func buildVisualize(checkout, depotTools string) error {
	// Do a gyp build of visualize_color_gamut.
	glog.Info("Starting build of visualize_color_gamut")
	if err := buildskia.NinjaBuild(checkout, depotTools, []string{}, buildskia.RELEASE_BUILD, "visualize_color_gamut", runtime.NumCPU(), true); err != nil {
		return fmt.Errorf("Failed to build: %s", err)
	}
	return nil
}

func main() {
	defer common.LogPanic()
	if *local {
		common.Init()
	} else {
		common.InitWithMetrics2("imageinfo", influxHost, influxUser, influxPassword, influxDatabase, local)
	}
	if *workRoot == "" {
		glog.Fatal("The --work_root flag is required.")
	}
	if *depotTools == "" {
		glog.Fatal("The --depot_tools flag is required.")
	}
	if err := os.MkdirAll(filepath.Join(*workRoot, "tmp"), 0777); err != nil {
		glog.Fatalf("Failed to create WORK_ROOT/tmp dir: %s", err)
	}
	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://imageinfo.skia.org/oauth2callback/"
	}
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatalf("Failed to initialize the login system: %s", err)
	}
	var err error
	repo, err = gitinfo.CloneOrUpdate(common.REPO_SKIA, filepath.Join(*workRoot, "skia"), true)
	if err != nil {
		glog.Fatalf("Failed to clone Skia: %s", err)
	}
	imageStore, err = store.New()
	if err != nil {
		glog.Fatalf("Failed to connect to Google Storage: %s", err)
	}
	build = buildskia.New(*workRoot, *depotTools, repo, buildVisualize, 3)
	StartBuilding()
	cache = lru.New(NUM_CACHED_RESULT_IMAGES)
	loadTemplates()
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/info", infoHandler)
	r.HandleFunc("/img/{hash:[0-9a-zA-Z]+}", imageHandler)
	r.HandleFunc("/upload", uploadHandler)
	r.HandleFunc("/vis/{id:[0-9a-zA-Z]+}", visHandler)
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
