package github_common

import (
	"context"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

const (
	// GithubForkRemoteName is the name of the git remote used by Checkouts
	// which use GitHub.
	GithubForkRemoteName = "fork"
)

// SetupGithub performs additional setup for a Checkout which uses Github. This
// is required when not using NewGitCheckoutGithub to create the Parent.
// TODO(borenet): This is needed for RepoManagers which use NewDEPSLocal, since
// they need to pass in a GitCheckoutUploadRollFunc but can't do other
// initialization. Find a way to make this unnecessary.
func SetupGithub(ctx context.Context, co *git.Checkout, forkRepoURL string) error {
	// Check to see whether we have a remote for the fork.
	remoteOutput, err := co.Git(ctx, "remote", "show")
	if err != nil {
		return skerr.Wrap(err)
	}
	remoteFound := false
	remoteLines := strings.Split(remoteOutput, "\n")
	for _, remoteLine := range remoteLines {
		if remoteLine == GithubForkRemoteName {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		if _, err := co.Git(ctx, "remote", "add", GithubForkRemoteName, forkRepoURL); err != nil {
			return skerr.Wrap(err)
		}
	}
	if _, err := co.Git(ctx, "fetch", GithubForkRemoteName); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
