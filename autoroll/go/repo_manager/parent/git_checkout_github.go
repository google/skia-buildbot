package parent

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutGithubConfig provides configuration for Parents which use a local
// git checkout and upload changes to GitHub.
type GitCheckoutGithubConfig struct {
	GitCheckoutConfig
	ForkBranchName string `json:"forkBranchName"`
	ForkRepoURL    string `json:"forkRepoURL"`
}

// See documentation for util.Validator interface.
func (c GitCheckoutGithubConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ForkRepoURL == "" {
		return skerr.Fmt("ForkRepoURL is required")
	}
	return nil
}

// GitCheckoutUploadGithubRollFunc returns
func GitCheckoutUploadGithubRollFunc(githubClient *github.GitHub, userName, forkBranchName string) git_common.UploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool, commitMsg string) (int64, error) {
		// Make sure the forked repo is at the same hash as the target repo
		// before creating the pull request.
		if _, err := co.Git(ctx, "push", "-f", github_common.GithubForkRemoteName, fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Push the changes to the forked repository.
		if _, err := co.Git(ctx, "push", "-f", github_common.GithubForkRemoteName, fmt.Sprintf("%s:%s", git_common.RollBranch, forkBranchName)); err != nil {
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
		pr, err := githubClient.CreatePullRequest(title, upstreamBranch, headBranch, strings.Join(descComment, "\n"))
		if err != nil {
			return 0, skerr.Wrap(err)
		}

		// Add appropriate label to the pull request.
		if !dryRun {
			if err := githubClient.AddLabel(pr.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
				return 0, skerr.Wrap(err)
			}
		}

		return int64(pr.GetNumber()), nil
	}
}

// NewGitCheckoutGithub returns an implementation of Parent which uses a local
// git checkout and uploads pull requests to Github.
func NewGitCheckoutGithub(ctx context.Context, c GitCheckoutGithubConfig, reg *config_vars.Registry, githubClient *github.GitHub, serverURL, workdir, userName, userEmail string, co *git.Checkout, createRoll git_common.CreateRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// See documentation for GitCheckoutUploadRollFunc.
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, userName, c.ForkBranchName)

	// Create the GitCheckout Parent.
	p, err := NewGitCheckout(ctx, c.GitCheckoutConfig, reg, serverURL, workdir, userName, userEmail, co, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, p.Checkout.Checkout, c.ForkRepoURL); err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}
