package bt_gitstore

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
)

// btGitStoreRepoImpl is an implementation of the repograph.RepoImpl interface
// used for updating the BigTable GitStore implementation. In particular, it
// allows side-loading branch heads.
type btGitStoreRepoImpl struct {
	repograph.RepoImpl
	overrideBranches []*git.Branch
}

func newRepoImplForUpdate(gs gitstore.GitStore) *btGitStoreRepoImpl {
	return &btGitStoreRepoImpl{
		RepoImpl:         gitstore.NewGitStoreRepoImpl(gs),
		overrideBranches: []*git.Branch{},
	}
}

func (ri *btGitStoreRepoImpl) Branches(ctx context.Context) ([]*git.Branch, error) {
	return ri.overrideBranches, nil
}

func (ri *btGitStoreRepoImpl) setBranches(branches map[string]string) {
	branchMap := make(map[string]string, len(ri.overrideBranches)+len(branches))
	for _, b := range ri.overrideBranches {
		branchMap[b.Name] = b.Head
	}
	for name, hash := range branches {
		branchMap[name] = hash
	}
	branchList := make([]*git.Branch, 0, len(branchMap))
	for name, hash := range branchMap {
		branchList = append(branchList, &git.Branch{
			Name: name,
			Head: hash,
		})
	}
	ri.overrideBranches = branchList
}
