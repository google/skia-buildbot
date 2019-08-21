package repograph

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

// MemCacheRepoImpl is a RepoImpl which just caches commits in memory.
type MemCacheRepoImpl struct {
	Commits    map[string]*vcsinfo.LongCommit
	BranchList []*git.Branch
}

// NewMemCacheRepoImpl returns a RepoImpl implementation which just caches
// commits in memory. The commits map must contain all commits needed by the
// given branch heads.
func NewMemCacheRepoImpl(commits map[string]*vcsinfo.LongCommit, branches []*git.Branch) *MemCacheRepoImpl {
	if commits == nil {
		commits = map[string]*vcsinfo.LongCommit{}
	}
	return &MemCacheRepoImpl{
		Commits:    commits,
		BranchList: branches,
	}
}

// See documentation for RepoImpl interface.
func (ri *MemCacheRepoImpl) Update(_ context.Context) error {
	return nil
}

// See documentation for RepoImpl interface.
func (ri *MemCacheRepoImpl) Details(_ context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if d, ok := ri.Commits[hash]; ok {
		return d, nil
	}
	return nil, skerr.Fmt("Unknown commit %s", hash)
}

// See documentation for RepoImpl interface.
func (ri *MemCacheRepoImpl) Branches(_ context.Context) ([]*git.Branch, error) {
	return ri.BranchList, nil
}

// See documentation for RepoImpl interface.
func (ri *MemCacheRepoImpl) UpdateCallback(_ context.Context, _, _ []*vcsinfo.LongCommit, _ *Graph) error {
	return nil
}
