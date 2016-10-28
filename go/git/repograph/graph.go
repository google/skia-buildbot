package repograph

/*
   The repograph package provides an in-memory representation of an entire Git repo.
*/

import (
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// Name of the file we store inside the Git checkout to speed up the
	// initial Update().
	CACHE_FILE = "sk_gitrepo.gob"
)

// Commit represents a commit in a Git repo.
type Commit struct {
	*vcsinfo.LongCommit
	ParentIndices []int
	repo          *Graph
}

// Parents returns the parents of this commit.
func (c *Commit) GetParents() []*Commit {
	rv := make([]*Commit, 0, len(c.ParentIndices))
	for _, idx := range c.ParentIndices {
		rv = append(rv, c.repo.commitsData[idx])
	}
	return rv
}

// Recurse runs the given function recursively over commit history, starting
// at the given commit. The function accepts the current Commit as a parameter.
// Returning false from the function indicates that recursion should stop for
// the current branch, however, recursion will continue for any other branches
// until they are similarly terminated. Returning an error causes recursion to
// stop without properly terminating other branchces. The error will bubble to
// the top and be returned. Here's an example of printing out the entire
// ancestry of a given commit:
//
// commit.Recurse(func(c *Commit) (bool, error) {
// 	glog.Info(c.Hash)
// 	return true, nil
// })
func (c *Commit) Recurse(f func(*Commit) (bool, error)) error {
	return c.recurse(f, make(map[*Commit]bool, len(c.repo.commitsData)))
}

// recurse is a helper function used by Recurse.
func (c *Commit) recurse(f func(*Commit) (bool, error), visited map[*Commit]bool) error {
	// For large repos, we may not have enough stack space to recurse
	// through the whole commit history. Since most commits only have
	// one parent, avoid recursion when possible.
	for {
		visited[c] = true
		keepGoing, err := f(c)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
		if len(c.ParentIndices) == 1 {
			p := c.repo.commitsData[c.ParentIndices[0]]
			if visited[p] {
				return nil
			}
			c = p
		} else {
			break
		}
	}
	for _, parentIdx := range c.ParentIndices {
		p := c.repo.commitsData[parentIdx]
		if visited[p] {
			continue
		}
		if err := p.recurse(f, visited); err != nil {
			return err
		}
	}
	return nil
}

// Graph represents an entire Git repo.
type Graph struct {
	branches    []*git.Branch
	commits     map[string]int
	commitsData []*Commit
	mtx         sync.RWMutex
	repo        *git.Repo
}

// gobGraph is a utility struct used for serializing a Graph using gob.
type gobGraph struct {
	Commits     map[string]int
	CommitsData []*Commit
}

// New returns a Graph instance which uses the given git.Graph.
func New(repo *git.Repo) (*Graph, error) {
	rv := &Graph{
		commits:     map[string]int{},
		commitsData: []*Commit{},
		repo:        repo,
	}

	cacheFile := path.Join(repo.Dir(), CACHE_FILE)
	f, err := os.Open(cacheFile)
	if err == nil {
		var r gobGraph
		if err := gob.NewDecoder(f).Decode(&r); err != nil {
			util.Close(f)
			return nil, err
		}
		util.Close(f)
		rv.commits = r.Commits
		rv.commitsData = r.CommitsData
		for _, c := range rv.commitsData {
			c.repo = rv
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Failed to read cache file: %s", err)
	}
	if err := rv.Update(); err != nil {
		return nil, err
	}
	return rv, nil
}

// NewGraph returns a Graph instance, creating a git.Repo from the repoUrl and workdir.
func NewGraph(repoUrl, workdir string) (*Graph, error) {
	repo, err := git.NewRepo(repoUrl, workdir)
	if err != nil {
		return nil, err
	}
	return New(repo)
}

// Repo returns the underlying git.Repo object.
func (r *Graph) Repo() *git.Repo {
	return r.repo
}

func (r *Graph) addCommit(hash string) error {
	d, err := r.repo.Details(hash)
	if err != nil {
		return fmt.Errorf("repograph.Graph: Failed to obtain Git commit details: %s", err)
	}

	var parents []int
	if len(d.Parents) > 0 {
		parentIndices := make([]int, 0, len(d.Parents))
		for _, h := range d.Parents {
			if h == "" {
				continue
			}
			p, ok := r.commits[h]
			if !ok {
				return fmt.Errorf("repograph.Graph: Could not find parent commit %q", h)
			}
			parentIndices = append(parentIndices, p)
		}
		if len(parentIndices) > 0 {
			parents = parentIndices
		}
	}

	c := &Commit{
		LongCommit:    d,
		ParentIndices: parents,
		repo:          r,
	}
	r.commits[hash] = len(r.commitsData)
	r.commitsData = append(r.commitsData, c)
	return nil
}

// Update syncs the local copy of the repo and loads new commits/branches into
// the Graph object.
func (r *Graph) Update() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Update the local copy.
	glog.Infof("Updating repograph.Graph...")
	if err := r.repo.Update(); err != nil {
		return fmt.Errorf("Failed to update repograph.Graph: %s", err)
	}

	// Obtain the list of branches.
	glog.Info("  Getting branches...")
	branches, err := r.repo.Branches()
	if err != nil {
		return fmt.Errorf("Failed to get branches for repograph.Graph: %s", err)
	}
	r.branches = branches

	// Load all commits from the repo.
	glog.Infof("  Loading commits...")
	for _, b := range r.branches {
		commits, err := r.repo.RevList(b.Head)
		if err != nil {
			return fmt.Errorf("Failed to 'git rev-list' for repograph.Graph: %s", err)
		}
		for i := len(commits) - 1; i >= 0; i-- {
			hash := commits[i]
			if hash == "" {
				continue
			}
			if _, ok := r.commits[hash]; ok {
				continue
			}
			if err := r.addCommit(hash); err != nil {
				return err
			}
		}
	}

	// Write to the cache file.
	glog.Infof("  Writing cache file...")
	cacheFile := path.Join(r.repo.Dir(), CACHE_FILE)
	f, err := os.Create(cacheFile)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(gobGraph{
		Commits:     r.commits,
		CommitsData: r.commitsData,
	}); err != nil {
		defer util.Close(f)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	glog.Infof("  Done. Graph has %d commits.", len(r.commits))
	return nil
}

// Branches returns the list of known branches in the repo.
func (r *Graph) Branches() []string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	rv := make([]string, 0, len(r.branches))
	for _, b := range r.branches {
		rv = append(rv, b.Name)
	}
	return rv
}

// Get returns a Commit object for the given ref, if such a commit exists. This
// function does not understand complex ref types (eg. HEAD~3); only branch
// names and full commit hashes are accepted.
func (r *Graph) Get(ref string) *Commit {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if c, ok := r.commits[ref]; ok {
		return r.commitsData[c]
	}
	for _, b := range r.branches {
		if ref == b.Name {
			if c, ok := r.commits[b.Head]; ok {
				return r.commitsData[c]
			}
		}
	}
	return nil
}

// RecurseAllBranches runs the given function recursively over the entire commit
// history, starting at each of the known branch heads. The function accepts the
// current Commit as a parameter. Returning false from the function indicates
// that recursion should stop for the current branch, however, recursion will
// continue for any other branches until they are similarly terminated.
// Returning an error causes recursion to stop without properly terminating
// other branchces. The error will bubble to the top and be returned. Here's an
// example of printing out all of the commits in the repo:
//
// repo.RecurseAllBranches(func(c *Commit) (bool, error) {
//      glog.Info(c.Hash)
//      return true, nil
// })
func (r *Graph) RecurseAllBranches(f func(*Commit) (bool, error)) error {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	visited := make(map[*Commit]bool, len(r.commitsData))
	for _, b := range r.branches {
		c, ok := r.commits[b.Head]
		if !ok {
			return fmt.Errorf("Branch %s points to unknown commit %s", b.Name, b.Head)
		}
		if _, ok := visited[r.commitsData[c]]; !ok {
			if err := r.commitsData[c].recurse(f, visited); err != nil {
				return err
			}
		}
	}
	return nil
}
