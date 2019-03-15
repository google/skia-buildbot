package gitstore

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
	"go.skia.org/infra/go/vcsinfo"
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
// The instances of gitiles.Repo is only used to fetch files.
func NewVCS(gitstore GitStore, defaultBranch string, repo *gitiles.Repo) (vcsinfo.VCS, error) {
	ret := &btVCS{
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
func (b *btVCS) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	b.secondaryVCS = secVCS
	b.secondaryExtractor = extractor
}

// Update implements the vcsinfo.VCS interface
func (b *btVCS) Update(ctx context.Context, pull, allBranches bool) error {
	// Simulate a pull by fetching the latest head of the target branch.
	if pull {
		allBranches, err := b.gitStore.GetBranches(ctx)
		if err != nil {
			return err
		}

		var ok bool
		b.branchInfo, ok = allBranches[b.defaultBranch]
		if !ok {
			return skerr.Fmt("Unable to find branch %s in BitTable repo %s", b.defaultBranch, (b.gitStore.(*btGitStore)).repoURL)
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
	found := b.timeRange(start.Add(time.Millisecond), MaxTime)
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
	return commits[0], nil
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
	return "", skerr.Fmt("Not implemented yet")
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

func (b *btVCS) timeRangeNG(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	startSec := start.Unix()
	endSec := end.Unix()
	n := len(b.indexCommits)
	startIdx := 0
	for ; startIdx < n; startIdx++ {
		exp := b.indexCommits[startIdx].Timestamp.Unix() >= startSec
		if exp {
			break
		}
	}

	endIdx := startIdx
	for ; endIdx < n; endIdx++ {
		exp := b.indexCommits[endIdx].Timestamp.Unix() >= endSec
		if exp {
			break
		}
	}

	if endIdx <= startIdx {
		return []*vcsinfo.IndexCommit{}
	}
	return b.indexCommits[startIdx:endIdx]
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
