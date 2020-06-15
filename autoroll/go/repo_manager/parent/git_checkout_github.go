package parent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutGithubConfig provides configuration for Parents which use a local
// git checkout and upload changes to GitHub.
// HERE HERE HERE
type GitCheckoutGithubConfig struct {
	GitCheckoutConfig
	// rmistry forkBranchName.
	// this needs to go and be created on the fly. REMOVE THIS
	// ForkBranchName string `json:"forkBranchName"`

	RollerName string `json:"rollerName"`

	// rmistry: forkRepoURL
	// Actually this is ok. This is just the user+repo name. Not the branch name. Above ForkBranchName is what should be
	// uniquely generated.
	ForkRepoURL string `json:"forkRepoURL"`
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

// rmistry: Can create own CreateNewRoll vs git_checkout that calls git_common if you want.
// Here you could create your own fork I think and then do what with that?
// create own fork and then call the other function to reuse things?

// GitCheckoutUploadGithubRollFunc returns
func GitCheckoutUploadGithubRollFunc(githubClient *github.GitHub, userName, rollerName string) git_common.UploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool, commitMsg string) (int64, error) {

		// Generate a fork branch name.
		forkBranchName := fmt.Sprintf("%s-%s", rollerName, uuid.New().String())
		// Create the fork branch.
		fmt.Println(forkBranchName)

		// DEFER THE DELETION OF IT!!!! cannot do that. must do that after it lands..

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
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, userName, c.RollerName)

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
