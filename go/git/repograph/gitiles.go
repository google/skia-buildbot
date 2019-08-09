package repograph

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/vcsinfo"
)

// gitilesRepoImpl is a RepoImpl implementation which uses Gitiles.
type gitilesRepoImpl struct {
	*gitiles.Repo
}

// NewGitilesRepoImpl returns a RepoImpl instance which is backed by GitStore.
func NewGitilesRepoImpl(repo *gitiles.Repo) RepoImpl {
	return &gitilesRepoImpl{repo}
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Update(_ context.Context) error {
	// We don't store anything locally for this RepoImpl; everything is
	// loaded dynamically from Gitiles.
	return nil
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Get(_ context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
	for _, h := range hashes {
		c, err := r.GetCommit(h)
		if err != nil {
			return nil, err
		}
		rv = append(rv, c)
	}
	return rv, nil
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Branches(_ context.Context) ([]*git.Branch, error) {
	return r.Repo.Branches()
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) UpdateCallback(_ context.Context, _ *Graph) error {
	return nil
}
