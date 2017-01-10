/*
	Automatic DEPS rolls of Skia into Chrome.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/autoroll/go/autoroller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

const (
	RIETVELD_URL = "https://codereview.chromium.org"
)

var (
	arb *autoroller.AutoRoller = nil

	mainTemplate *template.Template = nil
)

// flags
var (
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	childName      = flag.String("childName", "Skia", "Name of the project to roll.")
	childPath      = flag.String("childPath", "src/third_party/skia", "Path within Chromium repo of the project to roll.")
	cqExtraTrybots = flag.String("cqExtraTrybots", "", "Comma-separated list of trybots to run.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	sheriff        = flag.String("sheriff", "", "Email address to CC on rolls, or URL from which to obtain such an email address.")
	depot_tools    = flag.String("depot_tools", "", "Path to the depot_tools installation. If empty, assumes depot_tools is in PATH.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func getSheriff() ([]string, error) {
	// If the passed-in sheriff doesn't look like a URL, it's probably an
	// email address. Use it directly.
	if _, err := url.ParseRequestURI(*sheriff); err != nil {
		if strings.Count(*sheriff, "@") == 1 {
			return []string{*sheriff}, nil
		} else {
			return nil, fmt.Errorf("Sheriff must be an email address or a valid URL; %q doesn't look like either.", *sheriff)
		}
	}

	// Hit the URL to get the email address. Expect JSON.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(*sheriff)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	var sheriff struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sheriff); err != nil {
		return nil, err
	}
	return []string{sheriff.Username}, nil
}

func getCQExtraTrybots() string {
	return *cqExtraTrybots
}

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/main.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	reloadTemplates()
}

func modeJsonHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in with an @google.com account to do that.")
		return
	}

	var mode struct {
		Mode string `json:"mode"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&mode); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if err := arb.SetMode(mode.Mode, login.LoggedInAs(r), "[Placeholder Message]"); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll mode.")
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func statusJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Obtain the status info. Only display error messages if the user
	// is a logged-in Googler.
	status := arb.GetStatus(login.IsGoogler(r))
	if err := json.NewEncoder(w).Encode(&status); err != nil {
		sklog.Error(err)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	mainPage := struct {
		ProjectName string
		ProjectUser string
	}{
		ProjectName: *childName,
		ProjectUser: arb.User(),
	}
	if err := mainTemplate.Execute(w, mainPage); err != nil {
		sklog.Errorln("Failed to expand template:", err)
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/json/mode", modeJsonHandler).Methods("POST")
	r.HandleFunc("/json/status", httputils.CorsHandler(statusJsonHandler))
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("autoroll", influxHost, influxUser, influxPassword, influxDatabase, local)
	Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *local {
		*useMetadata = false
	}

	// Create the Rietveld client.
	client, err := auth.NewClientFromIdAndSecret(rietveld.CLIENT_ID, rietveld.CLIENT_SECRET, path.Join(*workdir, "oauth_cache"), rietveld.OAUTH_SCOPES...)
	if err != nil {
		sklog.Fatal(err)
	}
	r := rietveld.New(RIETVELD_URL, client)

	// Retrieve the list of extra CQ trybots.
	// TODO(borenet): Make this editable on the web front-end.
	cqExtraTrybots := getCQExtraTrybots()
	sklog.Infof("CQ extra trybots: %s", cqExtraTrybots)

	// Retrieve the initial email list.
	emails, err := getSheriff()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Sheriff: %s", strings.Join(emails, ", "))

	// Start the autoroller.
	arb, err = autoroller.NewAutoRoller(*workdir, *childPath, cqExtraTrybots, emails, r, time.Minute, 15*time.Minute, *depot_tools)
	if err != nil {
		sklog.Fatal(err)
	}

	// Feed AutoRoll stats into InfluxDB.
	go func() {
		for _ = range time.Tick(time.Minute) {
			status := arb.GetStatus(false)
			v := int64(0)
			if status.LastRoll != nil && status.LastRoll.Closed && status.LastRoll.Committed {
				v = int64(1)
			}
			metrics2.GetInt64Metric("autoroll.last-roll-result", map[string]string{"child-path": *childPath}).Update(v)
		}
	}()

	// Update the current sheriff in a loop.
	go func() {
		for _ = range time.Tick(30 * time.Minute) {
			emails, err := getSheriff()
			if err != nil {
				sklog.Errorf("Failed to retrieve current sheriff: %s", err)
			} else {
				arb.SetEmails(emails)
			}
		}
	}()

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	redirectURL := serverURL + "/oauth2callback/"

	if err := login.Init(redirectURL, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	runServer(serverURL)
}
