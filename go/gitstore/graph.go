package gitstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// GetRepoGraph returns *repograph.Graph backed by the given GitStore.
func GetRepoGraph(ctx context.Context, gs GitStore) (*repograph.Graph, error) {
	return repograph.NewWithRepoImpl(ctx, NewGitStoreRepoImpl(gs))
}

// GitStoreRepoImpl is an implementation of the repograph.RepoImpl interface
// which uses a GitStore to interact with a git repo.
type GitStoreRepoImpl struct {
	gs GitStore

	// These are stored between calls to Update so that we don't have to
	// Get individual commits as they are requested.
	BranchList []*git.Branch
	Commits    map[string]*vcsinfo.LongCommit
	lastUpdate time.Time
}

// NewGitStoreRepoImpl returns a repograph.RepoImpl instance which uses the
// given GitStore.
func NewGitStoreRepoImpl(gs GitStore) repograph.RepoImpl {
	return &GitStoreRepoImpl{gs: gs}
}

// See documentation for repograph.RepoImpl interface.
func (g *GitStoreRepoImpl) Update(ctx context.Context) error {
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
	g.BranchList = branches
	g.Commits = commitsMap
	return nil
}

// See documentation for repograph.RepoImpl interface.
func (g *GitStoreRepoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if d, ok := g.Commits[hash]; ok {
		return d, nil
	}
	sklog.Warningf("Commit %q not found in results; performing explicit lookup.", hash)
	got, err := g.gs.Get(ctx, []string{hash})
	if err != nil {
		return nil, err
	}
	for _, c := range got {
		if c == nil {
			return nil, fmt.Errorf("Commit %s not present in GitStore.", hash)
		}
		g.Commits[c.Hash] = c
	}
	return got[0], nil
}

// See documentation for repograph.RepoImpl interface.
func (g *GitStoreRepoImpl) Branches(ctx context.Context) ([]*git.Branch, error) {
	if g.BranchList == nil {
		return nil, errors.New("Need to call Update() before Branches()")
	}
	return g.BranchList, nil
}

// See documentation for repograph.RepoImpl interface.
func (g *GitStoreRepoImpl) UpdateCallback(ctx context.Context, graph *repograph.Graph) error {
	return nil
}
