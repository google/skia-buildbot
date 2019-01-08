package litevcs

import (
	"bytes"
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

type liteVCS struct {
	gitStore           GitStore
	repo               *gitiles.Repo
	secondaryVCS       vcsinfo.VCS
	secondaryExtractor depot_tools.DEPSExtractor

	indexCommits []*vcsinfo.IndexCommit // Updated by Update, From, LastNIndex, Range, IndexOf(slow), ByIndex
	hashes       []string
	timestamps   map[string]time.Time           //
	detailsCache map[string]*vcsinfo.LongCommit // Details
	mutex        sync.RWMutex
}

func NewVCS(gitstore GitStore, repo *gitiles.Repo) (vcsinfo.VCS, error) {
	ret := &liteVCS{
		gitStore: gitstore,
		repo:     repo,
	}
	return ret, ret.Update(context.TODO(), true, true)
}

// SetSecondaryRepo allows to add a secondary repository and extractor to this instance.
// It is not included in the constructor since it is currently only used by the Gold ingesters.
func (li *liteVCS) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	li.secondaryVCS = secVCS
	li.secondaryExtractor = extractor
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) Update(ctx context.Context, pull, allBranches bool) error {
	if !pull {
		return nil
	}
	li.mutex.RLock()
	startIdx := -1
	if len(li.indexCommits) > 0 {
		startIdx = li.indexCommits[len(li.indexCommits)-1].Index + 1
	}
	li.mutex.RUnlock()
	return li.fetchIndexRange(ctx, startIdx, maxInt, false)
}

// From implements the vcsinfo.VCS interface
func (li *liteVCS) From(start time.Time) []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	found := li.timeRange(start, maxTime)
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
	commits, _, err := li.gitStore.Get(ctx, []string{hash})
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
	_, indices, err := li.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return 0, err
	}
	if len(indices) == 0 {
		return 0, skerr.Fmt("Hash %s does not exist in repository", hash)
	}
	return indices[0], nil
}

// Update implements the vcsinfo.VCS interface
func (li *liteVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {

	// findFn returns the hash when N is within commits
	findFn := func(commits []*vcsinfo.IndexCommit) string {
		i := sort.Search(len(commits), func(i int) bool { return commits[i].Index >= N })
		return commits[i].Hash
	}

	var hash string
	startFetch := -1
	endFetch := int(math.MaxInt64)
	prepend := false

	li.mutex.RLock()
	if len(li.indexCommits) > 0 {
		firstIdx := li.indexCommits[0].Index
		lastIdx := li.indexCommits[len(li.indexCommits)-1].Index
		if (N >= firstIdx) && (N <= lastIdx) {
			hash = findFn(li.indexCommits)
		} else {
			startFetch = util.MinInt(N, firstIdx)
			endFetch = util.MaxInt(N, lastIdx) + 1
			prepend = N < firstIdx
		}
	}
	li.mutex.RUnlock()

	// Fetch the hash
	if hash == "" {
		if err := li.fetchIndexRange(ctx, startFetch, endFetch, prepend); err != nil {
			return nil, err
		}

		li.mutex.RLock()
		hash = findFn(li.indexCommits)
		li.mutex.RUnlock()
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

func (li *liteVCS) fetchIndexRange(ctx context.Context, startIndex, endIndex int, prepend bool) error {
	newIC, err := li.gitStore.RangeN(ctx, startIndex, endIndex)
	if err != nil {
		return err
	}

	if len(newIC) == 0 {
		return nil
	}

	li.mutex.Lock()
	defer li.mutex.Unlock()
	if len(li.indexCommits) == 0 {
		li.indexCommits = newIC
		return nil
	}

	oldStart := li.indexCommits[0].Index
	oldEnd := li.indexCommits[len(li.indexCommits)-1].Index
	newStart := newIC[0].Index
	newEnd := newIC[len(newIC)-1].Index

	if prepend {
		if (newEnd + 1) != oldStart {
			return skerr.Fmt("Prepend failed !")
		}
		li.indexCommits = append(newIC, li.indexCommits...)
	} else {
		if newStart != (oldEnd + 1) {
			return skerr.Fmt("Append failed")
		}
		li.indexCommits = append(li.indexCommits, newIC...)
	}
	return nil
}

func (li *liteVCS) timeRange(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	n := len(li.indexCommits)
	sklog.Infof("C: ----------------------------------------------------------------")
	for _, commit := range li.indexCommits {
		sklog.Infof("C: %s\n\n", spew.Sdump(commit))
	}
	startFn := func(i int) bool { return li.indexCommits[i].Timestamp.Sub(start) >= 0 }
	endFn := func(i int) bool { return li.indexCommits[i].Timestamp.After(end) }
	startIdx := sort.Search(n, startFn)
	endIdx := sort.Search(n, endFn)
	if startIdx > endIdx {
		return []*vcsinfo.IndexCommit{}
	}
	return li.indexCommits[startIdx:endIdx]
}
