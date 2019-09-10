package bt_vcs

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// BigTableVCS implements the vcsinfo.VCS interface based on a BT-backed GitStore.
type BigTableVCS struct {
	gitStore           gitstore.GitStore
	gitiles            *gitiles.Repo
	branch             string
	secondaryVCS       vcsinfo.VCS
	secondaryExtractor depot_tools.DEPSExtractor

	// This mutex protects detailsCache and indexCommits
	mutex sync.RWMutex
	// detailsCache is for LongCommits so we don't have to query gitStore every time
	detailsCache map[string]*vcsinfo.LongCommit
	indexCommits []*vcsinfo.IndexCommit
}

// NewVCS returns an instance of vcsinfo.VCS that is backed by the given GitStore and uses the
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

// SetSecondaryRepo allows to add a secondary repository and extractor to this instance.
// It is not included in the constructor since it is currently only used by the Gold ingesters.
func (b *BigTableVCS) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	b.secondaryVCS = secVCS
	b.secondaryExtractor = extractor
}

// Update implements the vcsinfo.VCS interface
func (b *BigTableVCS) Update(ctx context.Context, _, _ bool) error {
	var oldHead *vcsinfo.IndexCommit
	b.mutex.Lock()
	if len(b.indexCommits) > 0 {
		oldHead = b.indexCommits[len(b.indexCommits)-1]
	}
	b.mutex.Unlock()

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
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.indexCommits = append(reloadIcs, ics...)
	} else {
		// Append the new commits onto the existing slice, accounting
		// for the overlapped commit, if applicable.
		if oldHead != nil {
			ics = ics[1:]
		}
		b.mutex.Lock()
		defer b.mutex.Unlock()
		b.indexCommits = append(b.indexCommits, ics...)
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
	// Instantiate a list of N nil values so we can directly insert them into the proper index.
	rv := make([]*vcsinfo.LongCommit, len(hashes))

	missedHashes := []string{}
	// Index into hashes of which hashes were not in the cache
	missedHashesIdx := []int{}

	func() {
		b.mutex.RLock()
		defer b.mutex.RUnlock()
		for i, hash := range hashes {
			c, ok := b.detailsCache[hash]
			if ok {
				rv[i] = c
			} else {
				missedHashesIdx = append(missedHashesIdx, i)
				missedHashes = append(missedHashes, hash)
			}
		}
	}()
	if len(missedHashes) == 0 {
		return rv, nil
	}

	// bulk fetch the missedHashes.
	commits, err := b.gitStore.Get(ctx, missedHashes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Get missed hashes %q (superset %q)", missedHashes, hashes)
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	for i, hash := range missedHashes {
		c := commits[i]
		if c != nil {
			rv[missedHashesIdx[i]] = c
			b.detailsCache[hash] = c
		}
	}

	return rv, nil
}

// details returns all meta data details we care about.
func (b *BigTableVCS) details(ctx context.Context, hash string, _ bool) (*vcsinfo.LongCommit, error) {
	commits, err := b.gitStore.Get(ctx, []string{hash})
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 || commits[0] == nil {
		return nil, skerr.Fmt("Commit %s not found", hash)
	}
	return commits[0], nil
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
	// TODO(borenet): Should we build a map[hash]*IndexCommit?
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, ic := range b.indexCommits {
		if ic.Hash == hash {
			return ic.Index, nil
		}
	}
	return 0, skerr.Fmt("Unknown commit %s", hash)
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
	ic, err := b.getAtIndex(idx)
	if err != nil {
		return nil, err
	}
	return b.Details(ctx, ic.Hash, false)
}

// GetFile implements the vcsinfo.VCS interface
func (b *BigTableVCS) GetFile(ctx context.Context, fileName, commitHash string) (string, error) {
	var buf bytes.Buffer
	if err := b.gitiles.ReadFileAtRef(ctx, fileName, commitHash, &buf); err != nil {
		return "", skerr.Wrapf(err, "reading file %s @ %s via gitiles", fileName, commitHash)
	}
	return buf.String(), nil
}

// ResolveCommit implements the vcsinfo.VCS interface
func (b *BigTableVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	if b.secondaryVCS == nil {
		return "", vcsinfo.NoSecondaryRepo
	}

	foundCommit, err := b.secondaryExtractor.ExtractCommit(b.secondaryVCS.GetFile(ctx, "DEPS", commitHash))
	if err != nil {
		return "", err
	}
	return foundCommit, nil
}

// GetGitStore implements the gitstore.GitStoreBased interface
func (b *BigTableVCS) GetGitStore() gitstore.GitStore {
	return b.gitStore
}

// timeRange retrieves IndexCommits from the given gime range. Assumes that the
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
