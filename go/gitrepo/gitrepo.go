package gitrepo

/*
   The gitrepo package provides an in-memory representation of an entire Git repo.
*/

import (
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	// Name of the file we store inside the Git checkout to speed up the
	// initial Update().
	CACHE_FILE = ".git/sk_gitrepo.gob"

	// We only consider branches on the "origin" remote.
	REMOTE_BRANCH_PREFIX = "origin/"
)

// Commit represents a commit in a Git repo.
type Commit struct {
	Hash      string
	Parents   []int
	repo      *Repo
	Timestamp time.Time
}

// Parents returns the parents of this commit.
func (c *Commit) GetParents() []*Commit {
	rv := make([]*Commit, 0, len(c.Parents))
	for _, idx := range c.Parents {
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
		if len(c.Parents) == 1 {
			p := c.repo.commitsData[c.Parents[0]]
			if visited[p] {
				return nil
			}
			c = p
		} else {
			break
		}
	}
	for _, parentIdx := range c.Parents {
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

// Repo represents an entire Git repo.
type Repo struct {
	branches    []*gitinfo.GitBranch
	commits     map[string]int
	commitsData []*Commit
	mtx         sync.RWMutex
	repoUrl     string
	workdir     string
}

// gobRepo is a utility struct used for serializing a Repo using gob.
type gobRepo struct {
	Commits     map[string]int
	CommitsData []*Commit
}

// NewRepo returns a Repo instance which uses the given repoUrl and workdir.
func NewRepo(repoUrl, workdir string) (*Repo, error) {
	rv := &Repo{
		commits:     map[string]int{},
		commitsData: []*Commit{},
		repoUrl:     repoUrl,
		workdir:     workdir,
	}
	if _, err := os.Stat(workdir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
				return nil, fmt.Errorf("Failed to create workdir for gitrepo.Repo: %s", err)
			}
		} else {
			return nil, fmt.Errorf("There is a problem with the workdir for gitrepo.Repo: %s", err)
		}
	}
	if _, err := os.Stat(path.Join(workdir, ".git")); err != nil {
		if os.IsNotExist(err) {
			glog.Infof("Cloning %s...", repoUrl)
			if _, err := exec.RunCwd(workdir, "git", "clone", repoUrl, "."); err != nil {
				return nil, fmt.Errorf("Failed to clone gitrepo.Repo: %s", err)
			}
		} else {
			return nil, fmt.Errorf("Failed to create gitrepo.Repo: %s", err)
		}
	}
	cacheFile := path.Join(workdir, CACHE_FILE)
	f, err := os.Open(cacheFile)
	if err == nil {
		var r gobRepo
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

func (r *Repo) addCommit(hash string) error {
	output, err := exec.RunCwd(r.workdir, "git", "log", "-n", "1", "--format=format:%P%n%ct", hash)
	if err != nil {
		return fmt.Errorf("gitrepo.Repo: Failed to obtain Git commit details: %s", err)
	}
	split := strings.Split(output, "\n")
	if len(split) != 2 {
		return fmt.Errorf("git log returned incorrect format: %s", output)
	}
	parentLine := strings.Split(split[0], " ")
	var parents []int
	if len(parentLine) > 0 {
		parentHashes := make([]int, 0, len(parentLine))
		for _, h := range parentLine {
			if h == "" {
				continue
			}
			p, ok := r.commits[h]
			if !ok {
				return fmt.Errorf("gitrepo.Repo: Could not find parent commit %q", h)
			}
			parentHashes = append(parentHashes, p)
		}
		if len(parentHashes) > 0 {
			parents = parentHashes
		}
	}
	ts, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return err
	}
	c := &Commit{
		Hash:      hash,
		Parents:   parents,
		repo:      r,
		Timestamp: time.Unix(ts, 0),
	}
	r.commits[hash] = len(r.commitsData)
	r.commitsData = append(r.commitsData, c)
	return nil
}

// Update syncs the local copy of the repo and loads new commits/branches into
// the Repo object.
func (r *Repo) Update() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Update the local copy.
	glog.Infof("Updating %s...", r.repoUrl)
	if err := exec.Run(&exec.Command{
		Name:    "git",
		Args:    []string{"fetch", "origin"},
		Dir:     r.workdir,
		Timeout: 4 * time.Minute,
	}); err != nil {
		return fmt.Errorf("Failed to update gitrepo.Repo: %s", err)
	}

	// Obtain the list of branches.
	glog.Info("  Getting branches...")
	branches, err := gitinfo.GetBranches(r.workdir)
	if err != nil {
		return fmt.Errorf("Failed to get branches for gitrepo.Repo: %s", err)
	}
	filteredBranches := make([]*gitinfo.GitBranch, 0, len(branches))
	for _, b := range branches {
		if !strings.HasPrefix(b.Name, REMOTE_BRANCH_PREFIX) {
			continue
		}
		b.Name = b.Name[len(REMOTE_BRANCH_PREFIX):]
		if b.Name == "HEAD" {
			continue
		}
		filteredBranches = append(filteredBranches, b)
	}
	r.branches = filteredBranches

	// Load all commits from the repo.
	glog.Infof("  Loading commits...")
	for _, b := range r.branches {
		output, err := exec.RunCwd(r.workdir, "git", "rev-list", REMOTE_BRANCH_PREFIX+b.Name)
		if err != nil {
			return fmt.Errorf("Failed to 'git rev-list' for gitrepo.Repo: %s", err)
		}
		split := strings.Split(output, "\n")
		for i := len(split) - 1; i >= 0; i-- {
			hash := split[i]
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
	cacheFile := path.Join(r.workdir, CACHE_FILE)
	f, err := os.Create(cacheFile)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(gobRepo{
		Commits:     r.commits,
		CommitsData: r.commitsData,
	}); err != nil {
		defer util.Close(f)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	glog.Infof("  Done.")
	return nil
}

// Branches returns the list of known branches in the repo.
func (r *Repo) Branches() []string {
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
func (r *Repo) Get(ref string) *Commit {
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
func (r *Repo) RecurseAllBranches(f func(*Commit) (bool, error)) error {
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
