/*
	Package utils provides common utilities for Task Scheduler.
*/

package utils

import (
	"context"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	// Don't schedule on these branches.
	// WARNING: Any commit reachable from any of these branches will be
	// skipped. So, for example, if you fork a branch from head of master
	// and immediately blacklist it, no tasks will be scheduled for any
	// commits on master up to the branch point.
	// TODO(borenet): An alternative would be to only follow the first
	// parent for merge commits. That way, we could remove the checks which
	// cause this issue but still blacklist the branch as expected. The
	// downside is that we'll miss commits in the case where we fork a
	// branch, merge it back, and delete the new branch head.
	BRANCH_BLACKLIST = map[string][]string{
		common.REPO_SKIA_INTERNAL: {
			"skia-master",
		},
	}
)

// RecurseAllBranches runs the given func on every commit on all branches, with
// some Task Scheduler-specific exceptions.
func RecurseAllBranches(ctx context.Context, repoUrl string, repo *repograph.Graph, w *window.Window, fn func(string, *repograph.Graph, *repograph.Commit) error) error {
	blacklistBranches := BRANCH_BLACKLIST[repoUrl]
	blacklistCommits := make(map[*repograph.Commit]string, len(blacklistBranches))
	for _, b := range blacklistBranches {
		c := repo.Get(b)
		if c != nil {
			blacklistCommits[c] = b
		}
	}
	if err := repo.RecurseAllBranches(func(c *repograph.Commit) error {
		if blacklistBranch, ok := blacklistCommits[c]; ok {
			sklog.Infof("Skipping blacklisted branch %q", blacklistBranch)
			return repograph.ErrStopRecursing
		}
		for head, blacklistBranch := range blacklistCommits {
			isAncestor, err := repo.IsAncestor(c.Hash, head.Hash)
			if err != nil {
				return err
			} else if isAncestor {
				sklog.Infof("Skipping blacklisted branch %q (--is-ancestor)", blacklistBranch)
				return repograph.ErrStopRecursing
			}
		}
		if !w.TestCommit(repoUrl, c) {
			return repograph.ErrStopRecursing
		}
		return fn(repoUrl, repo, c)
	}); err != nil {
		return err
	}
	return nil
}
