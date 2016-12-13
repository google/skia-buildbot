package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/android_ingest/go/continuous"
	"go.skia.org/infra/android_ingest/go/handlers"
	"go.skia.org/infra/android_ingest/go/lookup"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
)

// flags
var (
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	workRoot       = flag.String("work_root", "", "Directory location where all the work is done.")
	repoUrl        = flag.String("repo_url", "", "URL of the git repo where buildids are to be stored.")
	branch         = flag.String("branch", "git_master-skia", "The branch where to look for buildids.")
)

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	defer common.LogPanic()
	if *local {
		common.Init()
	} else {
		common.InitWithMetrics2("androidingest", influxHost, influxUser, influxPassword, influxDatabase, local)
	}
	if *workRoot == "" {
		glog.Fatal("The --work_root flag must be supplied.")
	}
	if *repoUrl == "" {
		glog.Fatal("The --repo_url flag must be supplied.")
	}

	// Create a new auth'd client.
	client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, androidbuildinternal.AndroidbuildInternalScope)
	if err != nil {
		glog.Fatalf("Unable to create authenticated client: %s", err)
	}

	// The repo we're adding commits to.
	checkout, err := git.NewCheckout(*repoUrl, *workRoot)
	if err != nil {
		glog.Fatalf("Unable to create the checkout of %q at %q: %s", *repoUrl, *workRoot, err)
	}
	if err := checkout.Update(); err != nil {
		glog.Fatalf("Unable to update the checkout of %q at %q: %s", *repoUrl, *workRoot, err)
	}

	// checkout isn't go routine safe, but lookup.New() only uses it in New(), so this
	// is safe, i.e. when we later pass checkout to continuous.New().
	lookup, err := lookup.New(checkout)
	if err != nil {
		glog.Fatalf("Failed to create buildid lookup cache: %s", err)
	}

	// Start process that adds buildids to the git repo.
	process, err := continuous.New(*branch, checkout, lookup, client, *local)
	if err != nil {
		glog.Fatalf("Failed to start continuous process of adding new buildids to git repo: %s", err)
	}
	process.Start()

	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		redirectURL = "https://android-ingest.skia.org/oauth2callback/"
	}
	if err := login.InitFromMetadataOrJSON(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		glog.Fatalf("Failed to initialize the login system: %s", err)
	}
	handlers.Init(*resourcesDir, *local, process)
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(makeResourceHandler())
	r.HandleFunc("/upload", handlers.UploadHandler)
	r.HandleFunc("/", handlers.MainHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
