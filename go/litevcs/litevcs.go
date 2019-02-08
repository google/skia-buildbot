package litevcs

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type liteVCS struct {
	gitStore           GitStore
	repo               *gitiles.Repo
	defaultBranch      string
	secondaryVCS       vcsinfo.VCS
	secondaryExtractor depot_tools.DEPSExtractor

	branchInfo   *BranchPointer
	indexCommits []*vcsinfo.IndexCommit // Updated by Update, From, LastNIndex, Range, IndexOf(slow), ByIndex
	hashes       []string
	timestamps   map[string]time.Time           //
	detailsCache map[string]*vcsinfo.LongCommit // Details
	mutex        sync.RWMutex
}

func NewVCS(gitstore GitStore, defaultBranch string, repo *gitiles.Repo) (vcsinfo.VCS, error) {
	ret := &liteVCS{
		gitStore:      gitstore,
		repo:          repo,
		defaultBranch: defaultBranch,
	}
	if err := ret.Update(context.TODO(), true, false); err != nil {
		return nil, err
	}
	return ret, nil
}

// SetSecondaryRepo allows to add a secondary repository and extractor to this instance.
// It is not included in the constructor since it is currently only used by the Gold ingesters.
func (li *liteVCS) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	li.secondaryVCS = secVCS
	li.secondaryExtractor = extractor
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) Update(ctx context.Context, pull, allBranches bool) error {
	// Simulate a pull by fetching the latest head of the target branch.
	if pull {
		allBranches, err := li.gitStore.GetBranches(ctx)
		if err != nil {
			return err
		}

		var ok bool
		li.branchInfo, ok = allBranches[li.defaultBranch]
		if !ok {
			return skerr.Fmt("Unable to find branch %s in BitTable repo %s", li.defaultBranch, (li.gitStore.(*btGitStore)).repoURL)
		}
	}

	// Get all index commits for the current branch.
	return li.fetchIndexRange(ctx, 0, li.branchInfo.Index+1)
}

// ---------------DONE ^ DONE

// From implements the vcsinfo.VCS interface
func (li *liteVCS) From(start time.Time) []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	// Add a millisecond because we only want commits after the startTime. Timestamps in git are
	// only at second level granularity.
	found := li.timeRange(start.Add(time.Millisecond), maxTime)
	ret := make([]string, len(found))
	for i, c := range found {
		ret[i] = c.Hash
	}
	return ret
}

// Details implements the vcsinfo.VCS interface
func (li *liteVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	li.mutex.Lock()
	defer li.mutex.Unlock()
	return li.details(ctx, hash, includeBranchInfo)
}

func (li *liteVCS) details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	commits, err := li.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, skerr.Fmt("Commit %s not found", hash)
	}
	return commits[0], nil
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	if N > len(li.indexCommits) {
		N = len(li.indexCommits)
	}
	ret := make([]*vcsinfo.IndexCommit, 0, N)
	return append(ret, li.indexCommits[len(li.indexCommits)-N:]...)
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) Range(begin, end time.Time) []*vcsinfo.IndexCommit {
	sklog.Infof("-----------------RANGE   %d      %d   ", begin.UnixNano()/int64(time.Millisecond), end.UnixNano()/int64(time.Millisecond))
	return li.timeRange(begin, end)
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) IndexOf(ctx context.Context, hash string) (int, error) {
	li.mutex.RLock()
	defer li.mutex.Unlock()

	for _, c := range li.indexCommits {
		if c.Hash == hash {
			return c.Index, nil
		}
	}

	// If it was not in memory we need to fetch it
	details, err := li.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return 0, err
	}

	if len(details) == 0 {
		return 0, skerr.Fmt("Hash %s does not exist in repository on branch %s", hash, li.defaultBranch)
	}

	return 0, nil
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {

	// findFn returns the hash when N is within commits
	findFn := func(commits []*vcsinfo.IndexCommit) string {
		i := sort.Search(len(commits), func(i int) bool { return commits[i].Index >= N })
		return commits[i].Hash
	}

	var hash string
	li.mutex.RLock()
	if len(li.indexCommits) > 0 {
		firstIdx := li.indexCommits[0].Index
		lastIdx := li.indexCommits[len(li.indexCommits)-1].Index
		if (N >= firstIdx) && (N <= lastIdx) {
			hash = findFn(li.indexCommits)
		}
	}
	li.mutex.RUnlock()

	// Fetch the hash
	if hash == "" {
		return nil, fmt.Errorf("Hash index not found: %d", N)
	}
	return li.details(ctx, hash, false)
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	var buf bytes.Buffer
	if err := li.repo.ReadFileAtRef(fileName, commitHash, &buf); err != nil {
		return "", skerr.Fmt("Error reading file %s @ %s via gitiles: %s", fileName, commitHash, err)
	}
	return buf.String(), nil
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", skerr.Fmt("Not implemented yet")
}

// fetchIndexRange gets in the range [startIndex, endIndex).
func (li *liteVCS) fetchIndexRange(ctx context.Context, startIndex, endIndex int) error {
	newIC, err := li.gitStore.RangeN(ctx, startIndex, endIndex, li.defaultBranch)
	if err != nil {
		return err
	}

	if len(newIC) == 0 {
		return nil
	}

	li.mutex.Lock()
	defer li.mutex.Unlock()
	li.indexCommits = newIC
	return nil
}

func (li *liteVCS) timeRangeNG(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	startSec := start.Unix()
	endSec := end.Unix()
	n := len(li.indexCommits)
	startIdx := 0
	for ; startIdx < n; startIdx++ {
		exp := li.indexCommits[startIdx].Timestamp.Unix() >= startSec
		sklog.Infof("qqq: %d       %d   %d    %v      ", startIdx, li.indexCommits[startIdx].Timestamp.Unix(), start.Unix(), exp)
		if exp {
			break
		}
	}

	endIdx := startIdx
	for ; endIdx < n; endIdx++ {
		exp := li.indexCommits[endIdx].Timestamp.Unix() >= endSec
		sklog.Infof("xxx: %d       %d   %d    %v      ", endIdx, li.indexCommits[endIdx].Timestamp.Unix(), end.Unix(), exp)

		if exp {
			break
		}
	}

	if endIdx <= startIdx {
		return []*vcsinfo.IndexCommit{}
	}
	return li.indexCommits[startIdx:endIdx]
}

func (li *liteVCS) timeRange(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	n := len(li.indexCommits)
	startIdx := 0
	for ; startIdx < n; startIdx++ {
		exp := li.indexCommits[startIdx].Timestamp.After(start) || li.indexCommits[startIdx].Timestamp.Equal(start)
		sklog.Infof("qqq: %d       %d   %d    %v      ", startIdx, li.indexCommits[startIdx].Timestamp.Unix(), start.Unix(), exp)
		if exp {
			break
		}
	}

	endIdx := startIdx
	for ; endIdx < n; endIdx++ {
		exp := li.indexCommits[endIdx].Timestamp.After(end) || li.indexCommits[endIdx].Timestamp.Equal(end)
		sklog.Infof("xxx: %d       %d   %d    %v      ", endIdx, li.indexCommits[endIdx].Timestamp.Unix(), end.Unix(), exp)

		if exp {
			break
		}
	}

	if endIdx <= startIdx {
		return []*vcsinfo.IndexCommit{}
	}
	return li.indexCommits[startIdx:endIdx]
}

// func (li *liteVCS) timeRange(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
// 	n := len(li.indexCommits)
// 	sklog.Infof("C: ----------------------------------------------------------------")
// 	for _, commit := range li.indexCommits {
// 		sklog.Infof("C: %s\n\n", spew.Sdump(commit))
// 	}
// 	startFn := func(i int) bool { return li.indexCommits[i].Timestamp.Sub(start) >= 0 }
// 	endFn := func(i int) bool { return li.indexCommits[i].Timestamp.After(end) }
// 	startIdx := sort.Search(n, startFn)
// 	endIdx := sort.Search(n, endFn)
// 	if startIdx > endIdx {
// 		return []*vcsinfo.IndexCommit{}
// 	}
// 	return li.indexCommits[startIdx:endIdx]
// }
