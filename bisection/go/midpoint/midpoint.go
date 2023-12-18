package midpoint

import (
	"context"
	"slices"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	GITILES_EMPTY_RESP_ERROR = "Gitiles returned 0 commits, which should not happen."
)

// A RepoFork represents revision overrides in addition to the base git revision.
// For example, if Commit is chromium/src@1, RepoFork may be V8@2 which is passed
// along to Buildbucket as a deps_revision_overrides.
// TODO(jeffyoon@) - Reorganize this into a types folder.
type RepoFork struct {
	// repositoryUrl is the url to the repository, ie/ https://chromium.googlesource.com/chromium/src
	RepositoryUrl string

	// startGitHash is the starting revision that was used to determine the next candidate.
	StartGitHash string

	// endGitHash is the ending revision that was used to determine the next candidate.
	EndGitHash string

	// nextGitHash is the next git revision that should be used to test with for the specified repository.
	NextGitHash string
}

// A Commit represents the next git revision to build the project at.
// TODO(jeffyoon@) - Reorganize this into a types folder.
type Commit struct {
	// gitHash is the Git SHA1 hash to build for the project.
	GitHash string

	// repositoryUrl is the url to the repository, ie/ https://chromium.googlesource.com/chromium/src
	RepositoryUrl string

	// RepoFork defines any deps_revisions_overrides properties that should be passed along to the build call.
	RepoFork *RepoFork
}

// isGitHashValid checks validity of the git hash.
func isGitHashValid(gitHash string) error {
	if gitHash == "" {
		return skerr.Fmt("Git hash is a required parameter and must be defined.")
	}

	return nil
}

// GetMidpoint determines the next candidate for Bisection by selecting the middle change
// from a given range of changes for a specific project. If the two changes are
// adjacent, a DEPS roll is assumed and we search for the next culprit in the
// range of commits before and after the roll for that specific project.
// See doc.go for an example.
func GetMidpoint(ctx context.Context, gc gitiles.GitilesRepo, repositoryUrl, startGitHash, endGitHash string) (*Commit, error) {
	if startGitHash == endGitHash {
		return nil, skerr.Fmt("Both git hashes are the same; Start: %s, End: %s", startGitHash, endGitHash)
	}
	if err := isGitHashValid(startGitHash); err != nil {
		return nil, skerr.Wrapf(err, "StartGitHash: %s", startGitHash)
	}
	if err := isGitHashValid(endGitHash); err != nil {
		return nil, skerr.Wrapf(err, "EndGitHash: %s", endGitHash)
	}

	// Find the midpoint between the provided commit hashes. Take the lower bound
	// if the list is an odd count. If the gitiles response is == endGitHash, it
	// this means both start and end are adjacent, and DEPS needs to be unravelled
	// to find the potential culprit.
	// LogLinear will return in reverse chronological order, inclusive of the end git hash.
	lc, err := gc.LogLinear(ctx, startGitHash, endGitHash)
	if err != nil {
		return nil, err
	}

	// The list can only be empty if the start and end commits are the same.
	if len(lc) == 0 {
		return nil, skerr.Fmt("%s. Start %s and end %s hashes may be reversed.", GITILES_EMPTY_RESP_ERROR, startGitHash, endGitHash)
	}

	mid := &Commit{
		RepositoryUrl: repositoryUrl,
		RepoFork:      nil,
	}

	// Two adjacent commits returns one commit equivalent to the end git hash.
	if len(lc) == 1 && lc[0].ShortCommit.Hash == endGitHash {
		sklog.Debugf("Start hash %s and end hash %s are adjacent to each other. Assuming a DEPS roll.", startGitHash, endGitHash)
		// TODO(jeffyoon@): Add DEPS processing.
		mid.GitHash = startGitHash
		return mid, nil
	}

	// Pop off the first element, since it's the end hash.
	// Golang divide will return lower bound when odd.
	lc = lc[1:]

	// Sort to chronological order before taking the midpoint. This means for even
	// lists, we opt to the higher index (ie/ in [1,2,3,4] len == 4 and midpoint
	// becomes index 2 (which = 3))
	slices.Reverse(lc)
	mlc := lc[len(lc)/2]

	mid.GitHash = mlc.ShortCommit.Hash

	return mid, nil
}
