package repograph

import (
	"context"

	"go.skia.org/infra/go/git"
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

type repoUpdater struct {
	*git.Repo
}

func (r *repoUpdater) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
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
