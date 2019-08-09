package repograph

import (
	"context"
	"errors"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// gitstoreRepoImpl is an implementation of the RepoImpl interface which uses a
// GitStore to interact with a git repo.
type gitstoreRepoImpl struct {
	gs gitstore.GitStore

	// These are stored between calls to Update so that we don't have to
	// Get individual commits as they are requested.
	branches   []*git.Branch
	commits    map[string]*vcsinfo.LongCommit
	lastUpdate time.Time
}

// NewGitStoreRepoImpl returns a RepoImpl instance which is backed by GitStore.
func NewGitStoreRepoImpl(gs gitstore.GitStore) RepoImpl {
	return &gitstoreRepoImpl{
		gs: gs,
	}
}

// See documentation for RepoImpl interface.
func (g *gitstoreRepoImpl) Update(ctx context.Context) error {
	branchPtrs, err := g.gs.GetBranches(ctx)
	if err != nil {
		return err
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
		return err
	}
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := g.gs.Get(ctx, hashes)
	if err != nil {
		return err
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}

	g.lastUpdate = now
	g.branches = branches
	g.commits = commitsMap
	return nil
}

// See documentation for RepoImpl interface.
func (g *gitstoreRepoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := g.commits[hash]; ok {
		return c, nil
	}
	sklog.Warningf("Commit %q not found in results; performing explicit lookup.", hash)
	got, err := g.gs.Get(ctx, []string{hash})
	if err != nil {
		return nil, err
	}
	for _, c := range got {
		g.commits[c.Hash] = c
	}
	return got[0], nil
}

// See documentation for RepoImpl interface.
func (g *gitstoreRepoImpl) Branches(ctx context.Context) ([]*git.Branch, error) {
	if g.branches == nil {
		return nil, errors.New("Need to call Update() before Branches()")
	}
	return g.branches, nil
}

// See documentation for RepoImpl interface.
func (g *gitstoreRepoImpl) UpdateCallback(ctx context.Context, graph *Graph) error {
	return nil
}

// See documentation for RepoImpl interface.
func (r *gitstoreRepoImpl) InitCache(branches []*git.Branch, commits map[string]*Commit) error {
	for _, c := range commits {
		r.commits[c.Hash] = c.LongCommit
	}
	r.branches = branches
	return nil
}
