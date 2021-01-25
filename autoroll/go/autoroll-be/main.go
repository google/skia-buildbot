package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
	GS_BUCKET_AUTOROLLERS  = "skia-autoroll"
)

// flags
var (
	chatWebHooksFile  = flag.String("chat_webhooks_file", "", "Chat webhook config.")
	configContents    = flag.String("config", "", "Base 64 encoded configuration in JSON format, mutually exclusive with --config_file.")
	configFile        = flag.String("config_file", "", "Configuration file to use, mutually exclusive with --config.")
	emailCreds        = flag.String("email_creds", "", "Directory containing credentials for sending emails.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service port.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	recipesCfgFile    = flag.String("recipes_cfg", "", "Path to the recipes.cfg file.")
	workdir           = flag.String("workdir", ".", "Directory to use for scratch work.")
	hang              = flag.Bool("hang", false, "If true, just hang and do nothing.")
)

// AutoRollerI is the common interface for starting an AutoRoller and handling HTTP requests.
type AutoRollerI interface {
	// Start initiates the AutoRoller's loop.
	Start(ctx context.Context, tickFrequency time.Duration)
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

	// Rollers use a custom temporary dir, to ensure that it's on a
	// persistent disk. Create it if it does not exist.
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		if err := os.Mkdir(os.TempDir(), os.ModePerm); err != nil {
			sklog.Fatalf("Failed to create %s: %s", os.TempDir(), err)
		}
	}

	// Decode the config.
	if (*configContents == "" && *configFile == "") || (*configContents != "" && *configFile != "") {
		sklog.Fatal("Exactly one of --config or --config_file is required.")
	}
	var configBytes []byte
	var err error
	if *configContents != "" {
		configBytes, err = base64.StdEncoding.DecodeString(*configContents)
	} else {
		err = util.WithReadFile(*configFile, func(f io.Reader) error {
			configBytes, err = ioutil.ReadAll(f)
			return err
		})
	}
	if err != nil {
		sklog.Fatal(err)
	}
	var cfgProto config.Config
	if err := prototext.Unmarshal(configBytes, &cfgProto); err != nil {
		sklog.Fatal(err)
	}
	// TODO(borenet): Remove the old-style config and just use the proto
	// version everywhere.
	cfg, err := roller.ProtoToConfig(&cfgProto)
	if err != nil {
		sklog.Fatal(err)
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	namespace := ds.AUTOROLL_NS
	if cfg.IsInternal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	chatbot.Init(fmt.Sprintf("%s -> %s AutoRoller", cfg.ChildDisplayName, cfg.ParentDisplayName))

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()

	var emailer *email.GMail
	var chatBotConfigReader chatbot.ConfigReader
	var gcsClient gcs.GCSClient
	rollerName := cfg.RollerName
	if *local {
		hostname, err := os.Hostname()
		if err != nil {
			sklog.Fatalf("Could not get hostname: %s", err)
		}
		rollerName = fmt.Sprintf("autoroll_%s", hostname)
		gcsClient = mem_gcsclient.New("fake-bucket")
	} else {
		s, err := storage.NewClient(ctx)
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("Writing persistent data to gs://%s/%s", GS_BUCKET_AUTOROLLERS, rollerName)
		gcsClient = gcsclient.New(s, GS_BUCKET_AUTOROLLERS)

		// Emailing init.
		emailClientId, err := ioutil.ReadFile(filepath.Join(*emailCreds, metadata.GMAIL_CLIENT_ID))
		if err != nil {
			sklog.Fatal(err)
		}
		emailClientSecret, err := ioutil.ReadFile(filepath.Join(*emailCreds, metadata.GMAIL_CLIENT_SECRET))
		if err != nil {
			sklog.Fatal(err)
		}
		cachedGMailToken, err := ioutil.ReadFile(filepath.Join(*emailCreds, metadata.GMAIL_CACHED_TOKEN_AUTOROLL))
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
		chatBotConfigReader = func() string {
			if b, err := ioutil.ReadFile(*chatWebHooksFile); err != nil {
				sklog.Errorf("Failed to read chat config %q: %s", *chatWebHooksFile, err)
				return ""
			} else {
				return string(b)
			}
		}
	}

	serverURL := roller.AUTOROLL_URL_PUBLIC + "/r/" + cfg.RollerName
	if cfg.IsInternal {
		serverURL = roller.AUTOROLL_URL_PRIVATE + "/r/" + cfg.RollerName
	}

	// TODO(borenet/rmistry): Create a code review sub-config as described in
	// https://skia-review.googlesource.com/c/buildbot/+/116980/6/autoroll/go/autoroll/main.go#261
	// so that we can get rid of these vars and the various conditionals.
	var g *gerrit.Gerrit
	var githubClient *github.GitHub

	// The rollers use the gitcookie created by gitauth package.
	if !*local {
		gitcookiesPath := filepath.Join(user.HomeDir, ".gitcookies")
		if _, err := gitauth.New(ts, gitcookiesPath, true, cfg.ServiceAccount); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
	}

	if cfg.Gerrit != nil {
		// Create the code review API client.
		gc, err := cfg.Gerrit.GetConfig()
		if err != nil {
			sklog.Fatalf("Failed to get Gerrit config: %s", err)
		}
		g, err = gerrit.NewGerritWithConfig(gc, cfg.Gerrit.URL, client)
		if err != nil {
			sklog.Fatalf("Failed to create Gerrit client: %s", err)
		}
	} else if cfg.Github != nil {
		pathToGithubToken := filepath.Join(user.HomeDir, github.GITHUB_TOKEN_FILENAME)
		if !*local {
			pathToGithubToken = filepath.Join(github.GITHUB_TOKEN_SERVER_PATH, github.GITHUB_TOKEN_FILENAME)
			// Setup the required SSH key from secrets if we are not running
			// locally and if the file does not already exist.
			sshKeySrc := filepath.Join(github.SSH_KEY_SERVER_PATH, github.SSH_KEY_FILENAME)
			sshKeyDestDir := filepath.Join(user.HomeDir, ".ssh")
			sshKeyDest := filepath.Join(sshKeyDestDir, github.SSH_KEY_FILENAME)
			if _, err := os.Stat(sshKeyDest); os.IsNotExist(err) {
				b, err := ioutil.ReadFile(sshKeySrc)
				if err != nil {
					sklog.Fatalf("Could not read from %s: %s", sshKeySrc, err)
				}
				if _, err := fileutil.EnsureDirExists(sshKeyDestDir); err != nil {
					sklog.Fatalf("Could not create %s: %s", sshKeyDest, err)
				}
				if err := ioutil.WriteFile(sshKeyDest, b, 0600); err != nil {
					sklog.Fatalf("Could not write to %s: %s", sshKeyDest, err)
				}
			}
			// Make sure github is added to known_hosts.
			github.AddToKnownHosts(ctx)
		}
		// Instantiate githubClient using the github token secret.
		gBody, err := ioutil.ReadFile(pathToGithubToken)
		if err != nil {
			sklog.Fatalf("Couldn't find githubToken in %s: %s.", pathToGithubToken, err)
		}
		gToken := strings.TrimSpace(string(gBody))
		githubHttpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken}))
		githubClient, err = github.NewGitHub(ctx, cfg.Github.RepoOwner, cfg.Github.RepoName, githubHttpClient)
		if err != nil {
			sklog.Fatalf("Could not create Github client: %s", err)
		}
	}

	sklog.Info("Creating manual roll DB.")
	manualRolls, err := manual.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create manual roll DB: %s", err)
	}

	if *recipesCfgFile == "" {
		*recipesCfgFile = filepath.Join(*workdir, "recipes.cfg")
	}

	// Set environment variable for depot_tools.
	if err := os.Setenv("SKIP_GCE_AUTH_FOR_GIT", "1"); err != nil {
		sklog.Fatal(err)
	}

	arb, err := roller.NewAutoRoller(ctx, *cfg, emailer, chatBotConfigReader, g, githubClient, *workdir, *recipesCfgFile, serverURL, gcsClient, client, rollerName, *local, manualRolls)
	if err != nil {
		sklog.Fatal(err)
	}

	// Start the roller.
	arb.Start(ctx, time.Minute /* tickFrequency */)

	if g != nil {
		// Periodically delete old roll CLs.
		// "git cl upload" performs some steps after the actual upload of the
		// CL. When these steps fail, all we know is that the command failed,
		// and since we didn't get an issue number back we have to assume that
		// no CL was uploaded. This can leave us with orphaned roll CLs.
		myEmail, err := g.GetUserEmail(ctx)
		if err != nil {
			sklog.Fatal(err)
		}
		go func() {
			for range time.Tick(60 * time.Minute) {
				issues, err := g.Search(ctx, 100, true, gerrit.SearchOwner(myEmail), gerrit.SearchStatus(gerrit.CHANGE_STATUS_DRAFT))
				if err != nil {
					sklog.Errorf("Failed to retrieve autoroller issues: %s", err)
					continue
				}
				issues2, err := g.Search(ctx, 100, true, gerrit.SearchOwner(myEmail), gerrit.SearchStatus(gerrit.CHANGE_STATUS_NEW))
				if err != nil {
					sklog.Errorf("Failed to retrieve autoroller issues: %s", err)
					continue
				}
				issues = append(issues, issues2...)
				for _, ci := range issues {
					if ci.Updated.Before(time.Now().Add(-168 * time.Hour)) {
						if err := g.Abandon(ctx, ci, "Abandoning new/draft issues older than a week."); err != nil {
							sklog.Errorf("Failed to abandon old issue %s: %s", g.Url(ci.Issue), err)
						}
					}
				}
			}
		}()
	} else if githubClient != nil {
		var forkRepoURL string
		if cfg.GithubDEPSRepoManager != nil {
			forkRepoURL = cfg.GithubDEPSRepoManager.ForkRepoURL
		} else if cfg.GithubRepoManager != nil {
			forkRepoURL = cfg.GithubRepoManager.ForkRepoURL
		}
		if forkRepoURL != "" {
			// Periodically delete old fork branches for this roller.
			// Github rollers create new fork branches for each roll (skbug.com/10328). Branches from
			// merged PRs should be cleaned up via
			// https://help.github.com/en/github/administering-a-repository/managing-the-automatic-deletion-of-branches
			// But that does not address failed and abandoned PRs.
			reForkBranchWithTimestamp := regexp.MustCompile(`^.*?-(\d+)$`)
			go func() {
				for range time.Tick(60 * time.Minute) {
					sklog.Infof("Finding all fork branches that start with the rollers name %s", rollerName)
					forkRepoMatches := parent.REForkRepoURL.FindStringSubmatch(forkRepoURL)
					forkRepoOwner := forkRepoMatches[2]
					forkRepoName := forkRepoMatches[3]
					refs, err := githubClient.ListMatchingReferences(forkRepoOwner, forkRepoName, fmt.Sprintf("refs/heads/%s-", rollerName))
					if err != nil {
						sklog.Errorf("Failed to retrieve matching references for %s: %s", rollerName, err)
						continue
					}
					sklog.Infof("Found matching references for %s: %s", rollerName, refs)

					// Fork branches have the creation timestamp in their names. Use this to find
					// branches older than a week and delete them. We do it this way because there are no
					// timestamps returned for refs in the github API.
					for _, r := range refs {
						forkBranchNameMatches := reForkBranchWithTimestamp.FindStringSubmatch(*r.Ref)
						if len(forkBranchNameMatches) != 2 {
							sklog.Infof("Fork branch %s is not in expected format %s. Skipping it.", *r.Ref, reForkBranchWithTimestamp)
							continue
						}
						creationTS, err := strconv.ParseInt(forkBranchNameMatches[1], 10, 64)
						if err != nil {
							sklog.Errorf("Could not read timestamp from fork branch %s: %s", *r.Ref, err)
							continue
						}
						creationTime := time.Unix(creationTS, 0)
						elapsedDuration := time.Now().Sub(creationTime)
						elapsedDays := elapsedDuration.Hours() / 24
						sklog.Infof("Fork branch %s was created %f days ago", *r.Ref, elapsedDays)
						if elapsedDays > 7 {
							if err := githubClient.DeleteReference(forkRepoOwner, forkRepoName, *r.Ref); err != nil {
								sklog.Errorf("Could not delete fork branch %s: %s", *r.Ref, err)
								continue
							}
							sklog.Infof("Deleted fork branch %s", *r.Ref)
						}
					}
				}
			}()
		}
	}
	httputils.RunHealthCheckServer(*port)
}
