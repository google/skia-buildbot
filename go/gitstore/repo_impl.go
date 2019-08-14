package gitstore

import (
	"context"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// GetRepoGraph returns *repograph.Graph backed by the given GitStore.
func GetRepoGraph(ctx context.Context, gs GitStore) (*repograph.Graph, error) {
	ri, err := NewGitStoreRepoImpl(ctx, gs)
	if err != nil {
		return nil, err
	}
	return repograph.NewWithRepoImpl(ctx, ri)
}

// gitStoreRepoImpl is an implementation of the repograph.RepoImpl interface
// which uses a GitStore to interact with a git repo.
type gitStoreRepoImpl struct {
	gs GitStore

	// These are stored between calls to Update so that we don't have to
	// Get individual commits as they are requested.
	BranchList []*git.Branch
	Commits    map[string]*vcsinfo.LongCommit
	lastUpdate time.Time
}

// NewGitStoreRepoImpl returns a repograph.RepoImpl instance which uses the
// given GitStore.
func NewGitStoreRepoImpl(ctx context.Context, gs GitStore) (repograph.RepoImpl, error) {
	rv := &gitStoreRepoImpl{gs: gs}
	if err := rv.Update(ctx); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for repograph.RepoImpl interface.
func (g *gitStoreRepoImpl) Update(ctx context.Context) error {
	branchPtrs, err := g.gs.GetBranches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to read branches from GitStore")
	}
	branches := make([]*git.Branch, 0, len(branchPtrs))
	for name, ptr := range branchPtrs {
		if name != "" {
			branches = append(branches, &git.Branch{
				Name: name,
				Head: ptr.Head,
			})
		}
	}

	from := g.lastUpdate.Add(-10 * time.Minute)
	now := time.Now()
	to := now.Add(time.Second)
	indexCommits, err := g.gs.RangeByTime(ctx, from, to, "")
	if err != nil {
		return skerr.Wrapf(err, "Failed to read IndexCommits from GitStore")
	}
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := g.gs.Get(ctx, hashes)
	if err != nil {
		return skerr.Wrapf(err, "Failed to read LongCommits from GitStore")
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}

	g.lastUpdate = now
	g.BranchList = branches
	g.Commits = commitsMap
	return nil
}

// See documentation for repograph.RepoImpl interface.
func (g *gitStoreRepoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if d, ok := g.Commits[hash]; ok {
		return d, nil
	}
	// Update() should have pre-fetched all of the commits for us, so we
	// shouldn't have hit this code. Log a warning and fall back to
	// retrieving the commit from GitStore.
	sklog.Warningf("Commit %q not found in results; performing explicit lookup.", hash)
	got, err := g.gs.Get(ctx, []string{hash})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read commit %s from GitStore", hash)
	}
	for _, c := range got {
		if c == nil {
			return nil, skerr.Fmt("Commit %s not present in GitStore.", hash)
		}
		g.Commits[c.Hash] = c
	}
	return got[0], nil
}

// See documentation for repograph.RepoImpl interface.
func (g *gitStoreRepoImpl) Branches(ctx context.Context) ([]*git.Branch, error) {
	if g.BranchList == nil {
		return nil, skerr.Fmt("Need to call Update() before Branches()")
	}
	return g.BranchList, nil
}

// See documentation for repograph.RepoImpl interface.
func (g *gitStoreRepoImpl) UpdateCallback(ctx context.Context, graph *repograph.Graph) error {
	return nil
}
