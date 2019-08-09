package repograph

import (
	"context"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// gitilesRepoImpl is a RepoImpl implementation which uses Gitiles.
type gitilesRepoImpl struct {
	*gitiles.Repo
	branches []*git.Branch
	commits  map[string]*vcsinfo.LongCommit
}

// NewGitilesRepoImpl returns a RepoImpl instance which is backed by GitStore.
func NewGitilesRepoImpl(repo *gitiles.Repo) RepoImpl {
	return &gitilesRepoImpl{
		Repo: repo,
	}
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Update(_ context.Context) error {
	sklog.Infof("Updating %s from gitiles.", r.Repo.URL)
	if r.commits == nil {
		r.commits = make(map[string]*vcsinfo.LongCommit, 1024) // Just a guess.
	}
	oldBranches := make(map[string]*git.Branch, len(r.branches))
	for _, branch := range r.branches {
		oldBranches[branch.Name] = branch
	}
	branches, err := r.Repo.Branches()
	if err != nil {
		return err
	}
	for _, branch := range branches {
		oldBranch := oldBranches[branch.Name]
		if oldBranch != nil {
			// If there's nothing new, skip this branch.
			if branch.Head == oldBranch.Head {
				continue
			}
			// Find any new commits.
			commits, err := r.Repo.Log(oldBranch.Head, branch.Head)
			if err != nil {
				return err
			}
			if len(commits) > 0 {
				for _, c := range commits {
					r.commits[c.Hash] = c
				}
				// Skip the below fallback case.
				sklog.Warningf("History has changed for %s", r.Repo.URL)
				continue
			}
		}
		// This is a new branch, or history has changed. Load commits
		// in batches from Gitiles, stopping when we see a commit we've
		// seen before. It's possible that this will cause us to miss
		// commits in the case of merges (the order of commits returned
		// from Gitiles is not clearly documented), but we'll fall back
		// to performing requests for commits missing from the cache in
		// Update().
		sklog.Infof("Loading all commits for branch %s of %s", branch.Name, r.Repo.URL)
		addedCommits := 0
		if err := r.Repo.LogFnBatch(branch.Head, func(commits []*vcsinfo.LongCommit) error {
			for _, c := range commits {
				if _, ok := r.commits[c.Hash]; ok {
					return gitiles.ErrStopIteration
				}
				r.commits[c.Hash] = c
				addedCommits++
				if addedCommits%500 == 0 {
					sklog.Infof("Added %d commits so far.", addedCommits)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	r.branches = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Details(_ context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := r.commits[hash]; ok {
		return c, nil
	}
	return r.GetCommit(hash)
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) Branches(_ context.Context) ([]*git.Branch, error) {
	return r.branches, nil
}

// See documentation for RepoImpl interface.
func (r *gitilesRepoImpl) UpdateCallback(_ context.Context, _ *Graph) error {
	return nil
}
