package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"go.chromium.org/luci/common/errors"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/conversion"
	"go.skia.org/infra/autoroll/go/config/db"
	"go.skia.org/infra/autoroll/go/config/deepvalidation"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/du"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	gsBucketAutoroll = "skia-autoroll"

	secretChatWebhooks = "autoroll-chat-webhooks"
	secretProject      = "skia-infra-public"

	configPassedAsFlag = "--config"
)

type HangOption string

const (
	hangNone                 HangOption = ""
	hangImmediately          HangOption = "immediately"
	hangBeforeRollerCreation HangOption = "before-roller-creation"
	hangBeforeRunning        HangOption = "before-running"
)

var hangOptions = []HangOption{hangNone, hangImmediately, hangBeforeRollerCreation, hangBeforeRunning}

// flags
var (
	configContents     = flag.String("config", "", "Base 64 encoded configuration in JSON format, mutually exclusive with --config_file.")
	configFile         = common.NewMultiStringFlag("config_file", nil, "Configuration file(s) to use, mutually exclusive with --config.")
	skipConfigFile     = common.NewMultiStringFlag("skip-config-file", nil, "Regular expression(s) indicating config files to skip. Only valid with --config_file.")
	firestoreInstance  = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port               = flag.String("port", ":8000", "HTTP service port.")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workdir            = flag.String("workdir", ".", "Directory to use for scratch work.")
	hang               = flag.String("hang", string(hangNone), fmt.Sprintf("If set, just hang and do nothing, at specified points in the code. Options: %v", hangOptions))
	validateConfig     = flag.Bool("validate-config", false, "If set, validate the config and exit without running the autoroll backend.")
	deepValidateConfig = flag.Bool("deep-validate-config", false, "If set, validate the config deeply, making necessary network requests, and exit. Note: the caller must have all of the permissions that the roller itself has.")
	genK8sConfig       = flag.String("gen-k8s-config", "", "Path to the root of the k8s-config repo. If set, generate a Kubernetes config file for the roller config(s) and write it in the given directory, without running the autoroll backend.")
)

// AutoRollerI is the common interface for starting an AutoRoller and handling HTTP requests.
type AutoRollerI interface {
	// Start initiates the AutoRoller's loop.
	Start(ctx context.Context, tickFrequency time.Duration)
	// AddHandlers allows the AutoRoller to respond to specific HTTP requests.
	AddHandlers(r chi.Router)
}

// clientConfig returns a common httputils.ClientConfig to be used for all
// HTTP clients.
func clientConfig(ts oauth2.TokenSource) httputils.ClientConfig {
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly()
	// Gerrit occasionally times out when creating CLs. Increase the request
	// timeout to compensate. Remove this when b/261896675 is fixed.
	c.RequestTimeout = 10 * time.Minute
	return c
}

func main() {
	// Parse and validate flags.
	common.InitWithMust(
		"autoroll-be",
		common.PrometheusOpt(promPort),
		common.StructuredLogging(local),
	)

	if HangOption(*hang) == hangImmediately {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
	}

	if (*configContents == "" && len(*configFile) == 0) || (*configContents != "" && len(*configFile) > 0) {
		sklog.Fatal("--config and --config_file are mutually exclusive")
	}
	if len(*configFile) > 1 && !(*validateConfig || *deepValidateConfig || *genK8sConfig != "") {
		sklog.Fatal("Multiple --config_file only supported with --validate-config, --deep-validate-config, or --gen-k8s-config.")
	}
	if *configContents != "" && *genK8sConfig != "" {
		sklog.Fatal("--config is not compatible with --gen-k8s-config. Use --config_file instead.")
	}
	// This is just to prevent confusion, since one would assume that deep
	// validation should happen *before* generating the k8s configs. We don't
	// do that because deep validation requires more setup and we'd prefer to
	// defer that until necessary.
	if *deepValidateConfig && *genK8sConfig != "" {
		sklog.Fatal("--deep-validate-config and --gen-k8s-config are mutually exclusive.")
	}
	var skipConfigFiles []*regexp.Regexp
	if skipConfigFile != nil {
		skipConfigFiles = make([]*regexp.Regexp, 0, len(*skipConfigFile))
		for _, skipRegex := range *skipConfigFile {
			re, err := regexp.Compile(skipRegex)
			if err != nil {
				sklog.Fatalf("Invalid regex for --skip_config_file %q: %s", skipRegex, err)
			}
			skipConfigFiles = append(skipConfigFiles, re)
		}
	}

	// Decode the config(s).
	configsMap := map[string]*config.Config{} // All provided configs.
	var cfg *config.Config                    // A single config; the one we'll actually use.
	var configBytes []byte                    // The raw bytes for cfg.
	if *configContents != "" {
		var err error
		configBytes, err = base64.StdEncoding.DecodeString(*configContents)
		if err != nil {
			sklog.Fatal(err)
		}
		cfg = new(config.Config)
		if err := prototext.Unmarshal(configBytes, cfg); err != nil {
			sklog.Fatal(err)
		}
		configsMap[configPassedAsFlag] = cfg // No filename, since the config was passed as a flag.
	} else {
		for _, configFileFlag := range *configFile {
			cfgFiles, err := filepath.Glob(configFileFlag)
			if err != nil {
				sklog.Fatal(err)
			}
			for _, cfgFile := range cfgFiles {
				if anyMatch(skipConfigFiles, cfgFile) {
					sklog.Infof("Skipping %s", cfgFile)
					continue
				}
				if err := util.WithReadFile(cfgFile, func(f io.Reader) error {
					var err error
					configBytes, err = io.ReadAll(f)
					return err
				}); err != nil {
					sklog.Fatal(err)
				}
				cfg = new(config.Config)
				if err := prototext.Unmarshal(configBytes, cfg); err != nil {
					sklog.Fatal(err)
				}
				configsMap[cfgFile] = cfg
			}
		}
	}

	// Validate the config(s), then exit if that's all we were supposed to do.
	for file, cfg := range configsMap {
		anyFailed := false
		if err := cfg.Validate(); err != nil {
			anyFailed = true
			if file == configPassedAsFlag {
				fmt.Fprintf(os.Stderr, "Config failed validation: %s\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Config file %s failed validation: %s\n\n", file, err)
			}
		}
		if anyFailed {
			os.Exit(1)
		}
	}
	if *validateConfig && !*deepValidateConfig {
		sklog.Infof("%d configs passed basic validation.", len(configsMap))
		return
	}

	ctx := context.Background()

	// Generate k8s configs, then exit if that's all we were supposed to do.
	if *genK8sConfig != "" {
		for file, cfg := range configsMap {
			if err := conversion.ConvertConfig(ctx, cfg, file, *genK8sConfig); err != nil {
				sklog.Fatalf("failed to convert config: %s", err)
			}
		}
		return
	}

	// Set up to run the autoroll backend.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
	if err != nil {
		sklog.Fatal(err)
	}
	client := clientConfig(ts).Client()

	user, err := user.Current()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Current user: %s; HOME=%s", user.Name, user.HomeDir)

	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create HTTP clients for Gerrit and Github.

	// Gerrit sometimes throws 404s for CLs that we've just uploaded, likely
	// due to eventual consistency. Rather than error out, use an HTTP
	// client which retries 4XX errors.
	gerritHttpClient := clientConfig(ts).WithRetry4XX().Client()

	// Instantiate githubClient using the github token secret.
	var githubHttpClient *http.Client
	// Find any GitHub config.
	var githubCfg *config.GitHubConfig
	for _, cfg := range configsMap {
		if cfg.GetGithub() != nil {
			githubCfg = cfg.GetGithub()
			break
		}
	}
	if githubCfg != nil {
		var gToken string
		if *local {
			pathToGithubToken := filepath.Join(user.HomeDir, github.GITHUB_TOKEN_FILENAME)
			gBody, err := os.ReadFile(pathToGithubToken)
			if err != nil {
				sklog.Fatalf("Couldn't find githubToken in %s: %s.", pathToGithubToken, err)
			}
			gToken = strings.TrimSpace(string(gBody))
		} else {
			gBody, err := secretClient.Get(ctx, secretProject, githubCfg.TokenSecret, secret.VersionLatest)
			if err != nil {
				sklog.Fatalf("Failed to retrieve secret %s: %s", githubCfg.TokenSecret, err)
			}
			gToken = strings.TrimSpace(gBody)

			// Setup the required SSH key from secrets if we are not running
			// locally and if the file does not already exist.
			sshKeyDestDir := filepath.Join(user.HomeDir, ".ssh")
			sshKeyDest := filepath.Join(sshKeyDestDir, github.SSH_KEY_FILENAME)
			if _, err := os.Stat(sshKeyDest); os.IsNotExist(err) {
				sshKey, err := secretClient.Get(ctx, secretProject, githubCfg.SshKeySecret, secret.VersionLatest)
				if err != nil {
					sklog.Fatalf("Failed to retrieve secret %s: %s", githubCfg.SshKeySecret, err)
				}
				if _, err := fileutil.EnsureDirExists(sshKeyDestDir); err != nil {
					sklog.Fatalf("Could not create %s: %s", sshKeyDest, err)
				}
				if err := os.WriteFile(sshKeyDest, []byte(sshKey), 0600); err != nil {
					sklog.Fatalf("Could not write to %s: %s", sshKeyDest, err)
				}
			}
			// Make sure github is added to known_hosts.
			github.AddToKnownHosts(ctx)
		}
		githubHttpClient = httputils.
			DefaultClientConfig().
			WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})).
			Client()
	}

	// Perform deep validation and exit if requested.
	if *deepValidateConfig {
		sklog.Info("Performing deep config validation")
		if err := deepvalidation.DeepValidateMulti(ctx, client, githubHttpClient, configsMap); err != nil {
			errors.WalkLeaves(err, func(err error) bool {
				fmt.Fprintf(os.Stderr, "%s\n\n", err)
				return true
			})
			os.Exit(1)
		}
		sklog.Info("All configs passed validation")
		os.Exit(0)
	}

	// Rollers use a custom temporary dir, to ensure that it's on a
	// persistent disk. Create it if it does not exist.
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		if err := os.Mkdir(os.TempDir(), os.ModePerm); err != nil {
			sklog.Fatalf("Failed to create %s: %s", os.TempDir(), err)
		}
	}

	// Periodically log disk usage of the working directory.
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		if err := du.PrintJSONReport(ctx, *workdir, 2, true); err != nil {
			sklog.Errorf("Failed to generate disk usage report: %s", err)
		}
	})

	namespace := ds.AUTOROLL_NS
	if cfg.IsInternal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	chatbot.Init(fmt.Sprintf("%s -> %s AutoRoller", cfg.ChildDisplayName, cfg.ParentDisplayName))

	var emailer email.Client
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
		sklog.Infof("Writing persistent data to gs://%s/%s", gsBucketAutoroll, rollerName)
		gcsClient = gcsclient.New(s, gsBucketAutoroll)

		emailer, err = email.NewClient(ctx)
		if err != nil {
			sklog.Fatal(err)
		}

		chatBotConfigReader = func() string {
			chatWebhooks, err := secretClient.Get(ctx, secretProject, secretChatWebhooks, secret.VersionLatest)
			if err != nil {
				sklog.Errorf("Failed to read chat config: %s", err)
				return ""
			} else {
				return chatWebhooks
			}
		}

		// Update the roller config in the DB.
		configDB, err := db.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, namespace, *firestoreInstance, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		if err := configDB.Put(ctx, cfg.RollerName, cfg); err != nil {
			sklog.Fatal(err)
		}
	}

	serverURL := roller.AutorollURLPublic + "/r/" + cfg.RollerName
	if cfg.IsInternal {
		serverURL = roller.AutorollURLPrivate + "/r/" + cfg.RollerName
	}

	// TODO(borenet/rmistry): Create a code review sub-config as described in
	// https://skia-review.googlesource.com/c/buildbot/+/116980/6/autoroll/go/autoroll/main.go#261
	// so that we can get rid of these vars and the various conditionals.
	var g *gerrit.Gerrit
	var githubClient *github.GitHub

	// The rollers use the gitcookie created by gitauth package.
	if !*local {
		// Prevent conflicts with other auth systems, eg. LUCI.
		if _, err := exec.RunSimple(ctx, "git config --global --unset-all credential.helper"); err != nil {
			sklog.Fatalf("Failed to unset credential.helper: %s", err)
		}

		gitcookiesPath := filepath.Join(user.HomeDir, ".gitcookies")
		sklog.Infof("Writing gitcookies to %s", gitcookiesPath)
		if _, err := gitauth.New(ctx, ts, gitcookiesPath, true, cfg.ServiceAccount); err != nil {
			sklog.Fatalf("Failed to create git cookie updater: %s", err)
		}
		sklog.Infof("Successfully initiated git authenticator")

		// Global git configuration.
		if _, err := exec.RunSimple(ctx, fmt.Sprintf("git config --global --add safe.directory %s", *workdir)); err != nil {
			sklog.Fatal("Failed to set global git config")
		}
	}

	if cfg.GetGerrit() != nil {
		// Create the code review API client.
		gc := cfg.GetGerrit()
		if gc == nil {
			sklog.Fatal("Gerrit config doesn't exist.")
		}
		gerritConfig := codereview.GerritConfigs[gc.Config]
		g, err = gerrit.NewGerritWithConfig(gerritConfig, gc.Url, gerritHttpClient)
		if err != nil {
			sklog.Fatalf("Failed to create Gerrit client: %s", err)
		}
	} else if cfg.GetGithub() != nil {
		gc := cfg.GetGithub()
		githubClient, err = github.NewGitHub(ctx, gc.RepoOwner, gc.RepoName, githubHttpClient)
		if err != nil {
			sklog.Fatalf("Could not create Github client: %s", err)
		}
	}

	sklog.Info("Creating status DB.")
	statusDB, err := status.NewDB(ctx, firestore.FIRESTORE_PROJECT, namespace, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create status DB: %s", err)
	}

	sklog.Info("Creating manual roll DB.")
	manualRolls, err := manual.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create manual roll DB: %s", err)
	}

	sklog.Infof("Creating roller cleanup DB.")
	rollerCleanup, err := roller_cleanup.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create roller cleanup DB: %s", err)
	}

	// Set environment variable for depot_tools.
	if err := os.Setenv("SKIP_GCE_AUTH_FOR_GIT", "1"); err != nil {
		sklog.Fatal(err)
	}

	if HangOption(*hang) == hangBeforeRollerCreation {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
	}

	arb, err := roller.NewAutoRoller(ctx, cfg, emailer, chatBotConfigReader, g, githubClient, *workdir, serverURL, gcsClient, client, rollerName, *local, statusDB, manualRolls, rollerCleanup)
	if err != nil {
		sklog.Fatal(err)
	}

	if HangOption(*hang) == hangBeforeRunning {
		sklog.Infof("--hang provided; doing nothing.")
		httputils.RunHealthCheckServer(*port)
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
				issues, err := g.Search(ctx, 100, true, gerrit.SearchOwner(myEmail), gerrit.SearchStatus(gerrit.ChangeStatusNew))
				if err != nil {
					sklog.Errorf("Failed to retrieve autoroller issues: %s", err)
					continue
				}
				for _, ci := range issues {
					if ci.Updated.Before(time.Now().Add(-168 * time.Hour)) {
						// Gerrit search sometimes returns incorrect statuses.
						// Reload the ChangeInfo and verify that we actually
						// need to abandon the CL.
						ci, err := g.GetChange(ctx, ci.Id)
						if err != nil {
							sklog.Errorf("Failed to retrieve change details: %s", err)
							continue
						}
						if ci.Status == gerrit.ChangeStatusNew {
							if err := g.Abandon(ctx, ci, "Abandoning new/draft issues older than a week."); err != nil && !strings.Contains(err.Error(), "change is abandoned") {
								sklog.Errorf("Failed to abandon old issue %s: %s", g.Url(ci.Issue), err)
							}
						}
					}
				}
			}
		}()
	} else if githubClient != nil && cfg.GetParentChildRepoManager() != nil {
		rm := cfg.GetParentChildRepoManager()
		var forkRepoURL string
		if rm.GetDepsLocalGithubParent() != nil {
			forkRepoURL = rm.GetDepsLocalGithubParent().ForkRepoUrl
		} else if rm.GetGitCheckoutGithubFileParent() != nil {
			forkRepoURL = rm.GetGitCheckoutGithubFileParent().GitCheckout.ForkRepoUrl
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
					forkRepoMatches := parent.REGitHubForkRepoURL.FindStringSubmatch(forkRepoURL)
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
						elapsedDuration := time.Since(creationTime)
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

// anyMatch returns true if any of the given regexes matches the given input
// string.
func anyMatch(regexes []*regexp.Regexp, inp string) bool {
	for _, regex := range regexes {
		if regex.MatchString(inp) {
			return true
		}
	}
	return false
}
