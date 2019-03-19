package repograph

import (
	"context"
	"fmt"
	"path"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
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

func (r *repoUpdater) Update(ctx context.Context, graph *Graph) error {
	// Update the local copy.
	sklog.Infof("Updating repograph.Graph...")
	if err := r.Repo.Update(ctx); err != nil {
		return fmt.Errorf("Failed to update repograph.Graph: %s", err)
	}

	// Obtain the list of branches.
	sklog.Info("  Getting branches...")
	newBranchesList, err := r.Repo.Branches(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get branches for repograph.Graph: %s", err)
	}
	newBranchesMap := make(map[string]string, len(newBranchesList))
	for _, branch := range newBranchesList {
		newBranchesMap[branch.Name] = branch.Head
	}
	graph.graphMtx.Lock()
	defer graph.graphMtx.Unlock()
	oldBranchesMap := make(map[string]string, len(graph.branches))
	for _, branch := range graph.branches {
		oldBranchesMap[branch.Name] = branch.Head
	}

	// Load new commits from the repo.
	var newCommits []*vcsinfo.LongCommit
	sklog.Infof("  Loading commits...")
	needOrphanCheck := false
	for _, branch := range newBranchesList {
		newHead := newBranchesMap[branch.Name]
		oldHead := oldBranchesMap[branch.Name]

		// Shortcut: if the branch is up-to-date, skip it.
		if newHead == oldHead {
			continue
		}

		// Load all commits on this branch.
		// First, try to load only new commits on this branch.
		var commits []string
		newBranch := true
		if oldHead != "" {
			anc, err := r.Repo.IsAncestor(ctx, oldHead, newHead)
			if err != nil {
				return err
			}
			if anc {
				commits, err = r.Repo.RevList(ctx, "--topo-order", fmt.Sprintf("%s..%s", oldHead, newHead))
				if err != nil {
					return err
				}
				newBranch = false
			} else {
				needOrphanCheck = true
			}
		}

		// If this is a new branch, or if the old branch head is not
		// reachable from the new (eg. if commit history was modified),
		// load ALL commits reachable from the branch head.
		if newBranch {
			sklog.Infof("  Branch %s is new or its history has changed; loading all commits.", branch.Name)
			commits, err = r.Repo.RevList(ctx, "--topo-order", newHead)
			if err != nil {
				return fmt.Errorf("Failed to 'git rev-list' for repograph.Graph: %s", err)
			}
		}
		for i := len(commits) - 1; i >= 0; i-- {
			hash := commits[i]
			if hash == "" {
				continue
			}
			if _, ok := graph.commits[hash]; ok {
				continue
			}
			d, err := r.Repo.Details(ctx, hash)
			if err != nil {
				return fmt.Errorf("repograph.Graph: Failed to obtain Git commit details: %s", err)
			}
			if err := graph.addCommit(d); err != nil {
				return err
			}
			newCommits = append(newCommits, d)
		}
	}
	if !needOrphanCheck {
		// Check to see whether any branches were deleted.
		for branch, _ := range oldBranchesMap {
			if _, ok := newBranchesMap[branch]; !ok {
				needOrphanCheck = true
				break
			}
		}
	}
	if needOrphanCheck {
		sklog.Warningf("History change detected; checking for orphaned commits.")
		visited := make(map[*Commit]bool, len(graph.commits))
		for _, newBranchHead := range newBranchesMap {
			// Not using Get() because graphMtx is locked.
			if err := graph.commits[newBranchHead].recurse(func(c *Commit) error {
				return nil
			}, visited); err != nil {
				return err
			}
		}
		for hash, c := range graph.commits {
			if !visited[c] {
				sklog.Warningf("Commit %s is orphaned. Removing from the Graph.", hash)
				delete(graph.commits, hash)
			}
		}
	}

	graph.branches = newBranchesList
	return graph.WriteCacheFile(path.Join(r.Dir(), CACHE_FILE))
}
