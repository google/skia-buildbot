package bt_gitstore

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
)

// btGitStoreRepoImplForUpdate is an implementation of the repograph.RepoImpl
// interface used for updating the BigTable GitStore implementation. In
// particular, it allows side-loading branch heads.
type btGitStoreRepoImplForUpdate struct {
	repograph.RepoImpl
	overrideBranches []*git.Branch
}

func newRepoImplForUpdate(ctx context.Context, gs gitstore.GitStore) (*btGitStoreRepoImplForUpdate, error) {
	ri, err := gitstore.NewGitStoreRepoImpl(ctx, gs)
	if err != nil {
		return nil, err
	}
	return &btGitStoreRepoImplForUpdate{
		RepoImpl:         ri,
		overrideBranches: []*git.Branch{},
	}, nil
}

func (ri *btGitStoreRepoImplForUpdate) Branches(ctx context.Context) ([]*git.Branch, error) {
	return ri.overrideBranches, nil
}

func (ri *btGitStoreRepoImplForUpdate) setBranches(branches map[string]string) {
	branchMap := make(map[string]string, len(ri.overrideBranches)+len(branches))
	for _, b := range ri.overrideBranches {
		branchMap[b.Name] = b.Head
	}
	for name, hash := range branches {
		if hash == "" {
			delete(branchMap, name)
		} else {
			branchMap[name] = hash
		}
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
