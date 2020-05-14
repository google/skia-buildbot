package parent

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

const (
	TMPL_COMMIT_MSG_GITHUB = `Roll {{.ChildPath}} {{.RollingFrom.String}}..{{.RollingTo.String}} ({{len .Revisions}} commits)

{{.ChildRepo}}/compare/{{.RollingFrom.String}}...{{.RollingTo.String}}

{{if .IncludeLog}}git log {{.RollingFrom}}..{{.RollingTo}} --first-parent --oneline
{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}{{end}}{{if len .TransitiveDeps}}
Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.Dep}} {{.RollingFrom}}..{{.RollingTo}}
{{end}}{{end}}

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+doc/master/autoroll/README.md

`

	githubForkRemoteName = "fork"
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
func GitCheckoutUploadGithubRollFunc(githubClient *github.GitHub, userName, forkBranchName string) GitCheckoutUploadRollFunc {
	return func(ctx context.Context, co *git.Checkout, upstreamBranch, hash string, emails []string, dryRun bool, commitMsg string) (int64, error) {
		// Make sure the forked repo is at the same hash as the target repo
		// before creating the pull request.
		if _, err := co.Git(ctx, "push", "-f", githubForkRemoteName, fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
			return 0, skerr.Wrap(err)
		}

		// Push the changes to the forked repository.
		if _, err := co.Git(ctx, "push", "-f", githubForkRemoteName, fmt.Sprintf("%s:%s", rollBranch, forkBranchName)); err != nil {
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
func NewGitCheckoutGithub(ctx context.Context, c GitCheckoutGithubConfig, reg *config_vars.Registry, githubClient *github.GitHub, serverURL, workdir, userName, userEmail string, co *git.Checkout, createRoll GitCheckoutCreateRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.CommitMsgTmpl == "" {
		c.CommitMsgTmpl = TMPL_COMMIT_MSG_GITHUB
	}

	// See documentation for GitCheckoutUploadRollFunc.
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, userName, c.ForkBranchName)

	// Create the GitCheckout Parent.
	p, err := NewGitCheckout(ctx, c.GitCheckoutConfig, reg, serverURL, workdir, userName, userEmail, co, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := SetupGithub(ctx, p, c.ForkRepoURL); err != nil {
		return nil, skerr.Wrap(err)
	}
	return p, nil
}

// SetupGithub performs additional setup for a GitCheckoutParent which uses
// Github. This is required when not using NewGitCheckoutGithub to create the
// Parent.
// TODO(borenet): This is needed for RepoManagers which use NewDEPSLocal, since
// they need to pass in a GitCheckoutUploadRollFunc but can't do other
// initialization. Find a way to make this unnecessary.
func SetupGithub(ctx context.Context, p *GitCheckoutParent, forkRepoURL string) error {
	// Check to see whether we have a remote for the fork.
	remoteOutput, err := p.Git(ctx, "remote", "show")
	if err != nil {
		return skerr.Wrap(err)
	}
	remoteFound := false
	remoteLines := strings.Split(remoteOutput, "\n")
	for _, remoteLine := range remoteLines {
		if remoteLine == githubForkRemoteName {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		if _, err := p.Git(ctx, "remote", "add", githubForkRemoteName, forkRepoURL); err != nil {
			return skerr.Wrap(err)
		}
	}
	if _, err := p.Git(ctx, "fetch", githubForkRemoteName); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
