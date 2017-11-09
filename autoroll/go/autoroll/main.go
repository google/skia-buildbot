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
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/webhook"

	"go.skia.org/infra/autoroll/go/google3"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

var (
	arb AutoRollerI = nil

	mainTemplate *template.Template = nil
)

// flags
var (
	childName       = flag.String("childName", "Skia", "Name of the project to roll.")
	childPath       = flag.String("childPath", "src/third_party/skia", "Path within parent repo of the project to roll.")
	childBranch     = flag.String("child_branch", "master", "Branch of the project we want to roll.")
	cqExtraTrybots  = flag.String("cqExtraTrybots", "", "Comma-separated list of trybots to run.")
	depsCustomVars  = common.NewMultiStringFlag("deps_custom_var", nil, "Custom vars to pass to gclient, in the form \"key=value\"")
	gerritUrl       = flag.String("gerrit_url", gerrit.GERRIT_CHROMIUM_URL, "Gerrit URL the roller will be uploading issues to.")
	host            = flag.String("host", "localhost", "HTTP service host")
	local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	noLog           = flag.Bool("no_log", false, "If true, roll CLs do not include a git log (DEPS rollers only).")
	parentRepo      = flag.String("parent_repo", common.REPO_CHROMIUM, "Repo to roll into.")
	parentBranch    = flag.String("parent_branch", "master", "Branch of the parent repo we want to roll into.")
	parentWaterfall = flag.String("parent_waterfall", "", "Waterfall URL of the parent repo.")
	port            = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	preUploadSteps  = common.NewMultiStringFlag("pre_upload_step", nil, "Named steps to run before uploading roll CLs. Pre-upload steps and their names are available in https://skia.googlesource.com/buildbot/+/master/autoroll/go/repo_manager/pre_upload_steps.go")
	promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir    = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	rollIntoAndroid = flag.Bool("roll_into_android", false, "Roll into Android; do not do a DEPS/Manifest roll.")
	rollIntoGoogle3 = flag.Bool("roll_into_google3", false, "Roll into Google3; do not do a Gerrit roll.")
	sheriff         = flag.String("sheriff", "", "Email address to CC on rolls, or URL from which to obtain such an email address.")
	strategy        = flag.String("strategy", repo_manager.ROLL_STRATEGY_BATCH, "DEPS roll strategy; how many commits should be rolled at once.")
	throttleCount   = flag.Int64("throttle_count", 0, "Maximum number of attempts before throttling.")
	throttleTime    = flag.String("throttle_time", "", "Time window for throttle attempts, eg. \"30m\" or \"1h10m\"")
	useManifest     = flag.Bool("use_manifest", false, "Do a Manifest roll.")
	useMetadata     = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	workdir         = flag.String("workdir", ".", "Directory to use for scratch work.")
)

// AutoRollerI is the common interface for starting an AutoRoller and handling HTTP requests.
type AutoRollerI interface {
	// Start initiates the AutoRoller's loop.
	Start(tickFrequency, repoFrequency time.Duration)
	// AddHandlers allows the AutoRoller to respond to specific HTTP requests.
	AddHandlers(r *mux.Router)
	// SetMode sets the desired mode of the bot. This forces the bot to run and
	// blocks until it finishes.
	SetMode(m, user, message string) error
	// SetEmails sets the list of email addresses which are copied on rolls.
	SetEmails(e []string)
	// Return the roll-up status of the bot.
	GetStatus(isGoogler bool) *roller.AutoRollStatus
	// Return minimal status information for the bot.
	GetMiniStatus() *roller.AutoRollMiniStatus
}

// Update the current sheriff list.
func getSheriff() ([]string, error) {
	emails, err := getSheriffHelper()
	if err != nil {
		return nil, err
	}
	// TODO(borenet): Do we need this any more?
	if strings.Contains(*parentRepo, "chromium") && *childName != "WebRTC" {
		for i, s := range emails {
			emails[i] = strings.Replace(s, "google.com", "chromium.org", 1)
		}
	}
	return emails, nil
}

// Parse the sheriff list from JS. Expects the list in this format:
// document.write('somebody, somebodyelse')
// TODO(borenet): Remove this once Chromium has a proper sheriff endpoint, ie.
// https://bugs.chromium.org/p/chromium/issues/detail?id=769804
func getSheriffJS(js string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(js), "document.write('"), "')")
	list := strings.Split(trimmed, ",")
	rv := make([]string, 0, len(list))
	for _, name := range list {
		name = strings.TrimSpace(name)
		if name != "" {
			rv = append(rv, name+"@chromium.org")
		}
	}
	return rv
}

// Helper for loading the sheriff list.
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

	// Hit the URL to get the email address. Expect JSON or a JS file which
	// document.writes the Sheriff(s) in a comma-separated list.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(*sheriff)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if strings.HasSuffix(*sheriff, ".js") {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return getSheriffJS(string(body)), nil
	} else {
		var sheriff struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&sheriff); err != nil {
			return nil, err
		}
		return []string{sheriff.Username}, nil
	}
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
	// Obtain the status info. Only display potentially sensitive info if the user is a logged-in
	// Googler.
	type status struct {
		*roller.AutoRollStatus
		ParentWaterfall string `json:"parentWaterfall"`
	}
	st := status{
		AutoRollStatus:  arb.GetStatus(login.IsGoogler(r)),
		ParentWaterfall: *parentWaterfall,
	}
	if err := json.NewEncoder(w).Encode(&st); err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain status.")
		return
	}
}

func miniStatusJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := arb.GetMiniStatus()
	if err := json.NewEncoder(w).Encode(&status); err != nil {
		httputils.ReportError(w, r, err, "Failed to obtain status.")
		return
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
	}{
		ProjectName: *childName,
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
	r.HandleFunc("/json/ministatus", httputils.CorsHandler(miniStatusJsonHandler))
	r.HandleFunc("/json/status", httputils.CorsHandler(statusJsonHandler))
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	arb.AddHandlers(r)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"autoroll",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	defer common.Defer()

	Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *rollIntoGoogle3 {
		if *cqExtraTrybots != "" {
			sklog.Fatalf("Can not specify --cqExtraTrybots with --roll_into_google3.")
		}
		if *parentBranch != "" {
			sklog.Fatalf("Can not specify --parent_branch with --roll_into_google3.")
		}
		if *strategy != "" && *strategy != repo_manager.ROLL_STRATEGY_BATCH {
			sklog.Fatalf("Can not specify --strategy with --roll_into_google3.")
		}
		if *rollIntoAndroid {
			sklog.Fatalf("Can not specify --roll_into_android with --roll_into_google3.")
		}
		if *useManifest {
			sklog.Fatalf("Can not specify --use_manifest with --roll_into_google3.")
		}
		if *gerritUrl != "" {
			sklog.Fatalf("Can not specify --gerrit_url with --roll_into_google3.")
		}
		if len(*preUploadSteps) != 0 {
			sklog.Fatalf("Can not specify --pre_upload_step with --roll_into_google3.")
		}
	}

	if *local {
		*useMetadata = false
		webhook.InitRequestSaltForTesting()
	}

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}
	gitcookiesPath := path.Join(user.HomeDir, ".gitcookies")
	androidInternalGerritUrl := *gerritUrl

	if *useMetadata {
		if err := webhook.InitRequestSaltFromMetadata(); err != nil {
			sklog.Fatal(err)
		}

		// If we are rolling into Android get the Gerrit Url from metadata.
		androidInternalGerritUrl, err = metadata.ProjectGet("android_internal_gerrit_url")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the code review API client.
	var g *gerrit.Gerrit
	if *gerritUrl != "" {
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
		var err error
		g, err = gerrit.NewGerrit(gUrl, gitcookiesPath, nil)
		if err != nil {
			sklog.Fatalf("Failed to create Gerrit client: %s", err)
		}
		g.TurnOnAuthenticatedGets()
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

	// Sync depot_tools.
	var depotTools string
	if !*rollIntoAndroid && !*rollIntoGoogle3 {
		depotTools, err = depot_tools.Sync(*workdir)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// Create the autoroller.
	strat, err := repo_manager.GetNextRollStrategy(*strategy, *childBranch, "")
	if err != nil {
		sklog.Fatal(err)
	}
	var tc *state_machine.ThrottleConfig
	if *throttleTime != "" && *throttleCount != 0 {
		parsed, err := human.ParseDuration(*throttleTime)
		if err != nil {
			sklog.Fatal(err)
		}
		tc = &state_machine.ThrottleConfig{
			AttemptCount: *throttleCount,
			TimeWindow:   parsed,
		}
	}
	if *rollIntoAndroid {
		arb, err = roller.NewAndroidAutoRoller(*workdir, *parentBranch, *childPath, *childBranch, cqExtraTrybots, emails, g, repo_manager.StrategyRemoteHead(*childBranch), *preUploadSteps, serverURL, tc)
	} else if *rollIntoGoogle3 {
		arb, err = google3.NewAutoRoller(*workdir, common.REPO_SKIA, *childBranch)
	} else if *useManifest {
		arb, err = roller.NewManifestAutoRoller(*workdir, *parentRepo, *parentBranch, *childPath, *childBranch, cqExtraTrybots, emails, g, depotTools, strat, *preUploadSteps, serverURL, tc)
	} else {
		arb, err = roller.NewDEPSAutoRoller(*workdir, *parentRepo, *parentBranch, *childPath, *childBranch, cqExtraTrybots, emails, g, depotTools, strat, *preUploadSteps, !*noLog, *depsCustomVars, serverURL, tc)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	// Start the roller.
	arb.Start(time.Minute /* tickFrequency */, 15*time.Minute /* repoFrequency */)

	// Feed AutoRoll stats into metrics.
	cleanup.Repeat(time.Minute, func() {
		status := arb.GetStatus(false)
		v := int64(0)
		if status.LastRoll != nil && status.LastRoll.Closed && status.LastRoll.Committed {
			v = int64(1)
		}
		metrics2.GetInt64Metric("autoroll_last_roll_result", map[string]string{"child_path": *childPath}).Update(v)
	}, nil)

	// Update the current sheriff in a loop.
	cleanup.Repeat(30*time.Minute, func() {
		emails, err := getSheriff()
		if err != nil {
			sklog.Errorf("Failed to retrieve current sheriff: %s", err)
		} else {
			arb.SetEmails(emails)
		}
	}, nil)

	if g != nil {
		// Periodically delete old roll CLs.
		// "git cl upload" performs some steps after the actual upload of the
		// CL. When these steps fail, all we know is that the command failed,
		// and since we didn't get an issue number back we have to assume that
		// no CL was uploaded. This can leave us with orphaned roll CLs.
		myEmail, err := g.GetUserEmail()
		if err != nil {
			sklog.Fatal(err)
		}
		go func() {
			for range time.Tick(60 * time.Minute) {
				issues, err := g.Search(100, gerrit.SearchOwner(myEmail), gerrit.SearchStatus(gerrit.CHANGE_STATUS_DRAFT))
				if err != nil {
					sklog.Errorf("Failed to retrieve autoroller issues: %s", err)
					continue
				}
				issues2, err := g.Search(100, gerrit.SearchOwner(myEmail), gerrit.SearchStatus(gerrit.CHANGE_STATUS_NEW))
				if err != nil {
					sklog.Errorf("Failed to retrieve autoroller issues: %s", err)
					continue
				}
				issues = append(issues, issues2...)
				for _, ci := range issues {
					if ci.Updated.Before(time.Now().Add(-168 * time.Hour)) {
						if err := g.Abandon(ci, "Abandoning new/draft issues older than a week."); err != nil {
							sklog.Errorf("Failed to abandon old issue %s: %s", g.Url(ci.Issue), err)
						}
					}
				}
			}
		}()
	}

	login.SimpleInitMust(*port, *local)

	runServer(serverURL)
}
