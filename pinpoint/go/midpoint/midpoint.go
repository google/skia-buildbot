package midpoint

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/pinpoint/go/common"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	GitilesEmptyResponseErr = "Gitiles returned 0 commits, which should not happen."
	ChromiumSrcGit          = "https://chromium.googlesource.com/chromium/src.git"
)

// CommitRange provides information about the left and right commits used to determine
// the next commit to bisect against.
type CommitRange struct {
	Left  *common.CombinedCommit
	Right *common.CombinedCommit
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
func (m *MidpointHandler) findMidpoint(ctx context.Context, startCommit, endCommit *pb.Commit) (*pb.Commit, error) {
	startGitHash, endGitHash := startCommit.GetGitHash(), endCommit.GetGitHash()
	url := startCommit.Repository

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

	// For even lists, we opt to the higher index (ie/ in [4, 3, 2, 1] len == 4 and midpoint
	// becomes index 2 (which = 2))
	mlc := lc[len(lc)/2]

	nextHash := mlc.ShortCommit.Hash
	sklog.Debugf("Next midpoint commit: %s", nextHash)
	return &pb.Commit{
		Repository: url,
		GitHash:    nextHash,
	}, nil
}

// fetchGitDeps fetches all the git-based dependencies as a repo-Commit map.
func (m *MidpointHandler) fetchGitDeps(ctx context.Context, commit *pb.Commit) (map[string]*pb.Commit, error) {
	denormalized := make(map[string]*pb.Commit, 0)

	gc := m.getOrCreateRepo(commit.Repository)
	content, err := gc.ReadFileAtRef(ctx, "DEPS", commit.GitHash)
	if err != nil {
		// Even if the provided http client is provided without With2xxOnly,
		// gitiles.go get() enforces http.StatusOK and returns a nil response
		// with this error.
		if strings.Contains(err.Error(), "404 Not Found") {
			sklog.Debugf("gitiles.ReadFileAtRef returned 404 for DEPS file %s@%s", commit.Repository, commit.GitHash)
			return denormalized, nil
		}
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
		denormalized[u] = &pb.Commit{
			Repository: u,
			GitHash:    depsEntry.Version,
		}
	}

	return denormalized, nil
}

// findMidCommitInDEPS finds the median git hash from the delta of the DEPS contents at both commits.
func (m *MidpointHandler) findMidCommitInDEPS(ctx context.Context, startCommit, endCommit *pb.Commit) (*pb.Commit, error) {
	if startCommit.Repository != endCommit.Repository {
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
	if len(startDeps) < 1 || len(endDeps) < 1 {
		sklog.Debugf("DEPS does not exist at both %v and %v so no midpoint is identifiable", startCommit, endCommit)
		return nil, nil
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

	// Note: This should assume another DEPS roll and look for the next midpoint there,
	// but it currently terminate at layer - 1 and returns startCommit as the midpoint
	// for two adjancet changes.
	if strings.HasPrefix(dMid.GitHash, dStart.GitHash) {
		sklog.Debugf("Returning startCommit because the two commits %v and %v, parsed from DEPS files at %v and %v respectively, are adjacent.", dStart, dEnd, startCommit, endCommit)
		return nil, nil
	}

	sklog.Debugf("Next modified dep: %v", dMid)
	return dMid, nil
}

// findDepsCommit finds the commit in the DEPS for the target repo.
//
// In other words, it fetches the DEPS file at baseCommit, and finds the git hash for targetRepoUrl.
// It returns a Commit that can be used to search for middle commit in the DEPS and then construct
// a CombinedCommit to build Chrome with modified DEPS.
func (m *MidpointHandler) findDepsCommit(ctx context.Context, baseCommit *pb.Commit, targetRepoUrl string) (*pb.Commit, error) {
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
// For example:
//
//	{C@1} vs {C@1, V8@2, WRT@3, Blink@4 ..., devtools@5} would backfill as such:
//	  * V8 info would be filled from C1
//	  * WRT would be filled from V8 above
//	  * Blink would be filled from WRT above.
//	... and so on.
func (m *MidpointHandler) fillModifiedDeps(ctx context.Context, start, end *common.CombinedCommit) error {
	if len(end.ModifiedDeps) > len(start.ModifiedDeps) {
		start, end = end, start
	}
	for len(start.ModifiedDeps) > len(end.ModifiedDeps) {
		// if we are at start=(C@1, V8@2, WRT@3) and end=(C@1, V8@1)
		// compared against, we want to fetch WRT's commit hash from V8@1
		// start.ModifiedDeps == [V8@2, WRT@3, WebRTC@4] and end.ModifiedDeps == [V8@1]
		// the target dependency is WRT, index == 1 == len(end.ModifiedDeps)
		targetDepRepoUrl := start.ModifiedDeps[len(end.ModifiedDeps)].Repository

		// if we are comparing two combined commits, where one has modified deps and the other
		// does not, we need to start filling modified deps using the base commit (or main),
		// which in most cases is chromium/src. for example, following the example above,
		// if we have start=(Main:C@1, Deps:V8@2, WRT@3) and end=(Main:C@1), we need to
		// fill end's deps starting from C@1 (which is main).
		refCommit := end.Main
		if len(end.ModifiedDeps) > 0 {
			refCommit = end.GetLatestModifiedDep()
		}
		endDepCommit, err := m.findDepsCommit(ctx, refCommit, targetDepRepoUrl)
		if err != nil {
			return err
		}

		end.ModifiedDeps = append(end.ModifiedDeps, endDepCommit)
	}

	return nil
}

// findMidCommit coordinates the search for finding the midpoint between the two commits.
// findMidCommit assumes that it's operating within the same repo. Care is required if
// findMidCommit is used to find the midCommit between commits from two different repos.
// See doc.go for edge cases.
func (m *MidpointHandler) findMidCommit(ctx context.Context, startCommit, endCommit *pb.Commit) (*pb.Commit, error) {
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

	// If there was no DEPS roll, the midpoint between two adjacent commits
	// is the start commit.
	//
	// Note: when findMidCommitInDEPS() finds two adjacent git-based dependencies from
	// DEPS files, it should traverse deeper also assuming a DEPS roll. As of right now,
	// it terminates by returning nil, meaning that it only goes in layer - 1.
	// And because we aren't traversing any further, this is being treated as a termination
	// clause saying there's no midpoint.
	if midCommitFromDEPS == nil {
		sklog.Debugf("There are no more commits to parse through in the DEP between %v and %v", startCommit, endCommit)
		return startCommit, nil
	}

	sklog.Debugf("Next midpoint found through DEPS: %v", midCommitFromDEPS)
	return midCommitFromDEPS, nil
}

// Equal takes two combined commits and returns whether they are equal.
//
// Modified deps affects the equality of two combined commits. If the length of
// both modified deps are not equal between first and second, this check will
// backfill modified deps information from DEPS files such that they are equal
// before calculating and comparing the key.
func (m *MidpointHandler) Equal(ctx context.Context, first, second *common.CombinedCommit) (bool, error) {
	err := m.fillModifiedDeps(ctx, first, second)
	if err != nil {
		return false, skerr.Fmt("Failed to sync modified deps for both commits.")
	}

	return first.Key() == second.Key(), nil
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
func (m *MidpointHandler) FindMidCombinedCommit(ctx context.Context, startCommit, endCommit *common.CombinedCommit) (*common.CombinedCommit, error) {
	if startCommit.Key() == endCommit.Key() {
		return nil, skerr.Fmt("Unable to find midpoint between two commits that are identical")
	}
	if startCommit.Main.Repository != endCommit.Main.Repository {
		return nil, skerr.Fmt("Unable to find midpoint between two commits with different main repositories.")
	}

	// Commits with modified deps defined indicates that the main repository has
	// already been investigated and that we've reached a point where two adjacent
	// commits have been compared (where DEPS is analyzed). We search for the
	// midpoint from modified dep where commits for it differ.
	if len(startCommit.ModifiedDeps) > 0 || len(endCommit.ModifiedDeps) > 0 {
		// Create originals of the start commits before the deps are filled in.
		// These originals are needed before DEPS are filled in and they are edited.
		origStartCommit, origEndCommit := startCommit.Clone(), endCommit.Clone()
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

		startDep := startCommit.GetLatestModifiedDep()
		endDep := endCommit.GetLatestModifiedDep()

		// Checks if we are looking at the first or last commit in the DEPS roll.
		// If we reach a state where {C@1, V8@4) is compared to {C@2, V8@4},
		// and C@2 was a deps roll to V8@4, an equality check on the two objects
		// by key wouldn't work because of the difference in base.
		if startDep.GetGitHash() == endDep.GetGitHash() {
			sklog.Warningf("Both start and end DEPS are identical. Either startDep is the last commit in the DEPS roll or endDep is the first commit in the DEPS roll. Return original startCommit: %v. startDep: %v; endDep: %v", origStartCommit, startDep, endDep)
			return origStartCommit, nil
		}

		midDepCommit, err := m.findMidCommit(ctx, startDep, endDep)
		if err != nil {
			return nil, err
		}

		// Addresses the edge case if we are comparing the 2nd to last commit
		// in the roll so that the midpoint is the last commit in the roll.
		// i.e. {C@1, V8@3} vs {C@2} fills deps to {C@1, V8@3} vs {C@2, V8@4}
		// and returns {C@1, V8@4}
		if strings.HasPrefix(midDepCommit.GitHash, startDep.GitHash) && len(origStartCommit.ModifiedDeps) > len(origEndCommit.ModifiedDeps) {
			resp := startCommit.Clone()
			resp.UpsertModifiedDep(endDep)
			return resp, nil
		}
		// There is no mid to proceed on.
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

	// There is no mid.
	if strings.HasPrefix(midCommit.GitHash, startCommit.GetMainGitHash()) {
		return startCommit, nil
	}

	// Mid commit is through Main, so update that.
	if midCommit.Repository == startCommit.Main.Repository {
		return common.NewCombinedCommit(midCommit), nil
	}

	// Add this dependency to modified deps.
	resp := startCommit.Clone()
	resp.UpsertModifiedDep(midCommit)
	return resp, nil
}
