package parent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	github_api "github.com/google/go-github/v29/github"
	"github.com/google/uuid"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// REGitHubForkRepoURL is a regular expression which matches a GitHub fork
	// repo URL.
	REGitHubForkRepoURL = regexp.MustCompile(`^(git@github.com:|file:///)(.*)/(.*?)(\.git)?$`)
)

// ApplyExternalChangeGithubFunc returns a ApplyExternalChangeFunc which
// handles external change Ids for github checkouts.
func ApplyExternalChangeGithubFunc() git_common.ApplyExternalChangeFunc {
	return func(ctx context.Context, co *git.Checkout, externalChangeId string) error {
		// Fetch specified PR locally.
		if _, err := co.Git(ctx, "fetch", "origin", fmt.Sprintf("pull/%s/head", externalChangeId)); err != nil {
			return skerr.Wrap(err)
		}
		// Cherry-pick the PR patch without committing.
		if _, err := co.Git(ctx, "cherry-pick", "--no-commit", "FETCH_HEAD"); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	}
}

// GitCheckoutUploadGithubRollFunc returns a UploadRollFunc which uploads a CL
// to Github.
func GitCheckoutUploadGithubRollFunc(githubClient *github.GitHub, userName, rollerName, forkRepoURL string) git_common.UploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool, commitMsg string) (int64, error) {

		// Generate a fork branch name with unique id and creation timestamp.
		forkBranchName := fmt.Sprintf("%s-%s-%d", rollerName, uuid.New().String(), time.Now().Unix())
		// Find forkRepo owner and name.
		forkRepoMatches := REGitHubForkRepoURL.FindStringSubmatch(forkRepoURL)
		forkRepoOwner := forkRepoMatches[2]
		forkRepoName := forkRepoMatches[3]
		// Find SHA of main branch to use when creating the fork branch. It does not really
		// matter which SHA we use, we just have to use one that exists on the server. Always
		// get the SHA from the main branch because it should always exist.
		forkMainRef, err := githubClient.GetReference(forkRepoOwner, forkRepoName, git.DefaultRef)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		// Create the fork branch.
		if err := githubClient.CreateReference(forkRepoOwner, forkRepoName, fmt.Sprintf("refs/heads/%s", forkBranchName), *forkMainRef.Object.SHA); err != nil {
			return 0, skerr.Wrap(err)
		}
		sklog.Infof("Created branch %s in %s with SHA %s", forkBranchName, forkRepoURL, *forkMainRef.Object.SHA)

		// Make sure the forked repo is at the same hash as the target repo
		// before creating the pull request.
		if _, err := co.Git(ctx, "push", "-f", "--no-verify", github_common.GithubForkRemoteName, fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Push the changes to the forked repository.
		if _, err := co.Git(ctx, "push", "-f", "--no-verify", github_common.GithubForkRemoteName, fmt.Sprintf("%s:%s", git_common.RollBranch, forkBranchName)); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Build the commit message.
		commitMsg = strings.ReplaceAll(commitMsg, "git@github.com:", "https://github.com/")
		commitMsgLines := strings.Split(commitMsg, "\n")
		// Grab the first line of the commit msg to use as the title of the pull
		// request.
		title := commitMsgLines[0]
		// Use the remaining part of the commit message as the pull request
		// description.
		descComment := commitMsgLines[1:]
		if len(commitMsgLines) > 50 {
			// Truncate too large description comment because Github API cannot
			// handle large comments.
			descComment = append(commitMsgLines[:50], "...")
		}
		// Create a pull request.
		headBranch := fmt.Sprintf("%s:%s", userName, forkBranchName)
		var pr *github_api.PullRequest
		createPullRequestFunc := func() error {
			pr, err = githubClient.CreatePullRequest(title, upstreamBranch, headBranch, strings.Join(descComment, "\n"))
			return skerr.Wrap(err)
		}
		if err := backoff.Retry(createPullRequestFunc, codereview.GithubBackOffConfig); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Add appropriate label to the pull request.
		if !dryRun {
			addLabelFunc := func() error {
				return githubClient.AddLabel(pr.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL)
			}
			if err := backoff.Retry(addLabelFunc, codereview.GithubBackOffConfig); err != nil {
				return 0, skerr.Wrap(err)
			}
		}

		return int64(pr.GetNumber()), nil
	}
}

// NewGitCheckoutGithub returns an implementation of Parent which uses a local
// git checkout and uploads pull requests to Github.
func NewGitCheckoutGithub(ctx context.Context, c *config.GitCheckoutGitHubParentConfig, reg *config_vars.Registry, serverURL, workdir, rollerName string, cr codereview.CodeReview, createRoll git_common.CreateRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	githubClient, ok := cr.Client().(*github.GitHub)
	if !ok {
		return nil, skerr.Fmt("GitCheckoutGithub must use GitHub for code review.")
	}

	// See documentation for GitCheckoutUploadRollFunc.
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, cr.UserName(), rollerName, c.ForkRepoUrl)

	// Create the GitCheckout Parent.
	p, err := NewGitCheckout(ctx, c.GitCheckout, reg, workdir, cr, nil, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, p.Checkout.Checkout, c.ForkRepoUrl); err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}
