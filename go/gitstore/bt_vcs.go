package gitstore

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

const (
	// EV_NEW_GIT_COMMIT is the event that is fired when a previously unseen Git commit is available.
	// The event data for this commit is of type []*vcsinfo.IndexCommit containing all new commits
	// that have been added since the last commit was sent.
	EV_NEW_GIT_COMMIT = "gitstore:new-git-commit"

	// defaultWatchInterval is the interval at which we check for new commits being added to the repo.
	defaultWatchInterval = time.Second * 10
)

// btVCS implements the vcsinfo.VCS interface based on a BT-backed GitStore.
type btVCS struct {
	gitStore           GitStore
	repo               *gitiles.Repo
	defaultBranch      string
	secondaryVCS       vcsinfo.VCS
	secondaryExtractor depot_tools.DEPSExtractor

	branchInfo   *BranchPointer
	indexCommits []*vcsinfo.IndexCommit
	hashes       []string
	timestamps   map[string]time.Time           //
	detailsCache map[string]*vcsinfo.LongCommit // Details
	mutex        sync.RWMutex
}

// NewVCS returns an instance of vcsinfo.VCS that is backed by the given GitStore and uses the
// gittiles.Repo to retrieve files. Each instance provides an interface to one branch.
// If defaultBranch is "" all commits in the repository are considered.
// If evt is not nil and nCommits > 0 then this instance will continuously track
// the last nCommits and publish a EV_NEW_GIT_COMMIT event.
// The instances of gitiles.Repo is only used to fetch files.
func NewVCS(gitStore GitStore, defaultBranch string, repo *gitiles.Repo, evt eventbus.EventBus, nCommits int) (vcsinfo.VCS, error) {
	ret := &btVCS{
		gitStore:      gitStore,
		repo:          repo,
		defaultBranch: defaultBranch,
	}
	if err := ret.Update(context.TODO(), true, false); err != nil {
		return nil, err
	}

	// Start watching the repo for changes and fire events when commits change.
	if evt != nil && nCommits > 0 {
		startVCSTracker(gitStore, defaultWatchInterval, evt, defaultBranch, nCommits)
	}
	return ret, nil
}

// GetBranch implements the vcsinfo.VCS interface.
func (b *btVCS) GetBranch() string {
	return b.defaultBranch
}

// SetSecondaryRepo allows to add a secondary repository and extractor to this instance.
// It is not included in the constructor since it is currently only used by the Gold ingesters.
func (b *btVCS) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	b.secondaryVCS = secVCS
	b.secondaryExtractor = extractor
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) Update(ctx context.Context, pull, allBranches bool) error {
	// Check if we need to pull across all branches.
	targetBranch := b.defaultBranch
	if allBranches {
		targetBranch = ""
	}

	// Simulate a pull by fetching the latest head of the target branch.
	if pull {
		branchHeads, err := b.gitStore.GetBranches(ctx)
		if err != nil {
			return err
		}

		var ok bool
		b.branchInfo, ok = branchHeads[targetBranch]
		if !ok {
			return skerr.Fmt("Unable to find branch %q in BitTable repo %s", targetBranch, (b.gitStore.(*btGitStore)).repoURL)
		}
	}

	// Get all index commits for the current branch.
	return b.fetchIndexRange(ctx, 0, b.branchInfo.Index+1)
}

// From implements the vcsinfo.VCS interface
func (b *btVCS) From(start time.Time) []string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Add a millisecond because we only want commits after the startTime. Timestamps in git are
	// only at second level granularity.
	found := b.timeRange(start.Add(time.Millisecond), vcsinfo.MaxTime)
	ret := make([]string, len(found))
	for i, c := range found {
		ret[i] = c.Hash
	}
	return ret
}

// Details implements the vcsinfo.VCS interface
func (b *btVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.details(ctx, hash, includeBranchInfo)
}

// DetailsMulti implements the vcsinfo.VCS interface
func (b *btVCS) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	commits, err := b.gitStore.Get(ctx, hashes)
	if err != nil {
		return nil, err
	}

	if includeBranchInfo {
		branchPointers, err := b.gitStore.GetBranches(ctx)
		if err != nil {
			return nil, skerr.Fmt("Error retrieving branches: %s", err)
		}

		var egroup errgroup.Group
		for _, c := range commits {
			if c != nil {
				// Create a closure since we pass each value of 'c' to its own go-routine.
				func(c *vcsinfo.LongCommit) {
					egroup.Go(func() error {
						branches, err := b.getBranchInfo(ctx, c, branchPointers)
						if err != nil {
							return skerr.Fmt("Error getting branch info for commit %s: %s", c.Hash, err)
						}
						c.Branches = branches
						return nil
					})
				}(c)
			}
		}
		if err := egroup.Wait(); err != nil {
			return nil, err
		}
	}

	return commits, nil
}

// TODO(stephan): includeBranchInfo currently does nothing. This needs to fixed for the few clients
// that need it.

// details returns all meta data details we care about.
func (b *btVCS) details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	commits, err := b.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, skerr.Fmt("Commit %s not found", hash)
	}

	if includeBranchInfo {
		branchPointers, err := b.gitStore.GetBranches(ctx)
		if err != nil {
			return nil, skerr.Fmt("Error retrieving branches: %s", err)
		}

		branches, err := b.getBranchInfo(ctx, commits[0], branchPointers)
		if err != nil {
			return nil, skerr.Fmt("Error getting branch info for commit %s: %s", commits[0].Hash, err)
		}
		commits[0].Branches = branches
	}
	return commits[0], nil
}

// getBranchInfo determines which branches contain the given commit 'c'.
func (b *btVCS) getBranchInfo(ctx context.Context, c *vcsinfo.LongCommit, allBranches map[string]*BranchPointer) (map[string]bool, error) {
	ret := make(map[string]bool, len(allBranches))
	var mutex sync.Mutex
	var egroup errgroup.Group
	for branchName := range allBranches {
		if branchName != "" {
			func(branchName string) {
				egroup.Go(func() error {
					// Since we cannot look up a commit in a branch directly we query for all commits that
					// occurred at that specific timestamp (Git has second granularity) on the target branch.
					// Then we check whether the target commit is returned as part of the result.
					commits, err := b.gitStore.RangeByTime(ctx, c.Timestamp, c.Timestamp.Add(time.Second), branchName)
					if err != nil {
						return skerr.Fmt("Error in range query for branch %s: %s", branchName, err)
					}

					// Iterate over the commits at the given timestamp. Most of the time there should
					// only be one commit at a given one second time range.
					for _, idxCommit := range commits {
						if idxCommit.Hash == c.Hash {
							mutex.Lock()
							ret[branchName] = true
							mutex.Unlock()
							break
						}
					}
					return nil
				})
			}(branchName)
		}
	}
	if err := egroup.Wait(); err != nil {
		return nil, skerr.Fmt("Error retrieving branch membership: %s", err)
	}
	return ret, nil
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if N > len(b.indexCommits) {
		N = len(b.indexCommits)
	}
	ret := make([]*vcsinfo.IndexCommit, 0, N)
	return append(ret, b.indexCommits[len(b.indexCommits)-N:]...)
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) Range(begin, end time.Time) []*vcsinfo.IndexCommit {
	return b.timeRange(begin, end)
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) IndexOf(ctx context.Context, hash string) (int, error) {
	b.mutex.RLock()
	defer b.mutex.Unlock()

	for i := len(b.indexCommits) - 1; i >= 0; i-- {
		if hash == b.indexCommits[i].Hash {
			return b.indexCommits[i].Index, nil
		}
	}

	// If it was not in memory we need to fetch it
	details, err := b.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return 0, err
	}

	if len(details) == 0 {
		return 0, skerr.Fmt("Hash %s does not exist in repository on branch %s", hash, b.defaultBranch)
	}

	return 0, nil
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {
	// findFn returns the hash when N is within commits
	findFn := func(commits []*vcsinfo.IndexCommit) string {
		i := sort.Search(len(commits), func(i int) bool { return commits[i].Index >= N })
		return commits[i].Hash
	}

	var hash string
	b.mutex.RLock()
	if len(b.indexCommits) > 0 {
		firstIdx := b.indexCommits[0].Index
		lastIdx := b.indexCommits[len(b.indexCommits)-1].Index
		if (N >= firstIdx) && (N <= lastIdx) {
			hash = findFn(b.indexCommits)
		}
	}
	b.mutex.RUnlock()

	// Fetch the hash
	if hash == "" {
		return nil, fmt.Errorf("Hash index not found: %d", N)
	}
	return b.details(ctx, hash, false)
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	var buf bytes.Buffer
	if err := b.repo.ReadFileAtRef(fileName, commitHash, &buf); err != nil {
		return "", skerr.Fmt("Error reading file %s @ %s via gitiles: %s", fileName, commitHash, err)
	}
	return buf.String(), nil
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	if b.secondaryVCS == nil {
		return "", nil
	}

	foundCommit, err := b.secondaryExtractor.ExtractCommit(b.secondaryVCS.GetFile(ctx, "DEPS", commitHash))
	if err != nil {
		return "", err
	}
	return foundCommit, nil
}

// GetGitStore implements the gitstore.GitStoreBased interface
func (b *btVCS) GetGitStore() GitStore {
	return b.gitStore
}

// fetchIndexRange gets in the range [startIndex, endIndex).
func (b *btVCS) fetchIndexRange(ctx context.Context, startIndex, endIndex int) error {
	newIC, err := b.gitStore.RangeN(ctx, startIndex, endIndex, b.defaultBranch)
	if err != nil {
		return err
	}

	if len(newIC) == 0 {
		return nil
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.indexCommits = newIC
	return nil
}

func (b *btVCS) timeRange(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	n := len(b.indexCommits)
	startIdx := 0
	for ; startIdx < n; startIdx++ {
		exp := b.indexCommits[startIdx].Timestamp.After(start) || b.indexCommits[startIdx].Timestamp.Equal(start)
		if exp {
			break
		}
	}

	endIdx := startIdx
	for ; endIdx < n; endIdx++ {
		exp := b.indexCommits[endIdx].Timestamp.After(end) || b.indexCommits[endIdx].Timestamp.Equal(end)
		if exp {
			break
		}
	}

	if endIdx <= startIdx {
		return []*vcsinfo.IndexCommit{}
	}
	return b.indexCommits[startIdx:endIdx]
}

// startVCSTracker starts a background process that watches for new commits at the given interval.
// When a new commit is detected a EV_NEW_GIT_COMMIT event is triggered.
func startVCSTracker(gitStore GitStore, interval time.Duration, evt eventbus.EventBus, branch string, nCommits int) {
	ctx := context.TODO()
	// Keep track of commits.
	var prevCommits []*vcsinfo.IndexCommit
	go util.RepeatCtx(interval, ctx, func() {
		ctx := context.TODO()
		allBranches, err := gitStore.GetBranches(ctx)
		if err != nil {
			sklog.Errorf("Error retrieving branches: %s", err)
			return
		}

		branchInfo, ok := allBranches[branch]
		if !ok {
			sklog.Errorf("Branch %s not found in gitstore", branch)
			return
		}

		startIdx := util.MaxInt(0, branchInfo.Index+1-nCommits)
		commits, err := gitStore.RangeN(ctx, startIdx, int(math.MaxInt32), branch)
		if err != nil {
			sklog.Errorf("Error getting last %d commits: %s", nCommits, err)
			return
		}

		// If we received new commits then publish an event and save them for the next round.
		if len(prevCommits) != len(commits) || commits[len(commits)-1].Index > prevCommits[len(prevCommits)-1].Index {
			prevCommits = commits
			cpCommits := append([]*vcsinfo.IndexCommit{}, commits...)
			evt.Publish(EV_NEW_GIT_COMMIT, cpCommits, false)
		}
	})
}
