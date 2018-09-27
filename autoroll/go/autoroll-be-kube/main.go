package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
	GS_BUCKET_AUTOROLLERS  = "skia-autoroll"
)

// flags
var (
	configFile     = flag.String("config_file", "", "Configuration file to use.")
	emailCreds     = flag.String("email_creds", "", "Directory containing credentials for sending emails.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service port.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	recipesCfgFile = flag.String("recipes_cfg", "", "Path to the recipes.cfg file.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	hang           = flag.Bool("hang", false, "If true, just hang and do nothing.")
)

// AutoRollerI is the common interface for starting an AutoRoller and handling HTTP requests.
type AutoRollerI interface {
	// Start initiates the AutoRoller's loop.
	Start(ctx context.Context, tickFrequency, repoFrequency time.Duration)
	// AddHandlers allows the AutoRoller to respond to specific HTTP requests.
	AddHandlers(r *mux.Router)
}

func main() {
	common.InitWithMust(
		"autoroll-be",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()
	if *hang {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
	}

	skiaversion.MustLogVersion()

	// Rollers use a custom temporary dir, to ensure that it's on a
	// persistent disk. Create it if it does not exist.
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		if err := os.Mkdir(os.TempDir(), os.ModePerm); err != nil {
			sklog.Fatalf("Failed to create %s: %s", os.TempDir(), err)
		}
	}

	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*configFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&cfg)
	}); err != nil {
		sklog.Fatal(err)
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	client := auth.ClientFromTokenSource(ts)
	namespace := ds.AUTOROLL_NS
	if cfg.IsInternal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	gcsBucket := GS_BUCKET_AUTOROLLERS
	rollerName := cfg.RollerName
	if *local {
		gcsBucket = gcs.TEST_DATA_BUCKET
		hostname, err := os.Hostname()
		if err != nil {
			sklog.Fatalf("Could not get hostname: %s", err)
		}
		rollerName = fmt.Sprintf("autoroll_%s", hostname)
	}

	chatbot.Init(fmt.Sprintf("%s -> %s AutoRoller", cfg.ChildName, cfg.ParentName))

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}

	var emailer *email.GMail
	if !*local {
		// Emailing init.
		emailClientId, err := ioutil.ReadFile(path.Join(*emailCreds, metadata.GMAIL_CLIENT_ID))
		if err != nil {
			sklog.Fatal(err)
		}
		emailClientSecret, err := ioutil.ReadFile(path.Join(*emailCreds, metadata.GMAIL_CLIENT_SECRET))
		if err != nil {
			sklog.Fatal(err)
		}
		cachedGMailToken, err := ioutil.ReadFile(path.Join(*emailCreds, metadata.GMAIL_CACHED_TOKEN_AUTOROLL))
		if err != nil {
			sklog.Fatal(err)
		}
		tokenFile, err := filepath.Abs(user.HomeDir + "/" + GMAIL_TOKEN_CACHE_FILE)
		if err != nil {
			sklog.Fatal(err)
		}
		if err := ioutil.WriteFile(tokenFile, cachedGMailToken, os.ModePerm); err != nil {
			sklog.Fatalf("Failed to cache token: %s", err)
		}
		emailer, err = email.NewGMail(strings.TrimSpace(string(emailClientId)), strings.TrimSpace(string(emailClientSecret)), tokenFile)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	serverURL := roller.AUTOROLL_URL_PUBLIC + "/r/" + cfg.RollerName
	if cfg.IsInternal {
		serverURL = roller.AUTOROLL_URL_PRIVATE + "/r/" + cfg.RollerName
	}

	ctx := context.Background()

	// TODO(borenet/rmistry): Create a code review sub-config as described in
	// https://skia-review.googlesource.com/c/buildbot/+/116980/6/autoroll/go/autoroll/main.go#261
	// so that we can get rid of these vars and the various conditionals.
	var g *gerrit.Gerrit
	var githubClient *github.GitHub
	s, err := storage.NewClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Writing persistent data to gs://%s/%s", gcsBucket, rollerName)
	gcsClient := gcs.NewGCSClient(s, gcsBucket)

	// The rollers use the gitcookie created by gitauth package.
	gitcookiesPath := filepath.Join(user.HomeDir, ".gitcookies")
	if _, err := gitauth.New(ts, gitcookiesPath, true, cfg.ServiceAccount); err != nil {
		sklog.Fatalf("Failed to create git cookie updater: %s", err)
	}

	if cfg.GerritURL != "" {
		// Create the code review API client.
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

	// Set environment variable for depot_tools.
	if err := os.Setenv("SKIP_GCE_AUTH_FOR_GIT", "1"); err != nil {
		sklog.Fatal(err)
	}

	arb, err := roller.NewAutoRoller(ctx, cfg, emailer, g, githubClient, *workdir, *recipesCfgFile, serverURL, gitcookiesPath, gcsClient, client, rollerName)
	if err != nil {
		sklog.Fatal(err)
	}

	// Start the roller.
	arb.Start(ctx, time.Minute /* tickFrequency */, 15*time.Minute /* repoFrequency */)

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
	httputils.RunHealthCheckServer(*port)
}
