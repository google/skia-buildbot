package midpoint

import (
	"context"
	"hash/fnv"
	"math"
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
	GitilesEmptyResponseErr = "Gitiles returned 0 commits, which should not happen."
	chromiumSrcGit          = "https://chromium.googlesource.com/chromium/src.git"
)

// A Commit represents a commit of a given repository.
// TODO(jeffyoon@) - Reorganize this into a types folder.
type Commit struct {
	// GitHash is the Git SHA1 hash to build for the project.
	GitHash string

	// RepositoryUrl is the url to the repository, ie/ https://chromium.googlesource.com/chromium/src
	RepositoryUrl string
}

func NewChromiumCommit(h string) *Commit {
	return &Commit{
		GitHash:       h,
		RepositoryUrl: chromiumSrcGit,
	}
}

// A CombinedCommit represents one main base commit with any dependencies that require
// overrides as part of the build request.
// For example, if Commit is chromium/src@1, Dependency may be V8@2 which is passed
// along to Buildbucket as a deps_revision_overrides.
type CombinedCommit struct {
	// Main is the main base commit, usually a Chromium commit.
	Main *Commit
	// ModifiedDeps is a list of commits by repository url to provide as overrides, ie/ V8.
	ModifiedDeps ModifiedDeps
}

type ModifiedDeps []*Commit

// GetLatest returns the most recently added commit.
func (m ModifiedDeps) GetLatest() *Commit {
	return m[len(m)-1]
}

// TODO(jeffyoon@) - move this to a deps folder, likely with the types restructure above.
// DepsToMap translates all deps into a map.
func (cc *CombinedCommit) DepsToMap() map[string]string {
	resp := make(map[string]string, 0)
	for _, c := range cc.ModifiedDeps {
		resp[c.RepositoryUrl] = c.GitHash
	}
	return resp
}

// GetMainGitHash returns the git hash of main.
func (cc *CombinedCommit) GetMainGitHash() string {
	if cc.Main == nil {
		return ""
	}

	return cc.Main.GitHash
}

// Key returns all git hashes combined to use for map indexing
func (cc *CombinedCommit) Key() uint32 {
	h := fnv.New32a()

	if cc.Main == nil {
		return h.Sum32()
	}

	h.Write([]byte(cc.Main.GitHash))
	if cc.ModifiedDeps == nil {
		return h.Sum32()
	}

	for _, v := range cc.ModifiedDeps {
		h.Write([]byte(v.GitHash))
	}

	return h.Sum32()
}

// Clone returns a copy of this combined commit.
func (cc *CombinedCommit) Clone() *CombinedCommit {
	if cc.Main == nil {
		return &CombinedCommit{}
	}
	newCombinedCommit := &CombinedCommit{
		Main: &Commit{
			RepositoryUrl: cc.Main.RepositoryUrl,
			GitHash:       cc.Main.GitHash,
		},
	}

	if cc.ModifiedDeps != nil {
		newModDeps := make([]*Commit, len(cc.ModifiedDeps))
		copy(newModDeps, cc.ModifiedDeps)
		newCombinedCommit.ModifiedDeps = newModDeps
	}

	return newCombinedCommit
}

// UpsertModifiedDep inserts or updates a commit to ModifiedDeps
func (cc *CombinedCommit) UpsertModifiedDep(commit *Commit) {
	// This operation is O(n) but is bound worst case by min(the number of
	// git-based dependencies a repository supports, bisection iterations)
	// so this should be okay. At the time of implementation, there are ~250
	// git-based repositories, and Catapult supports 30 bisection iterations,
	// so O(30).
	if cc.ModifiedDeps == nil {
		cc.ModifiedDeps = []*Commit{commit}
		return
	}
	for _, mc := range cc.ModifiedDeps {
		if mc.RepositoryUrl == commit.RepositoryUrl {
			mc.GitHash = commit.GitHash
			return
		}
	}

	cc.ModifiedDeps = append(cc.ModifiedDeps, commit)
	return
}

// NewCombinedCommit returns a new CombinedCommit object.
func NewCombinedCommit(main *Commit, deps ...*Commit) *CombinedCommit {
	return &CombinedCommit{
		Main:         main,
		ModifiedDeps: deps,
	}
}

// CommitRange provides information about the left and right commits used to determine
// the next commit to bisect against.
type CommitRange struct {
	Left  *CombinedCommit
	Right *CombinedCommit
}

// MidpointHandler encapsulates all logic to determine the next potential candidate for Bisection.
type MidpointHandler struct {
	// repos is a map of repository url to a GitilesRepo object.
	repos map[string]gitiles.GitilesRepo

	c *http.Client
}

// New returns a new MidpointHandler.
func New(ctx context.Context, c *http.Client) *MidpointHandler {
	return &MidpointHandler{
		repos: make(map[string]gitiles.GitilesRepo, 0),
		c:     c,
	}
}

// WithRepo returns a MidpointHandler with the repository url mapped to a GitilesRepo object.
func (m *MidpointHandler) WithRepo(url string, r gitiles.GitilesRepo) *MidpointHandler {
	m.repos[url] = r
	return m
}

// getOrCreateRepo fetches the gitiles.GitilesRepo object for the repository url.
// If not present, it'll create an authenticated Repo client.
func (m *MidpointHandler) getOrCreateRepo(url string) gitiles.GitilesRepo {
	gr, ok := m.repos[url]
	if !ok {
		gr = gitiles.NewRepo(url, m.c)
		m.repos[url] = gr
	}
	return gr
}

// findMidpoint finds the median commit between two commits.
func (m *MidpointHandler) findMidpoint(ctx context.Context, startCommit, endCommit *Commit) (*Commit, error) {
	startGitHash, endGitHash := startCommit.GitHash, endCommit.GitHash
	url := startCommit.RepositoryUrl

	if startGitHash == endGitHash {
		return nil, skerr.Fmt("Both git hashes are the same; Start: %s, End: %s", startGitHash, endGitHash)
	}

	gc := m.getOrCreateRepo(url)

	// Find the midpoint between the provided commit hashes. Take the lower bound
	// if the list is an odd count. If the gitiles response is == endGitHash, it
	// this means both start and end are adjacent, and DEPS needs to be unravelled
	// to find the potential culprit.
	// LogFirstParent will return in reverse chronological order, inclusive of the end git hash.
	lc, err := gc.LogFirstParent(ctx, startGitHash, endGitHash)
	if err != nil {
		return nil, err
	}

	// The list can only be empty if the start and end commits are the same.
	if len(lc) == 0 {
		return nil, skerr.Fmt("%s. Start %s and end %s hashes may be reversed.", GitilesEmptyResponseErr, startGitHash, endGitHash)
	}

	// Two adjacent commits returns one commit equivalent to the end git hash.
	if len(lc) == 1 && lc[0].ShortCommit.Hash == endGitHash {
		return startCommit, nil
	}

	// Pop off the first element, since it's the end hash.
	// Golang divide will return lower bound when odd.
	lc = lc[1:]

	// Sort to chronological order before taking the midpoint. This means for even
	// lists, we opt to the higher index (ie/ in [1,2,3,4] len == 4 and midpoint
	// becomes index 2 (which = 3))
	slices.Reverse(lc)
	mlc := lc[len(lc)/2]

	nextHash := mlc.ShortCommit.Hash
	sklog.Debugf("Next midpoint commit: %s", nextHash)
	return &Commit{
		RepositoryUrl: url,
		GitHash:       nextHash,
	}, nil
}

// fetchGitDeps fetches all the git-based dependencies as a repo-Commit map.
func (m *MidpointHandler) fetchGitDeps(ctx context.Context, commit *Commit) (map[string]*Commit, error) {
	denormalized := make(map[string]*Commit, 0)

	gc := m.getOrCreateRepo(commit.RepositoryUrl)
	content, err := gc.ReadFileAtRef(ctx, "DEPS", commit.GitHash)
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
		denormalized[u] = &Commit{
			RepositoryUrl: u,
			GitHash:       depsEntry.Version,
		}
	}

	return denormalized, nil
}

// findMidCommitInDEPS finds the median git hash from the delta of the DEPS contents at both commits.
func (m *MidpointHandler) findMidCommitInDEPS(ctx context.Context, startCommit, endCommit *Commit) (*Commit, error) {
	if startCommit.RepositoryUrl != endCommit.RepositoryUrl {
		return nil, skerr.Fmt("two commits are from different repos and deps cannot be compared")
	}
	// Fetch deps for each git hash for the project
	startDeps, err := m.fetchGitDeps(ctx, startCommit)
	if err != nil {
		return nil, err
	}
	endDeps, err := m.fetchGitDeps(ctx, endCommit)
	if err != nil {
		return nil, err
	}

	// As part of a roll, some git-based dependencies can be removed.
	// If it doesn't exist, it can't have been rolled, so it's skipped.
	diffUrl := ""
	for url, sc := range startDeps {
		// If the dep doesn't exist, it couldn't have been rolled. Skip.
		ed, ok := endDeps[url]
		if !ok {
			continue
		}
		if sc.GitHash != ed.GitHash {
			diffUrl = url
			break
		}
	}
	if diffUrl == "" {
		sklog.Debugf("A DEPS roll was not identifiable from %v to %v", startCommit, endCommit)
		return nil, nil
	}

	dStart := startDeps[diffUrl]
	dEnd := endDeps[diffUrl]

	dMid, err := m.findMidpoint(ctx, dStart, dEnd)
	if err != nil {
		return nil, err
	}

	// The Gitiles response could've been empty, which occurs when the start
	// and end commits are the same.
	// This should not happen given the previous checks.
	if dMid.GitHash == "" {
		return nil, skerr.Fmt("The two commits %v and %v were the same while comparing deps files between %v and %v", dStart, dEnd, startCommit, endCommit)
	}

	if strings.HasPrefix(dMid.GitHash, dStart.GitHash) {
		sklog.Debugf("Returning startCommit because the two commits %v and %v, parsed from DEPS files at %v and %v respectively, are adjacent.", dStart, dEnd, startCommit, endCommit)
		return startCommit, nil
	}

	sklog.Debugf("Next modified dep: %v", dMid)
	return dMid, nil
}

// findDepsCommit finds the commit in the DEPS for the target repo.
//
// In other words, it fetches the DEPS file at baseCommit, and finds the git hash for targetRepoUrl.
// It returns a Commit that can be used to search for middle commit in the DEPS and then construct
// a CombinedCommit to build Chrome with modified DEPS.
func (m *MidpointHandler) findDepsCommit(ctx context.Context, baseCommit *Commit, targetRepoUrl string) (*Commit, error) {
	deps, err := m.fetchGitDeps(ctx, baseCommit)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	commit, ok := deps[targetRepoUrl]
	if !ok {
		return nil, skerr.Fmt("%s doesn't exist in DEPS", targetRepoUrl)
	}

	return commit, nil
}

// fillModifiedDeps ensures that both start and end have the same modified deps defined, each filling their content from their own DEPS files respectively.
// This function will modify ModifiedDeps for both start and end.
//
// Note: See comment in FindMidCombinedCommit().
//
//	This method will need to be updated to fill all missing modified deps.
//	{C@1} vs {C@1, V8@2, WRT@3, Blink@4 ..., devtools@5} we would actually need to backfill as such:
//	  * V8 info would be filled from C1
//	  * WRT would be filled from V8 above
//	  * Blink would be filled from WRT above.
//	... and so on.
func (m *MidpointHandler) fillModifiedDeps(ctx context.Context, start, end *CombinedCommit) error {
	if len(end.ModifiedDeps) > len(start.ModifiedDeps) {
		targetDepRepoUrl := end.ModifiedDeps.GetLatest().RepositoryUrl

		refCommit := start.Main
		if len(start.ModifiedDeps) > 0 {
			refCommit = start.ModifiedDeps.GetLatest()
		}
		smd, err := m.findDepsCommit(ctx, refCommit, targetDepRepoUrl)
		if err != nil {
			return err
		}

		start.ModifiedDeps = append(start.ModifiedDeps, smd)
	} else if len(start.ModifiedDeps) > len(end.ModifiedDeps) {
		targetDepRepoUrl := start.ModifiedDeps.GetLatest().RepositoryUrl

		refCommit := end.Main
		if len(end.ModifiedDeps) > 0 {
			refCommit = end.ModifiedDeps.GetLatest()
			sklog.Errorf("ref commit modified to %v", refCommit)
		}
		emd, err := m.findDepsCommit(ctx, refCommit, targetDepRepoUrl)
		if err != nil {
			return err
		}

		end.ModifiedDeps = append(end.ModifiedDeps, emd)
	}

	return nil
}

// findMidCommit coordinates the search for finding the midpoint between the two commits.
func (m *MidpointHandler) findMidCommit(ctx context.Context, startCommit, endCommit *Commit) (*Commit, error) {
	midCommit, err := m.findMidpoint(ctx, startCommit, endCommit)
	if err != nil {
		return nil, err
	}

	// If the calculated midpoint != start git hash, they are not adjacent,
	// so return the found commit right away.
	//
	// We use HasPrefix because nextCommitHash will always be the full SHA git hash,
	// but the provided startGitHash may be a short SHA.
	if !strings.HasPrefix(midCommit.GitHash, startCommit.GitHash) {
		sklog.Debugf("Next midpoint: %v", midCommit)
		return midCommit, nil
	}

	// The nextCommit == startHash, which means start and end are adjacent commits.
	// Assume a DEPS roll, so we'll find the next candidate by parsing DEPS rolls.
	sklog.Debugf("Start %v and end %v are adjacent to each other. Assuming a DEPS roll.", startCommit, endCommit)

	// Now we parse DEPS files at each start and end commits and identify which repository was rolled.
	// From that range, we can search for the midpoint.
	midCommitFromDEPS, err := m.findMidCommitInDEPS(ctx, startCommit, endCommit)
	if err != nil {
		return nil, err
	}

	if midCommitFromDEPS == nil {
		// DEPS were equal.
		return nil, nil
	}

	if strings.HasPrefix(midCommitFromDEPS.GitHash, startCommit.GitHash) {
		return nil, skerr.Fmt("There are no more commits to parse through in the DEP between %v and %v", startCommit, endCommit)
	}

	sklog.Debugf("Next midpoint found through DEPS: %v", midCommitFromDEPS)
	return midCommitFromDEPS, nil
}

// FindMidCommit finds the middle commit from the two given commits.
//
// It uses gitiles API to find the middle commit, and it also handles DEPS rolls when two commits
// are adjacent. If two commits are adjacent and no DEPS roll, then the first commit is returned;
// If two commits are adjacent and there is a DEPS roll on the second commit, then it will search
// for rolled repositories and find the middle commit between the roll.
//
// Note the returned Commit can be a different repo because it looks at DEPS, but it only looks at
// one level. If the DEPS of DEPS has rolls, it will not continue to search.
//
// TODO(b/326352320) remove this once it's usage is updated in pinpoint/pinpoint.go
func (m *MidpointHandler) FindMidCommit(ctx context.Context, startCommit, endCommit *Commit) (*Commit, error) {
	if startCommit.RepositoryUrl != endCommit.RepositoryUrl {
		return nil, skerr.Fmt("two commits are from different repos")
	}

	nextCommit, err := m.findMidpoint(ctx, startCommit, endCommit)
	if err != nil {
		return nil, err
	}

	// If startGitHash and endGitHash are not adjacent, return the found commit right away.
	//
	// We use HasPrefix because nextCommitHash will always be the full SHA git hash,
	// but the provided startGitHash may be a short SHA.
	if !strings.HasPrefix(nextCommit.GitHash, startCommit.GitHash) {
		return nextCommit, nil
	}

	// The nextCommit == startHash. This means start and end are adjacent commits.
	// Assume a DEPS roll, so we'll find the next candidate by parsing DEPS rolls.
	sklog.Debugf("Start hash %s and end hash %s are adjacent to each other. Assuming a DEPS roll.", startCommit.GitHash, endCommit.GitHash)

	midDepCommit, err := m.findMidCommitInDEPS(ctx, startCommit, endCommit)
	if err != nil {
		return nil, err
	}

	// If endGitHash doesn't have DEPS rolls, return the first commit.
	if midDepCommit == nil {
		return startCommit, nil
	}

	return midDepCommit, nil
}

// FindMidCombinedCommit searches for the median commit between two combined commits.
//
// The search takes place through Main if no ModifiedDeps are present.
// When ModifiedDeps are defined, it first searches for the repository that has different git hashes.
// It then uses those two git hashes as the range to determine the median.
//
// In both scenarios, if the two commits are adjacent, a DEPS roll is assumed. This will
// parse the content of DEPS files at the two commits and try to look for which git-based dependency
// might've been rolled. Once identified, it searches for a median from the base to rolled git hash.
//
// See midpoint/doc.go for examples and details.
func (m *MidpointHandler) FindMidCombinedCommit(ctx context.Context, startCommit, endCommit *CombinedCommit) (*CombinedCommit, error) {
	if startCommit.Main.RepositoryUrl != endCommit.Main.RepositoryUrl {
		return nil, skerr.Fmt("Unable to find midpoint between two commits with different main repositories.")
	}

	// Commits with modified deps defined indicates that the main repository has
	// already been investigated and that we've reached a point where two adjacent
	// commits have been compared (where DEPS is analyzed). We search for the
	// midpoint from modified dep where commits for it differ.
	if len(startCommit.ModifiedDeps) > 0 || len(endCommit.ModifiedDeps) > 0 {
		// Note: This will not support the scenario where we have dove into several deps and that
		// commit needs to be compared against the original starting commit.
		// For example, if we started with C@1 vs C@3, and through iteration hit
		// C@1 vs. (C@1, V8@1, WebRTC@1), this clause will hit and backfill won't happen.
		if math.Abs(float64(len(startCommit.ModifiedDeps)-len(endCommit.ModifiedDeps))) > 1 {
			return nil, skerr.Fmt("The implementation assumes one repo change per roll, and this backfill doesn't fill iteratively.")
		}
		// During bisection, either one of start or end combined commits may be missing
		// a modified deps definition.
		//
		// For example, if we have two Chromium commits 1 and 2 (which are adjacent),
		// a DEPS roll is assumed. If Chromium@1 had a dependency to V8@3, and
		// Chromium@2 rolled V8 to 5, the calculated mid combined commit would be
		// {Chromium@1, V8@4}.
		//
		// The next set of comparisons would thus be C@1 vs. {C@1, V8@4} and
		// {C@1, V8@4} vs. C@2. However, when these combined commits are passed here,
		// they may be missing explicit definitions (ie/ C@1 would not explicitly
		// define {C@1, V8@3}, even though it built V8@3. To make an equal comparison,
		// this logic here fills in V8@3 for C1.
		//
		// Same goes for {C@1, V8@4, WRT@11} and {C@1, V8@5}. This would fill WRT
		// to whatever value WRT was at V8@5.
		err := m.fillModifiedDeps(ctx, startCommit, endCommit)
		if err != nil {
			return nil, skerr.Fmt("Failed to sync modified deps for both commits.")
		}

		startDep := startCommit.ModifiedDeps.GetLatest()
		endDep := endCommit.ModifiedDeps.GetLatest()

		midDepCommit, err := m.findMidCommit(ctx, startDep, endDep)
		if err != nil {
			return nil, err
		}

		// If startDep is returned, indicator that all options have been exhausted,
		// so return the start commit.
		if strings.HasPrefix(midDepCommit.GitHash, startDep.GitHash) {
			return startCommit, nil
		}

		resp := startCommit.Clone()
		resp.UpsertModifiedDep(midDepCommit)
		return resp, nil
	}

	// No modified deps, so we search midpoint based on the main commit.
	sklog.Debugf("No ModifiedDeps, searching for midpoint in Chromium betwen %s and %s", startCommit.Main.GitHash, endCommit.Main.GitHash)
	midCommit, err := m.findMidCommit(ctx, startCommit.Main, endCommit.Main)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(midCommit.GitHash, startCommit.Main.GitHash) {
		return startCommit, nil
	}

	// Mid commit is through Main, so update that.
	if midCommit.RepositoryUrl == startCommit.Main.RepositoryUrl {
		return NewCombinedCommit(midCommit), nil
	}

	// Add this dependency to modified deps.
	resp := startCommit.Clone()
	resp.UpsertModifiedDep(midCommit)
	return resp, nil
}
