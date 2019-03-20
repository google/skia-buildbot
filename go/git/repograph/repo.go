package repograph

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path"

	"go.skia.org/infra/go/git"
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
	Commits  map[string]*Commit
}

// initFromFile initializes the Graph from a file.
func initFromFile(g *Graph, cacheFile string) error {
	var r gobGraph
	if err := util.MaybeReadGobFile(cacheFile, &r); err != nil {
		sklog.Errorf("Failed to read Graph cache file %s; deleting the file and starting from scratch: %s", cacheFile, err)
		if err2 := os.Remove(cacheFile); err != nil {
			return fmt.Errorf("Failed to read Graph cache file %s: %s\n...and failed to remove with: %s", cacheFile, err, err2)
		}
	}
	if r.Branches != nil {
		g.branches = r.Branches
	}
	if r.Commits != nil {
		g.commits = r.Commits
	}
	for _, c := range g.commits {
		for _, parentHash := range c.Parents {
			c.parents = append(c.parents, g.commits[parentHash])
		}
	}
	return nil
}

// Write the Graph to the cache file in the given Repo.
func writeCacheFile(g *Graph, cacheFile string) error {
	sklog.Infof("  Writing cache file...")
	g.graphMtx.RLock()
	defer g.graphMtx.RUnlock()
	return util.WithWriteFile(cacheFile, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(gobGraph{
			Branches: g.branches,
			Commits:  g.commits,
		})
	})
}

// localRepoImpl is an implementation of the RepoImpl interface which uses a local
// git.Repo to interact with a git repo.
type localRepoImpl struct {
	*git.Repo
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
	for _, hash := range hashes {
		details, err := r.Details(ctx, hash)
		if err != nil {
			return nil, err
		}
		rv = append(rv, details)
	}
	return rv, nil
}

// See documentation for RepoImpl interface.
func (r *localRepoImpl) UpdateCallback(ctx context.Context, g *Graph) error {
	return writeCacheFile(g, path.Join(r.Dir(), CACHE_FILE))
}
