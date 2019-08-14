package repograph

import (
	"context"
	"encoding/gob"
	"io"
	"os"
	"path"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// Name of the file we store inside the Git checkout to speed up the
	// initial Update().
	CACHE_FILE = "sk_gitrepo.gob"
)

// gobGraph is a utility struct used for serializing a Graph using gob.
type gobGraph struct {
	Branches []*git.Branch
	Commits  map[string]*vcsinfo.LongCommit
}

// initFromFile initializes the Graph from a file.
func initFromFile(cacheFile string) ([]*git.Branch, map[string]*vcsinfo.LongCommit, error) {
	var r gobGraph
	if err := util.MaybeReadGobFile(cacheFile, &r); err != nil {
		sklog.Errorf("Failed to read Graph cache file %s; deleting the file and starting from scratch: %s", cacheFile, err)
		if err2 := os.Remove(cacheFile); err2 != nil {
			return nil, nil, skerr.Wrapf(err, "Failed to read Graph cache file %s and failed to remove with: %s", cacheFile, err2)
		}
	}
	return r.Branches, r.Commits, nil
}

// Write the Graph to the cache file in the given Repo.
func writeCacheFile(branches []*git.Branch, commits map[string]*vcsinfo.LongCommit, cacheFile string) error {
	sklog.Infof("  Writing cache file...")
	return util.WithWriteFile(cacheFile, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(gobGraph{
			Branches: branches,
			Commits:  commits,
		})
	})
}

// localRepoImpl is an implementation of the RepoImpl interface which uses a local
// git.Repo to interact with a git repo.
type localRepoImpl struct {
	*git.Repo
	branches []*git.Branch
	commits  map[string]*vcsinfo.LongCommit
}

// NewLocalRepoImpl returns a RepoImpl backed by a local git repo.
func NewLocalRepoImpl(ctx context.Context, repoUrl, workdir string) (RepoImpl, error) {
	repo, err := git.NewRepo(ctx, repoUrl, workdir)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create git repo")
	}
	branches, commits, err := initFromFile(path.Join(repo.Dir(), CACHE_FILE))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if branches == nil {
		branches = []*git.Branch{}
	}
	if commits == nil {
		commits = map[string]*vcsinfo.LongCommit{}
	}
	return &localRepoImpl{
		Repo:     repo,
		branches: branches,
		commits:  commits,
	}, nil
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) Update(ctx context.Context) error {
	if err := r.Repo.Update(ctx); err != nil {
		return err
	}
	branches, err := r.Repo.Branches(ctx)
	if err != nil {
		return err
	}
	r.branches = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := r.commits[hash]; ok {
		return c, nil
	}
	rv, err := r.Repo.Details(ctx, hash)
	if err != nil {
		return nil, err
	}
	r.commits[hash] = rv
	return rv, nil
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) Branches(ctx context.Context) ([]*git.Branch, error) {
	return r.branches, nil
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) UpdateCallback(ctx context.Context, g *Graph) error {
	return writeCacheFile(r.branches, r.commits, path.Join(r.Dir(), CACHE_FILE))
}
