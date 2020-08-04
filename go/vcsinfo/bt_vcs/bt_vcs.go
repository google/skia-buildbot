package bt_vcs

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// BigTableVCS implements the vcsinfo.VCS interface based on a BT-backed GitStore.
type BigTableVCS struct {
	gitStore gitstore.GitStore
	gitiles  *gitiles.Repo
	branch   string

	// This mutex protects detailsCache and indexCommits
	mutex sync.RWMutex
	// detailsCache is for LongCommits so we don't have to query gitStore every time
	detailsCache map[string]*vcsinfo.LongCommit
	indexCommits []*vcsinfo.IndexCommit

	// This mutex is held throughout the execution of Update, to prevent
	// concurrent Updates from clobbering each other.
	updateMutex sync.Mutex
}

// New returns an instance of vcsinfo.VCS that is backed by the given GitStore and uses the
// gittiles.Repo to retrieve files. Each instance provides an interface to one branch.
// The instance of gitiles.Repo is only used to fetch files.
func New(ctx context.Context, gitStore gitstore.GitStore, branch string, repo *gitiles.Repo) (*BigTableVCS, error) {
	if gitStore == nil {
		return nil, errors.New("Cannot have nil gitStore")
	}
	ret := &BigTableVCS{
		gitStore:     gitStore,
		gitiles:      repo,
		branch:       branch,
		detailsCache: map[string]*vcsinfo.LongCommit{},
	}
	if err := ret.Update(ctx, true, false); err != nil {
		return nil, skerr.Wrapf(err, "could not perform initial update")
	}

	return ret, nil
}

// GetBranch implements the vcsinfo.VCS interface.
func (b *BigTableVCS) GetBranch() string {
	return b.branch
}

// Update implements the vcsinfo.VCS interface
func (b *BigTableVCS) Update(ctx context.Context, _, _ bool) error {
	b.updateMutex.Lock()
	defer b.updateMutex.Unlock()

	var oldHead *vcsinfo.IndexCommit
	b.mutex.RLock()
	if len(b.indexCommits) > 0 {
		oldHead = b.indexCommits[len(b.indexCommits)-1]
	}
	b.mutex.RUnlock()

	// Retrieve all IndexCommits including and after the newest
	// commit we have.
	oldIdx := 0
	if oldHead != nil {
		oldIdx = oldHead.Index
	}
	ics, err := b.gitStore.RangeN(ctx, oldIdx, math.MaxInt32, b.branch)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve IndexCommit range [%d:%d)", oldIdx, math.MaxInt32)
	}
	reload := false
	if len(ics) > 0 {
		// The first commit returned from RangeN should match
		// oldHead. If not, then history has changed and we need
		// to reload from scratch.
		if oldHead != nil && ics[0].Hash != oldHead.Hash {
			sklog.Errorf("Commit at index %d on branch %s was %s but is now %s; reloading from scratch.", oldHead.Index, b.branch, oldHead.Hash, ics[0].Hash)
			reload = true
		}
	} else if oldHead != nil {
		// We should've received at least the oldHead commit.
		// If not, then history has changed and we need to
		// reload from scratch.
		sklog.Errorf("Found did not find existing IndexCommit %d:%s for %s; reloading from scratch.", oldHead.Index, oldHead.Hash, b.branch)
		reload = true
	}

	// If necessary, load all other commits.
	if reload {
		// Load all of the commits on the branch up to the ones we
		// already retrieved.
		startIdx := 0
		endIdx := math.MaxInt32
		if len(ics) > 0 {
			endIdx = ics[0].Index
		}
		reloadIcs, err := b.gitStore.RangeN(ctx, 0, endIdx, b.branch)
		if err != nil {
			return skerr.Wrapf(err, "failed to reload IndexCommits from scratch [%d:%d)", startIdx, endIdx)
		}
		ics = append(reloadIcs, ics...)
	} else {
		// Remove the overlapped commit, if necessary.
		if oldHead != nil {
			ics = ics[1:]
		}
	}

	// Retrieve the new LongCommits.
	var lcs []*vcsinfo.LongCommit
	if len(ics) > 0 {
		hashes := make([]string, 0, len(ics))
		for _, ic := range ics {
			hashes = append(hashes, ic.Hash)
		}
		lcs, err = b.gitStore.Get(ctx, hashes)
		if err != nil {
			return skerr.Wrapf(err, "failed to retrieve new LongCommits")
		}
		for idx, ic := range ics {
			lc := lcs[idx]
			if lc == nil {
				return skerr.Fmt("GitStore returned nil for commit %s", ic.Hash)
			}
			lc.Index = ic.Index
		}
	}

	// Save the new data.
	if len(ics) > 0 {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		if reload {
			b.indexCommits = ics
			b.detailsCache = make(map[string]*vcsinfo.LongCommit, len(lcs))
		} else {
			b.indexCommits = append(b.indexCommits, ics...)
		}
		for _, lc := range lcs {
			b.detailsCache[lc.Hash] = lc
		}
		// Sanity check: an IndexCommit's should always be at the index
		// in our slice which matches its Index field.
		for idx, ic := range b.indexCommits {
			if ic.Index != idx {
				return skerr.Fmt("Commit %s has Index %d but is in our slice at index %d", ic.Hash, ic.Index, idx)
			}
		}
	}

	return nil
}

// From implements the vcsinfo.VCS interface
func (b *BigTableVCS) From(start time.Time) []string {
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
func (b *BigTableVCS) Details(ctx context.Context, hash string, _ bool) (*vcsinfo.LongCommit, error) {
	details, err := b.DetailsMulti(ctx, []string{hash}, false)
	if err != nil {
		return nil, err
	}
	return details[0], err
}

// DetailsMulti implements the vcsinfo.VCS interface
func (b *BigTableVCS) DetailsMulti(ctx context.Context, hashes []string, _ bool) ([]*vcsinfo.LongCommit, error) {
	rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for _, hash := range hashes {
		lc, ok := b.detailsCache[hash]
		if !ok {
			return nil, skerr.Fmt("Unknown commit %s", hash)
		}
		rv = append(rv, lc)
	}
	return rv, nil
}

// LastNIndex implements the vcsinfo.VCS interface
func (b *BigTableVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if N > len(b.indexCommits) {
		N = len(b.indexCommits)
	}
	ret := make([]*vcsinfo.IndexCommit, 0, N)
	return append(ret, b.indexCommits[len(b.indexCommits)-N:]...)
}

// Range implements the vcsinfo.VCS interface
func (b *BigTableVCS) Range(begin, end time.Time) []*vcsinfo.IndexCommit {
	return b.timeRange(begin, end)
}

// IndexOf implements the vcsinfo.VCS interface
func (b *BigTableVCS) IndexOf(ctx context.Context, hash string) (int, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	lc, ok := b.detailsCache[hash]
	if !ok {
		return -1, skerr.Fmt("Unknown commit %s", hash)
	}
	return lc.Index, nil
}

// getAtIndex returns the IndexCommit at the given index.
func (b *BigTableVCS) getAtIndex(idx int) (*vcsinfo.IndexCommit, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if idx < 0 || idx >= len(b.indexCommits) {
		return nil, skerr.Fmt("Hash index not found: %d", idx)
	}
	return b.indexCommits[idx], nil
}

// ByIndex implements the vcsinfo.VCS interface
func (b *BigTableVCS) ByIndex(ctx context.Context, idx int) (*vcsinfo.LongCommit, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if idx < 0 || idx >= len(b.indexCommits) {
		return nil, skerr.Fmt("Hash index not found: %d", idx)
	}
	return b.detailsCache[b.indexCommits[idx].Hash], nil
}

// GetFile implements the vcsinfo.VCS interface
func (b *BigTableVCS) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	contents, err := b.gitiles.ReadFileAtRef(ctx, fileName, commitHash)
	if err != nil {
		return "", skerr.Wrapf(err, "reading file %s @ %s via gitiles", fileName, commitHash)
	}
	return string(contents), nil
}

// GetGitStore implements the gitstore.GitStoreBased interface
func (b *BigTableVCS) GetGitStore() gitstore.GitStore {
	return b.gitStore
}

// timeRange retrieves IndexCommits from the given time range. Assumes that the
// caller holds b.mutex.
func (b *BigTableVCS) timeRange(start time.Time, end time.Time) []*vcsinfo.IndexCommit {
	if end.Before(start) {
		return []*vcsinfo.IndexCommit{}
	}
	n := len(b.indexCommits)
	// TODO(borenet,kjlubick): Git commit timestamps can be forged, so it's
	// entirely possible that the timestamps do not increase monotonically
	// in the IndexCommit slice. If that's the case, the return value of
	// this function may not be correct.
	startIdx := sort.Search(n, func(idx int) bool {
		return !b.indexCommits[idx].Timestamp.Before(start)
	})
	endIdx := startIdx + sort.Search(n-startIdx, func(idx int) bool {
		return !b.indexCommits[idx+startIdx].Timestamp.Before(end)
	})
	return b.indexCommits[startIdx:endIdx]
}

// Make sure BigTableVCS fulfills the VCS interface
var _ vcsinfo.VCS = (*BigTableVCS)(nil)
