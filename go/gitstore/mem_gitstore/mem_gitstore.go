package mem_gitstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

// MemGitStore implements the gitstore.GitStore interface in memory.
type MemGitStore struct {
	mtx      sync.RWMutex
	branches map[string]*gitstore.BranchPointer
	commits  map[string]*vcsinfo.LongCommit
}

// New returns an in-memory gitstore.GitStore implementation.
func New() *MemGitStore {
	return &MemGitStore{
		branches: map[string]*gitstore.BranchPointer{},
		commits:  map[string]*vcsinfo.LongCommit{},
	}
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	gs.mtx.Lock()
	defer gs.mtx.Unlock()
	for _, c := range commits {
		gs.commits[c.Hash] = c
	}
	return nil
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
	for _, hash := range hashes {
		// Don't bother checking whether the commit exists; per the
		// GitStore docs, Get may contain nil entries in the returned
		// slice.
		rv = append(rv, gs.commits[hash])
	}
	return rv, nil
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	gs.mtx.Lock()
	defer gs.mtx.Unlock()
	for name, hash := range branches {
		if hash == gitstore.DELETE_BRANCH {
			delete(gs.branches, name)
		} else {
			head, ok := gs.commits[hash]
			if !ok {
				return skerr.Fmt("Unknown commit %s for branch %s", hash, name)
			}
			gs.branches[name] = &gitstore.BranchPointer{
				Head:  hash,
				Index: head.Index,
			}
		}
	}
	return nil
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) GetBranches(ctx context.Context) (map[string]*gitstore.BranchPointer, error) {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	rv := make(map[string]*gitstore.BranchPointer, len(gs.branches))
	for name, ptr := range gs.branches {
		rv[name] = &gitstore.BranchPointer{
			Head:  ptr.Head,
			Index: ptr.Index,
		}
	}
	return rv, nil
}

// getIndexCommits is a helper function for retrieving IndexCommits.
func (gs *MemGitStore) getIndexCommits(branch string, include func(*vcsinfo.LongCommit) bool) []*vcsinfo.IndexCommit {
	gs.mtx.RLock()
	defer gs.mtx.RUnlock()
	ptr := gs.branches[branch]
	rv := []*vcsinfo.IndexCommit{}
	// TODO(borenet): This could be more efficient if we maintained a sorted
	// slice of commits.
	for _, c := range gs.commits {
		if (branch == gitstore.ALL_BRANCHES || ptr != nil && c.Branches[branch] && c.Index <= ptr.Index) && include(c) {
			rv = append(rv, c.IndexCommit())
		}
	}
	sort.Sort(vcsinfo.IndexCommitSlice(rv))
	return rv
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	return gs.getIndexCommits(branch, func(c *vcsinfo.LongCommit) bool {
		return c.Index >= startIndex && c.Index < endIndex
	}), nil
}

// See documentation for gitstore.GitStore interface.
func (gs *MemGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	return gs.getIndexCommits(branch, func(c *vcsinfo.LongCommit) bool {
		return !c.Timestamp.Before(start) && c.Timestamp.Before(end)
	}), nil
}

var _ gitstore.GitStore = &MemGitStore{}
