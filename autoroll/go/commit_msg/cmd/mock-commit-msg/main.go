package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/user"
	"path/filepath"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/pmezard/go-difflib/difflib"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/repo_root"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
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

	ctx := context.Background()

	ts, err := auth.NewDefaultTokenSource(true, auth.ScopeUserinfoEmail, auth.ScopeGerrit, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
	if err != nil {
		log.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Read the roller config file.
	cfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to read %s: %s", *configFile, err)
	}
	var cfg config.Config
	if err := prototext.Unmarshal(cfgBytes, &cfg); err != nil {
		log.Fatalf("Failed to decode config: %s", err)
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
		namespace := ds.AUTOROLL_NS
		if cfg.IsInternal {
			namespace = ds.AUTOROLL_INTERNAL_NS
		}
		if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
			log.Fatal(err)
		}

		var gerritClient *gerrit.Gerrit
		var githubClient *github.GitHub
		var cr codereview.CodeReview
		if cfg.GetGerrit() != nil {
			gc := cfg.GetGerrit()
			if gc == nil {
				log.Fatal("Gerrit config doesn't exist.")
			}
			gerritConfig := codereview.GerritConfigs[gc.Config]
			gerritClient, err = gerrit.NewGerritWithConfig(gerritConfig, gc.Url, client)
			if err != nil {
				log.Fatalf("Failed to create Gerrit client: %s", err)
			}
			cr, err = codereview.NewGerrit(gc, gerritClient, client)
			if err != nil {
				log.Fatalf("Failed to create Gerrit code review: %s", err)
			}
		} else if cfg.GetGithub() != nil {
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
			gc := cfg.GetGithub()
			if gc == nil {
				log.Fatal("Github config doesn't exist.")
			}
			githubClient, err = github.NewGitHub(ctx, gc.RepoOwner, gc.RepoName, githubHttpClient)
			if err != nil {
				log.Fatalf("Could not create Github client: %s", err)
			}
			cr, err = codereview.NewGitHub(gc, githubClient)
			if err != nil {
				log.Fatalf("Failed to create Gthub code review: %s", err)
			}
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

		rm, err := repo_manager.New(ctx, cfg.GetRepoManagerConfig(), reg, *workdir, cfg.RollerName, recipesCfgFile, *serverURL, cfg.ServiceAccount, client, cr, cfg.IsInternal, true)
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
		if cfg.GetGerrit() != nil {
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
		} else if cfg.GetGithub() != nil {
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
		reviewers, err = roller.GetReviewers(cfg.RollerName, cfg.Reviewer, cfg.ReviewerBackup)
		if err != nil {
			log.Fatalf("Failed to retrieve reviewers: %s", err)
		}
	}

	// Create the commit message builder.
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		log.Fatalf("Failed to create config var registry: %s", err)
	}
	b, err := commit_msg.NewBuilder(cfg.CommitMsg, reg, cfg.ChildDisplayName, cfg.ParentDisplayName, *serverURL, cfg.ChildBugLink, cfg.ParentBugLink, cfg.TransitiveDeps)
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
