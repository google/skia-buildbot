// Package git is the minimal interface that Perf needs to interact with a Git
// repo.
package git

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

const commitNumberCacheSize = 1000

// Git is the minimal functionality Perf needs to interface to Git.
type Git struct {
	repo              *gitinfo.GitInfo
	commitNumberCache *lru.Cache
}

// New creates a new *Git from the given instance configuration.
func New(ctx context.Context, instanceConfig *config.InstanceConfig) (*Git, error) {
	repo, err := gitinfo.CloneOrUpdate(ctx, instanceConfig.GitRepoConfig.URL, instanceConfig.GitRepoConfig.Dir, false)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cache, err := lru.New(commitNumberCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Git{
		repo:              repo,
		commitNumberCache: cache,
	}, nil
}

// CommitNumberFromGitHash looks up the commit number given the git hash.
func (g *Git) CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error) {
	iCommitNumer, ok := g.commitNumberCache.Get(githash)
	if !ok {
		var err error
		index, err := g.repo.IndexOf(ctx, githash)
		if err != nil {
			if err := g.repo.Update(ctx, true, false); err != nil {
				return types.BadCommitNumber, skerr.Wrap(err)
			}
			index, err = g.repo.IndexOf(ctx, githash)
			if err != nil {
				return types.BadCommitNumber, skerr.Fmt("Failed to find githash %q.", githash)
			}
		}
		commitNumber := types.CommitNumber(index)
		_ = g.commitNumberCache.Add(githash, commitNumber)
		return commitNumber, nil
	}
	return iCommitNumer.(types.CommitNumber), nil
}
