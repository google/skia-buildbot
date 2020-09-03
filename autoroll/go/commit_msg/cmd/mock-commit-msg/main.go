package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/user"
	"path/filepath"
	"strings"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"github.com/flynn/json5"
	"github.com/pmezard/go-difflib/difflib"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

var (
	configFile = flag.String("config", "", "Config file to parse. Required.")
	compare    = flag.Bool("compare", false, "Compare the generated commit message against the most recent actual commit message.")
	serverURL  = flag.String("server_url", "", "Server URL. Optional.")
	workdir    = flag.String("workdir", "", "Working directory. If not set, a temporary directory is created.")
)

func main() {
	common.Init()

	// Validation.
	if *configFile == "" {
		log.Fatal("--config is required.")
	}

	// Read the roller config file.
	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*configFile, func(r io.Reader) error {
		return json5.NewDecoder(r).Decode(&cfg)
	}); err != nil {
		log.Fatalf("Failed to read %s: %s", *configFile, err)
	}

	// Fake the serverURL based on the roller name.
	if *serverURL == "" {
		*serverURL = fmt.Sprintf("https://autoroll.skia.org/r/%s", cfg.RollerName)
	}

	// Obtain data to use for the commit message. If --compare was provided, use
	// the most recent actual commit message.
	var from, to *revision.Revision
	var revs []*revision.Revision
	var reviewers []string
	var realCommitMsg string
	var realRollURL string
	if *compare {
		// Create the working directory.
		if *workdir == "" {
			wd, err := ioutil.TempDir("", "")
			if err != nil {
				log.Fatal(err)
			}
			*workdir = wd
		}

		// Create the RepoManager.
		ctx := context.Background()
		ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
		if err != nil {
			log.Fatal(err)
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

		s, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}
		gcsClient := gcsclient.New(s, "skia-autoroll")

		var gerritClient *gerrit.Gerrit
		var githubClient *github.GitHub
		if cfg.Gerrit != nil {
			gc, err := cfg.Gerrit.GetConfig()
			if err != nil {
				log.Fatalf("Failed to get Gerrit config: %s", err)
			}
			gerritClient, err = gerrit.NewGerritWithConfig(gc, cfg.Gerrit.URL, client)
			if err != nil {
				log.Fatalf("Failed to create Gerrit client: %s", err)
			}
		} else if cfg.Github != nil {
			user, err := user.Current()
			if err != nil {
				log.Fatal(err)
			}
			pathToGithubToken := filepath.Join(user.HomeDir, github.GITHUB_TOKEN_FILENAME)
			// Instantiate githubClient using the github token secret.
			gBody, err := ioutil.ReadFile(pathToGithubToken)
			if err != nil {
				log.Fatalf("Couldn't find githubToken in %s: %s.", pathToGithubToken, err)
			}
			gToken := strings.TrimSpace(string(gBody))
			githubHttpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken}))
			githubClient, err = github.NewGitHub(ctx, cfg.Github.RepoOwner, cfg.Github.RepoName, githubHttpClient)
			if err != nil {
				log.Fatalf("Could not create Github client: %s", err)
			}
		}

		cr, err := cfg.CodeReview().Init(gerritClient, githubClient)
		if err != nil {
			log.Fatalf("Failed to initialize code review: %s", err)
		}
		reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
		if err != nil {
			log.Fatalf("Failed to create config var registry: %s", err)
		}

		repoRoot, err := repo_root.Get()
		if err != nil {
			log.Fatal(err)
		}
		recipesCfgFile := filepath.Join(repoRoot, "infra", "config", "recipes.cfg")

		rm, err := cfg.CreateRepoManager(ctx, cr, reg, gerritClient, githubClient, *workdir, recipesCfgFile, *serverURL, cfg.RollerName, gcsClient, client, true)
		if err != nil {
			log.Fatal(err)
		}

		statusDB := status.NewDatastoreDB()
		st, err := statusDB.Get(ctx, cfg.RollerName)
		if err != nil {
			log.Fatal(err)
		}
		if len(st.Recent) == 0 {
			log.Fatal("No recent commit messages to compare against!")
		}
		lastRoll := st.Recent[0]
		fromRev, err := rm.GetRevision(ctx, lastRoll.RollingFrom)
		if err != nil {
			log.Fatal(err)
		}
		from = fromRev
		toRev, err := rm.GetRevision(ctx, lastRoll.RollingTo)
		if err != nil {
			log.Fatal(err)
		}
		to = toRev
		// TODO(borenet): We don't have a RepoManager.Log(from, to) method to
		// return a slice of revisions, so we can't form an actual list here.
		revs = []*revision.Revision{to}
		if cfg.Gerrit != nil {
			ci, err := gerritClient.GetIssueProperties(ctx, lastRoll.Issue)
			if err != nil {
				log.Fatalf("Failed to get change: %s", err)
			}
			commit, err := gerritClient.GetCommit(ctx, ci.Issue, ci.Patchsets[len(ci.Patchsets)-1].ID)
			if err != nil {
				log.Fatalf("Failed to get commit: %s", err)
			}
			var realCommitMsgLines []string
			for _, line := range strings.Split(commit.Message, "\n") {
				// Filter out automatically-added lines.
				if strings.HasPrefix(line, "Change-Id") ||
					strings.HasPrefix(line, "Reviewed-on") ||
					strings.HasPrefix(line, "Reviewed-by") ||
					strings.HasPrefix(line, "Commit-Queue") ||
					strings.HasPrefix(line, "Cr-Commit-Position") ||
					strings.HasPrefix(line, "Cr-Branched-From") {
					continue
				}
				realCommitMsgLines = append(realCommitMsgLines, line)
			}
			realCommitMsg = strings.Join(realCommitMsgLines, "\n")
			for _, reviewer := range ci.Reviewers.Reviewer {
				// Exclude automatically-added reviewers.
				if strings.Contains(reviewer.Email, "gserviceaccount") ||
					strings.Contains(reviewer.Email, "commit-bot") {
					continue
				}
				reviewers = append(reviewers, reviewer.Email)
			}
			realRollURL = gerritClient.Url(ci.Issue)
		} else if cfg.Github != nil {
			pr, err := githubClient.GetPullRequest(int(lastRoll.Issue))
			if err != nil {
				log.Fatalf("Failed to get pull request: %s", err)
			}
			realCommitMsg = *pr.Title + "\n" + *pr.Body
			for _, reviewer := range pr.RequestedReviewers {
				reviewers = append(reviewers, *reviewer.Email)
			}
			realRollURL = *pr.HTMLURL
		} else {
			log.Fatal("Either Gerrit or Github is required.")
		}
	} else {
		from, to, revs, _ = commit_msg.FakeCommitMsgInputs()
		var err error
		reviewers, err = roller.GetSheriff(cfg.RollerName, cfg.Sheriff, cfg.SheriffBackup)
		if err != nil {
			log.Fatalf("Failed to retrieve sheriff: %s", err)
		}
	}

	// Create the commit message builder.
	b, err := commit_msg.NewBuilder(cfg.CommitMsgConfig, cfg.ChildDisplayName, *serverURL, cfg.TransitiveDeps)
	if err != nil {
		log.Fatalf("Failed to create commit message builder: %s", err)
	}

	// Build the commit message.
	genCommitMsg, err := b.Build(from, to, revs, reviewers)
	if err != nil {
		log.Fatalf("Failed to build commit message: %s", err)
	}
	if *compare {
		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(realCommitMsg)),
			B:        difflib.SplitLines(string(genCommitMsg)),
			FromFile: "Generated",
			ToFile:   "Actual",
			Context:  3,
			Eol:      "\n",
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Found most recent real roll:")
		fmt.Println(realRollURL)
		if diff != "" {
			fmt.Println("Generated commit message differs from most recent actual commit message (note that revision lists generated by this tool will be incomplete):")
			fmt.Println(diff)
			fmt.Println("=====================================================")
			fmt.Println("Full old commit message:")
			fmt.Println("=====================================================")
			fmt.Println(realCommitMsg)
			fmt.Println("=====================================================")
			fmt.Println("Full new commit message:")
			fmt.Println("=====================================================")
			fmt.Println(genCommitMsg)
		} else {
			fmt.Println("Generated commit message is identical to most recent actual commit message:")
			fmt.Println(genCommitMsg)
		}
	} else {
		fmt.Println(genCommitMsg)
	}
}
