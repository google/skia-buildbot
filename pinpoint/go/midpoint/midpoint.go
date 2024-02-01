package midpoint

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	GITILES_EMPTY_RESP_ERROR = "Gitiles returned 0 commits, which should not happen."
)

// A Commit represents a commit of a given repository.
// TODO(jeffyoon@) - Reorganize this into a types folder.
type Commit struct {
	// GitHash is the Git SHA1 hash to build for the project.
	GitHash string

	// RepositoryUrl is the url to the repository, ie/ https://chromium.googlesource.com/chromium/src
	RepositoryUrl string
}

// A CombinedCommit represents one main base commit with any dependencies that require
// overrides as part of the build request.
// For example, if Commit is chromium/src@1, Dependency may be V8@2 which is passed
// along to Buildbucket as a deps_revision_overrides.
type combinedCommit struct {
	// Main is the main base commit, usually a Chromium commit.
	Main *Commit
	// ModifiedDeps is a list of commits to provide as overrides, ie/ V8.
	ModifiedDeps []*Commit
}

// TODO(jeffyoon@) - move this to a deps folder, likely with the types restructure above.
// DepsToMap translates all deps into a map.
func (cc *combinedCommit) DepsToMap() map[string]string {
	resp := make(map[string]string, 0)
	for _, c := range cc.ModifiedDeps {
		resp[c.RepositoryUrl] = c.GitHash
	}
	return resp
}

// GetMainGitHash returns the git hash of main.
func (cc *combinedCommit) GetMainGitHash() string {
	if cc.Main == nil {
		return ""
	}

	return cc.Main.GitHash
}

func NewCombinedCommit(main *Commit, deps ...*Commit) *combinedCommit {
	return &combinedCommit{
		Main:         main,
		ModifiedDeps: deps,
	}
}

// CommitRange provides information about the left and right commits used to determine
// the next commit to bisect against.
type CommitRange struct {
	Left  *combinedCommit
	Right *combinedCommit
}

// HasLeftGitHash checks if left main git hash is set.
func (cr *CommitRange) HasLeftGitHash() bool {
	return cr.Left.Main.GitHash != ""
}

// HasRightGitHash checks if left main git hash is set
func (cr *CommitRange) HasRightGitHash() bool {
	return cr.Right.Main.GitHash != ""
}

type MidpointHandler interface {
	// DetermineNextCandidate returns the next target for bisection for the provided url, inbetween the start and end git hashes.
	DetermineNextCandidate(ctx context.Context, baseUrl, startGitHash, endGitHash string) (*combinedCommit, *CommitRange, error)
}

// MidpointHandler encapsulates all logic to determine the next potential candidate for Bisection.
type midpointHandler struct {
	// repos is a map of repository url to a GitilesRepo object.
	repos map[string]gitiles.GitilesRepo

	c *http.Client
}

// New returns a new MidpointHandler.
func New(ctx context.Context, c *http.Client) *midpointHandler {
	return &midpointHandler{
		repos: make(map[string]gitiles.GitilesRepo, 0),
		c:     c,
	}
}

// WithRepo returns a MidpointHandler with the repository url mapped to a GitilesRepo object.
func (m *midpointHandler) WithRepo(url string, r gitiles.GitilesRepo) *midpointHandler {
	m.repos[url] = r
	return m
}

// getOrCreateRepo fetches the gitiles.GitilesRepo object for the repository url.
// If not present, it'll create an authenticated Repo client.
func (m *midpointHandler) getOrCreateRepo(url string) gitiles.GitilesRepo {
	gr, ok := m.repos[url]
	if !ok {
		gr := gitiles.NewRepo(url, m.c)
		m.repos[url] = gr
	}
	return gr
}

// findMidpoint identiifes the median commit given a start and ending git hash.
func (m *midpointHandler) findMidpoint(ctx context.Context, url, startGitHash, endGitHash string) (string, error) {
	if startGitHash == endGitHash {
		return "", skerr.Fmt("Both git hashes are the same; Start: %s, End: %s", startGitHash, endGitHash)
	}

	gc := m.getOrCreateRepo(url)

	// Find the midpoint between the provided commit hashes. Take the lower bound
	// if the list is an odd count. If the gitiles response is == endGitHash, it
	// this means both start and end are adjacent, and DEPS needs to be unravelled
	// to find the potential culprit.
	// LogLinear will return in reverse chronological order, inclusive of the end git hash.
	lc, err := gc.LogLinear(ctx, startGitHash, endGitHash)
	if err != nil {
		return "", err
	}

	// The list can only be empty if the start and end commits are the same.
	if len(lc) == 0 {
		return "", skerr.Fmt("%s. Start %s and end %s hashes may be reversed.", GITILES_EMPTY_RESP_ERROR, startGitHash, endGitHash)
	}

	// Two adjacent commits returns one commit equivalent to the end git hash.
	if len(lc) == 1 && lc[0].ShortCommit.Hash == endGitHash {
		return startGitHash, nil
	}

	// Pop off the first element, since it's the end hash.
	// Golang divide will return lower bound when odd.
	lc = lc[1:]

	// Sort to chronological order before taking the midpoint. This means for even
	// lists, we opt to the higher index (ie/ in [1,2,3,4] len == 4 and midpoint
	// becomes index 2 (which = 3))
	slices.Reverse(lc)
	mlc := lc[len(lc)/2]

	return mlc.ShortCommit.Hash, nil
}

// fetchGitDeps calls Gitiles to read the DEPS content and parses out only the git-based dependencies.
func (m *midpointHandler) fetchGitDeps(ctx context.Context, gc gitiles.GitilesRepo, gitHash string) (map[string]string, error) {
	denormalized := make(map[string]string, 0)

	content, err := gc.ReadFileAtRef(ctx, "DEPS", gitHash)
	if err != nil {
		return denormalized, err
	}

	entries, err := deps_parser.ParseDeps(string(content))
	if err != nil {
		return denormalized, err
	}

	// We have no good way of determining whether the current DEP is a .git based
	// DEP or a CIPD based dep using the existing deps_parser, so we filter by
	// whether the Id has ".com" to differentiate. This likely needs refinement.
	for id, depsEntry := range entries {
		if !strings.Contains(id, ".com") {
			continue
		}
		// We want it in https://{DepsEntry.Id} format, without the .git
		u, _ := url.JoinPath("https://", id)
		denormalized[u] = depsEntry.Version
	}

	return denormalized, nil
}

// findRolledDep searches for the dependency that may have been rolled.
func (m *midpointHandler) findRolledDep(startDeps, endDeps map[string]string) string {
	for k, v := range startDeps {
		// If the dep doesn't exist, it couldn't have been rolled. Skip.
		if _, ok := endDeps[k]; !ok {
			continue
		}
		if v != endDeps[k] {
			return k
		}
	}

	return ""
}

// determineRolledDep coordinates the search to find which dep may have been rolled for adjacent commits.
func (m *midpointHandler) determineRolledDep(ctx context.Context, url, startGitHash, endGitHash string) (*Commit, *Commit, *Commit, error) {
	gc := m.getOrCreateRepo(url)

	// Fetch deps for each git hash for the project
	startDeps, err := m.fetchGitDeps(ctx, gc, startGitHash)
	if err != nil {
		return nil, nil, nil, err
	}

	endDeps, err := m.fetchGitDeps(ctx, gc, endGitHash)
	if err != nil {
		return nil, nil, nil, err
	}

	// Find the delta.
	diff := m.findRolledDep(startDeps, endDeps)

	// DEPS are the same.
	if diff == "" {
		return nil, nil, nil, nil
	}

	dStart := startDeps[diff]
	left := &Commit{
		RepositoryUrl: diff,
		GitHash:       dStart,
	}
	dEnd := endDeps[diff]
	right := &Commit{
		RepositoryUrl: diff,
		GitHash:       dEnd,
	}

	dNext, err := m.findMidpoint(ctx, diff, dStart, dEnd)
	if err != nil {
		return nil, nil, nil, err
	}
	next := &Commit{
		RepositoryUrl: diff,
		GitHash:       dNext,
	}

	return next, left, right, nil
}

// DetermineNextCandidate finds the next commit for culprit detection for the repository inbetween the provided starting and ending git hash.
// If the starting and ending git hashes are adjacent to each other, and if a DEPS roll has taken place, DetermineNextCandidate will search
// the rolled repository for the next culprit and return information about the roll and the next commit in the Dependency, which should be built
// on top of the Chromium commit specified as a deps override.
func (m *midpointHandler) DetermineNextCandidate(ctx context.Context, baseUrl, startGitHash, endGitHash string) (*combinedCommit, *CommitRange, error) {
	nextCommitHash, err := m.findMidpoint(ctx, baseUrl, startGitHash, endGitHash)
	if err != nil {
		return nil, nil, err
	}

	base := NewCombinedCommit(
		&Commit{
			RepositoryUrl: baseUrl,
			GitHash:       nextCommitHash,
		},
	)

	// We use HasPrefix because nextCommitHash will always be the full SHA git hash,
	// but the provided startGitHash may be a short SHA.
	if nextCommitHash != "" && strings.HasPrefix(nextCommitHash, startGitHash) {
		// The nextCommit == startHash. This means start and end are adjacent commits.
		// Assume a DEPS roll, so we'll find the next candidate by parsing DEPS rolls.
		sklog.Debugf("Start hash %s and end hash %s are adjacent to each other. Assuming a DEPS roll.", startGitHash, endGitHash)

		next, left, right, err := m.determineRolledDep(ctx, baseUrl, startGitHash, endGitHash)
		if err != nil {
			return nil, nil, err
		}

		if next != nil {
			base.ModifiedDeps = []*Commit{
				next,
			}
		}

		if left != nil || right != nil {
			cr := &CommitRange{}

			if left != nil {
				cr.Left = NewCombinedCommit(
					&Commit{
						RepositoryUrl: baseUrl,
						GitHash:       nextCommitHash,
					},
					left,
				)
			}

			if right != nil {
				cr.Right = NewCombinedCommit(
					&Commit{
						RepositoryUrl: baseUrl,
						GitHash:       nextCommitHash,
					},
					right,
				)
			}

			return base, cr, nil
		}
	}

	return base, nil, nil
}
