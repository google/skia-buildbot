package parent

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutGithubConfig provides configuration for Parents which use a local
// git checkout and upload changes to GitHub.
type GitCheckoutGithubConfig struct {
	GitCheckoutConfig
	Github *codereview.GithubConfig `json:"github"`
}

// See documentation for util.Validator interface.
func (c GitCheckoutGithubConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Github.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, _, err := github_common.SplitGithubUserAndRepo(c.RepoURL); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGitCheckoutGithub returns an implementation of Parent which uses a local
// git checkout and uploads pull requests to Github.
func NewGitCheckoutGithub(ctx context.Context, c GitCheckoutGithubConfig, reg *config_vars.Registry, githubClient *github.GitHub, serverURL, workdir, userName, userEmail string, co *git.Checkout, getLastRollRev GitCheckoutGetLastRollRevFunc, createRoll GitCheckoutCreateRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Set up the fork.
	_, repo, err := github_common.SplitGithubUserAndRepo(c.RepoURL)
	forkRepoURL := fmt.Sprintf("git@github.com:%s/%s.git", userName, repo)
	forkRemoteName := "fork"

	// This is created later but needed in uploadRoll.
	var p *GitCheckoutParent

	// See documentation for GitCheckoutUploadRollFunc.
	uploadRoll := func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool) (int64, error) {
		// Make sure the forked repo is at the same hash as the target repo
		// before creating the pull request.
		if _, err := co.Git(ctx, "push", "-f", forkRemoteName, fmt.Sprintf("origin/%s", c.Branch.String())); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Push the changes to the forked repository.
		if _, err := co.Git(ctx, "push", "-f", forkRemoteName, rollBranch); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Build the commit message.
		out, err := co.Git(ctx, "log", "-n1", hash)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		commitMsgLines := strings.Split(out, "\n")
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
		headBranch := fmt.Sprintf("%s:%s", userName, rollBranch)
		pr, err := githubClient.CreatePullRequest(title, p.Branch.String(), headBranch, strings.Join(descComment, "\n"))
		if err != nil {
			return 0, err
		}

		// Add appropriate label to the pull request.
		if !dryRun {
			if err := githubClient.AddLabel(pr.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
				return 0, err
			}
		}

		return int64(pr.GetNumber()), nil
	}

	// Create the GitCheckout Parent.
	p, err = NewGitCheckout(ctx, c.GitCheckoutConfig, reg, serverURL, workdir, userName, userEmail, co, getLastRollRev, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Check to see whether we have a remote for the fork.
	remoteOutput, err := p.Git(ctx, "remote", "show")
	if err != nil {
		return nil, err
	}
	remoteFound := false
	remoteLines := strings.Split(remoteOutput, "\n")
	for _, remoteLine := range remoteLines {
		if remoteLine == forkRemoteName {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		if _, err := p.Git(ctx, "remote", "add", forkRemoteName, forkRepoURL); err != nil {
			return nil, err
		}
	}
	if _, err := p.Git(ctx, "fetch", forkRemoteName); err != nil {
		return nil, err
	}

	return p, nil
}
