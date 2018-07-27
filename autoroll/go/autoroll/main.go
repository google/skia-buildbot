/*
	Automatic DEPS rolls of Skia into Chrome.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"

	"go.skia.org/infra/autoroll/go/google3"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
)

var (
	arb AutoRollerI = nil
	cfg roller.AutoRollerConfig

	mainTemplate *template.Template = nil
)

// flags
var (
	configFile     = flag.String("config_file", "", "Configuration file to use.")
	host           = flag.String("host", "localhost", "HTTP service host")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	recipesCfgFile = flag.String("recipes_cfg", "", "Path to the recipes.cfg file.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
)

// AutoRollerI is the common interface for starting an AutoRoller and handling HTTP requests.
type AutoRollerI interface {
	// Start initiates the AutoRoller's loop.
	Start(ctx context.Context, tickFrequency, repoFrequency time.Duration)
	// AddHandlers allows the AutoRoller to respond to specific HTTP requests.
	AddHandlers(r *mux.Router)
	// SetMode sets the desired mode of the bot.
	SetMode(ctx context.Context, m, user, message string) error
	// SetStrategy sets the desired next-roll-rev strategy.
	SetStrategy(ctx context.Context, strategy, user, message string) error
	// Return the roll-up status of the bot.
	GetStatus(isGoogler bool) *roller.AutoRollStatus
	// Return minimal status information for the bot.
	GetMiniStatus() *roller.AutoRollMiniStatus
	// Forcibly unthrottle the roller.
	Unthrottle() error
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

	if err := arb.SetMode(context.Background(), mode.Mode, login.LoggedInAs(r), mode.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll mode.")
		return
	}

	// Return the ARB status.
	statusJsonHandler(w, r)
}

func strategyJsonHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in with an @google.com account to do that.")
		return
	}

	var strategy struct {
		Message  string `json:"message"`
		Strategy string `json:"strategy"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&strategy); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if err := arb.SetStrategy(context.Background(), strategy.Strategy, login.LoggedInAs(r), strategy.Message); err != nil {
		httputils.ReportError(w, r, err, "Failed to set AutoRoll strategy.")
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
		ParentWaterfall: cfg.ParentWaterfall,
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

func unthrottleHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		httputils.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in with an @google.com account to do that.")
		return
	}
	if err := arb.Unthrottle(); err != nil {
		httputils.ReportError(w, r, err, "Failed to unthrottle.")
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
		ProjectName: cfg.ChildName,
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
	r.HandleFunc("/json/strategy", strategyJsonHandler).Methods("POST")
	r.HandleFunc("/json/unthrottle", unthrottleHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	arb.AddHandlers(r)
	sklog.AddLogsRedirect(r)
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
	skiaversion.MustLogVersion()

	if err := util.WithReadFile(*configFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&cfg)
	}); err != nil {
		sklog.Fatal(err)
	}

	chatbot.Init(fmt.Sprintf("%s -> %s AutoRoller", cfg.ChildName, cfg.ParentName))

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}
	// The rollers use the gitcookie created by gcompute-tools/git-cookie-authdaemon.
	gitcookiesPath := filepath.Join(user.HomeDir, ".git-credential-cache", "cookie")

	androidInternalGerritUrl := cfg.GerritURL
	var emailer *email.GMail
	if *local {
		webhook.InitRequestSaltForTesting()

		// Use the current user's default gitcookies.
		gitcookiesPath = path.Join(user.HomeDir, ".gitcookies")
	} else {

		if err := webhook.InitRequestSaltFromMetadata(metadata.WEBHOOK_REQUEST_SALT); err != nil {
			sklog.Fatal(err)
		}

		// Emailing init.
		emailClientId := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
		emailClientSecret := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
		cachedGMailToken := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CACHED_TOKEN_AUTOROLL))
		tokenFile, err := filepath.Abs(user.HomeDir + "/" + GMAIL_TOKEN_CACHE_FILE)
		if err != nil {
			sklog.Fatal(err)
		}
		if err := ioutil.WriteFile(tokenFile, []byte(cachedGMailToken), os.ModePerm); err != nil {
			sklog.Fatalf("Failed to cache token: %s", err)
		}
		emailer, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
		if err != nil {
			sklog.Fatal(err)
		}

		// If we are rolling into Android get the Gerrit Url from metadata.
		androidInternalGerritUrl, err = metadata.ProjectGet("android_internal_gerrit_url")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	ctx := context.Background()

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// TODO(borenet/rmistry): Create a code review sub-config as described in
	// https://skia-review.googlesource.com/c/buildbot/+/116980/6/autoroll/go/autoroll/main.go#261
	// so that we can get rid of these vars and the various conditionals.
	var g *gerrit.Gerrit
	var githubClient *github.GitHub
	if cfg.RollerType() == roller.ROLLER_TYPE_GOOGLE3 {
		arb, err = google3.NewAutoRoller(ctx, *workdir, common.REPO_SKIA, "master")
	} else {
		if cfg.GerritURL != "" {
			// Create the code review API client.
			if cfg.RollerType() == roller.ROLLER_TYPE_ANDROID {
				cfg.GerritURL = androidInternalGerritUrl
			}
			g, err = gerrit.NewGerrit(cfg.GerritURL, gitcookiesPath, nil)
			if err != nil {
				sklog.Fatalf("Failed to create Gerrit client: %s", err)
			}
			g.TurnOnAuthenticatedGets()
		} else {
			gToken := ""
			if *local {
				gBody, err := ioutil.ReadFile(path.Join(user.HomeDir, github.GITHUB_TOKEN_LOCAL_FILENAME))
				if err != nil {
					sklog.Fatalf("Couldn't find githubToken in the local file %s: %s.", github.GITHUB_TOKEN_LOCAL_FILENAME, err)
				}
				gToken = strings.TrimSpace(string(gBody))
			} else {
				gToken = metadata.Must(metadata.Get(github.GITHUB_TOKEN_METADATA_KEY))
			}
			githubHttpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken}))
			githubClient, err = github.NewGitHub(ctx, cfg.GithubRepoOwner, cfg.GithubRepoName, githubHttpClient)
			if err != nil {
				sklog.Fatalf("Could not create Github client: %s", err)
			}
		}

		if *recipesCfgFile == "" {
			*recipesCfgFile = filepath.Join(*workdir, "recipes.cfg")
		}
		arb, err = roller.NewAutoRoller(ctx, cfg, emailer, g, githubClient, *workdir, *recipesCfgFile, serverURL, gitcookiesPath)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	// Start the roller.
	arb.Start(ctx, time.Minute /* tickFrequency */, 15*time.Minute /* repoFrequency */)

	// Feed AutoRoll stats into metrics.
	cleanup.Repeat(time.Minute, func() {
		status := arb.GetStatus(false)
		v := int64(0)
		if status.LastRoll != nil && status.LastRoll.Closed && status.LastRoll.Committed {
			v = int64(1)
		}
		metrics2.GetInt64Metric("autoroll_last_roll_result", map[string]string{"roller": cfg.RollerName()}).Update(v)
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
