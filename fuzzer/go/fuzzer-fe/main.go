package main

/*
Runs the frontend portion of the fuzzer.  This primarily is the webserver (see DESIGN.md)
*/

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/frontend"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
	storage "google.golang.org/api/storage/v1"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil
	// detailsTemplate is used for /details, which actually displays the stacktraces and fuzzes.
	detailsTemplate *template.Template = nil

	storageService *storage.Service = nil
)

// Command line flags.
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":80", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")

	authWhiteList = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://fuzzer.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
)

func Init() {
	reloadTemplates()
}

func reloadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	detailsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/details.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func main() {
	defer common.LogPanic()
	// Calls flag.Parse()
	common.InitWithMetrics("fuzzer", graphiteServer)

	Init()

	if err := setupOAuth(); err != nil {
		glog.Fatal(err)
	}

	// TODO(kjlubick): Implement this using Skia Source
	mockLookup := func(packageName string, fileName string, lineNumber int) string {
		return "TODO"
	}

	if err := frontend.LoadFromGoogleStorage(storageService, mockLookup); err != nil {
		glog.Fatalf("Error loading in data from GCS: %v", err)
	}

	runServer()
}

func setupOAuth() error {
	var useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.InitFromMetadataOrJSON(useRedirectURL, login.DEFAULT_SCOPE, *authWhiteList); err != nil {
		return fmt.Errorf("Problem setting up server OAuth: %v", err)
	}

	client, err := auth.NewDefaultClient(true, auth.SCOPE_READ_ONLY)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %v", err)
	}

	storageService, err = storage.New(client)
	if err != nil {
		return fmt.Errorf("Problem authenticating: %v", err)
	}
	return nil
}

func runServer() {
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/details", detailHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/fuzz-list", fuzzListHandler)
	r.HandleFunc("/json/details", detailsHandler)

	rootHandler := login.ForceAuth(util.LoggingGzipRequestResponse(r), OAUTH2_CALLBACK_PATH)

	http.Handle("/", rootHandler)
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func detailHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := detailsTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func fuzzListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mockFuzzes := fuzz.FuzzSummary()

	if err := json.NewEncoder(w).Encode(mockFuzzes); err != nil {
		glog.Errorf("Failed to write or encode output: %v", err)
		return
	}
}

func detailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	i, err := strconv.ParseInt(r.FormValue("line"), 10, 32)
	if err != nil {
		i = -1
	}

	mockFuzz, err := fuzz.FuzzDetails(r.FormValue("file"), r.FormValue("func"), int(i), r.FormValue("fuzz-type") == "binary")
	if err != nil {
		util.ReportError(w, r, err, "There was a problem fulfilling the request.")
		return
	}

	if err := json.NewEncoder(w).Encode(mockFuzz); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}
