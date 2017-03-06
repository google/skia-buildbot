/*
	Automatic DEPS rolls of Skia into Chrome.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/autoroll/go/autoroller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
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
	childName      = flag.String("childName", "Skia", "Name of the project to roll.")
	childPath      = flag.String("childPath", "src/third_party/skia", "Path within parent repo of the project to roll.")
	cqExtraTrybots = flag.String("cqExtraTrybots", "", "Comma-separated list of trybots to run.")
	depot_tools    = flag.String("depot_tools", "", "Path to the depot_tools installation. If empty, assumes depot_tools is in PATH.")
	doGerrit       = flag.Bool("gerrit", false, "Upload to Gerrit instead of Rietveld.")
	host           = flag.String("host", "localhost", "HTTP service host")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	parentRepo     = flag.String("parent_repo", common.REPO_CHROMIUM, "Repo to roll into.")
	port           = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	sheriff        = flag.String("sheriff", "", "Email address to CC on rolls, or URL from which to obtain such an email address.")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
)

func getSheriff() ([]string, error) {
	emails, err := getSheriffHelper()
	if err != nil {
		return nil, err
	}
	if *doGerrit {
		for i, s := range emails {
			emails[i] = strings.Replace(s, "google.com", "chromium.org", 1)
		}
	}
	return emails, nil
}

func getSheriffHelper() ([]string, error) {
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

func runServer() {
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
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"autoroll",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *local {
		*useMetadata = false
	}
	if *useMetadata {
		// Get .gitcookies from metadata.
		hostname, err := os.Hostname()
		if err != nil {
			sklog.Fatal(err)
		}
		user, err := user.Current()
		if err != nil {
			sklog.Fatal(err)
		}
		gitcookies, err := metadata.ProjectGet(fmt.Sprintf("gitcookies_%s", hostname))
		if err != nil {
			sklog.Fatal(err)
		}
		if err := ioutil.WriteFile(path.Join(user.HomeDir, ".gitcookies"), []byte(gitcookies), 0600); err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the code review API client.
	var r *rietveld.Rietveld
	var g *gerrit.Gerrit
	if *doGerrit {
		g, err = gerrit.NewGerrit(gerrit.GERRIT_CHROMIUM_URL, path.Join(*workdir, ".gitcookies"), nil)
		if err != nil {
			sklog.Fatalf("Failed to create Gerrit client: %s", err)
		}
	} else {
		client, err := auth.NewClientFromIdAndSecret(rietveld.CLIENT_ID, rietveld.CLIENT_SECRET, path.Join(*workdir, "oauth_cache"), rietveld.OAUTH_SCOPES...)
		if err != nil {
			sklog.Fatal(err)
		}
		r = rietveld.New(RIETVELD_URL, client)
	}

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
	arb, err = autoroller.NewAutoRoller(*workdir, *parentRepo, *childPath, cqExtraTrybots, emails, r, g, time.Minute, 15*time.Minute, *depot_tools, *doGerrit)
	if err != nil {
		sklog.Fatal(err)
	}

	// Feed AutoRoll stats into metrics.
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

	login.SimpleInitMust(*port, *local)

	runServer()
}
