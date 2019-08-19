package repograph

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

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

// NewLocalGraph returns a Graph instance, creating a git.Repo from the repoUrl
// and workdir. May obtain cached data from a file in the git repo, but does NOT
// update the Graph; the caller is responsible for doing so before using the
// Graph if up-to-date data is required.
func NewLocalGraph(ctx context.Context, repoUrl, workdir string) (*Graph, error) {
	repo, err := git.NewRepo(ctx, repoUrl, workdir)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to sync %s", repoUrl)
	}
	cacheFile := filepath.Join(repo.Dir(), CACHE_FILE)
	ri, err := NewLocalRepoImpl(ctx, repo)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create LocalRepoImpl in %s", repo.Dir())
	}
	var graph *Graph
	err = util.WithReadFile(cacheFile, func(r io.Reader) error {
		g, err := NewFromGob(ctx, r, ri)
		if err != nil {
			if err2 := os.Remove(cacheFile); err2 != nil {
				return skerr.Wrapf(err, "Failed to read Graph cache file %s and failed to remove with: %s", cacheFile, err2)
			}
			return err
		}
		graph = g
		return nil
	})
	if os.IsNotExist(err) {
		return NewWithRepoImpl(ctx, ri)
	} else if err != nil {
		return nil, err
	}
	return graph, nil
}

// NewLocalMap returns a Map instance with Graphs for the given repo URLs.
// May obtain cached data from a file in the git repo, but does NOT update the
// Map; the caller is responsible for doing so before using the Map if
// up-to-date data is required.
func NewLocalMap(ctx context.Context, repos []string, workdir string) (Map, error) {
	rv := make(map[string]*Graph, len(repos))
	for _, r := range repos {
		g, err := NewLocalGraph(ctx, r, workdir)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create local Map in %s; failed on %s", workdir, r)
		}
		rv[r] = g
	}
	return rv, nil
}

// localRepoImpl is an implementation of the RepoImpl interface which uses a local
// git.Repo to interact with a git repo.
type localRepoImpl struct {
	*git.Repo
	branches []*git.Branch
	commits  map[string]*vcsinfo.LongCommit
}

// NewLocalRepoImpl returns a RepoImpl backed by a local git repo.
func NewLocalRepoImpl(ctx context.Context, repo *git.Repo) (RepoImpl, error) {
	return &localRepoImpl{
		Repo:     repo,
		branches: []*git.Branch{},
		commits:  map[string]*vcsinfo.LongCommit{},
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
	sklog.Infof("  Writing cache file...")
	return util.WithWriteFile(path.Join(r.Dir(), CACHE_FILE), func(w io.Writer) error {
		return g.WriteGob(w)
	})
}
