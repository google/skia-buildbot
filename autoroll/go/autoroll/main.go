/*
	Automatic DEPS rolls of Skia into Chrome.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/autoroll/go/autoroller"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

var (
	arb *autoroller.AutoRoller = nil

	mainTemplate *template.Template = nil
)

// flags
var (
	childName       = flag.String("childName", "Skia", "Name of the project to roll.")
	childPath       = flag.String("childPath", "src/third_party/skia", "Path within parent repo of the project to roll.")
	childBranch     = flag.String("child_branch", "master", "Branch of the project we want to roll.")
	cqExtraTrybots  = flag.String("cqExtraTrybots", "", "Comma-separated list of trybots to run.")
	host            = flag.String("host", "localhost", "HTTP service host")
	local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	parentRepo      = flag.String("parent_repo", common.REPO_CHROMIUM, "Repo to roll into.")
	parentBranch    = flag.String("parent_branch", "master", "Branch of the parent repo we want to roll into.")
	port            = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir    = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	sheriff         = flag.String("sheriff", "", "Email address to CC on rolls, or URL from which to obtain such an email address.")
	strategy        = flag.String("strategy", repo_manager.ROLL_STRATEGY_BATCH, "DEPS roll strategy; how many commits should be rolled at once.")
	useMetadata     = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	workdir         = flag.String("workdir", ".", "Directory to use for scratch work.")
	rollIntoAndroid = flag.Bool("roll_into_android", false, "Roll into Android; do not do a DEPS roll.")
	gerritUrl       = flag.String("gerrit_url", gerrit.GERRIT_CHROMIUM_URL, "Gerrit URL the roller will be uploading issues to.")
)

func getSheriff() ([]string, error) {
	emails, err := getSheriffHelper()
	if err != nil {
		return nil, err
	}
	if strings.Contains(*parentRepo, "chromium") {
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
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&mode); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if err := arb.SetMode(mode.Mode, login.LoggedInAs(r), mode.Message); err != nil {
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

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}
	gitcookiesPath := path.Join(user.HomeDir, ".gitcookies")
	androidInternalGerritUrl := *gerritUrl

	if *useMetadata {
		// If we are rolling into Android get the Gerrit Url from metadata.
		androidInternalGerritUrl, err = metadata.ProjectGet("android_internal_gerrit_url")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the code review API client.
	gUrl := *gerritUrl
	if *rollIntoAndroid {
		if !*local {
			// Android roller uses the gitcookie created by gcompute-tools/git-cookie-authdaemon.
			// TODO(rmistry): Turn this on via the GCE setup script so that it exists right when the instance comes up?
			gitcookiesPath = filepath.Join(user.HomeDir, ".git-credential-cache", "cookie")
		}
		gUrl = androidInternalGerritUrl
	} else {
		if strings.Contains(*parentRepo, "skia") {
			gUrl = gerrit.GERRIT_SKIA_URL
		}
	}
	g, err := gerrit.NewGerrit(gUrl, gitcookiesPath, nil)
	if err != nil {
		sklog.Fatalf("Failed to create Gerrit client: %s", err)
	}
	g.TurnOnAuthenticatedGets()

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

	// Sync depot_tools.
	var depotTools string
	if !*rollIntoAndroid {
		depotTools, err = depot_tools.Sync(*workdir)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Start the autoroller.
	arb, err = autoroller.NewAutoRoller(*workdir, *parentRepo, *parentBranch, *childPath, *childBranch, cqExtraTrybots, emails, g, depotTools, *rollIntoAndroid, *strategy)
	if err != nil {

		sklog.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	arb.Start(time.Minute /* tickFrequency */, 15*time.Minute /* repoFrequency */, ctx)

	// Feed AutoRoll stats into metrics.
	go func() {
		for range time.Tick(time.Minute) {
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
		for range time.Tick(30 * time.Minute) {
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
