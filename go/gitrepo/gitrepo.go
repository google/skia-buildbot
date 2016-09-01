package gitrepo

/*
   The gitrepo package provides an in-memory representation of an entire Git repo.
*/

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
)

const (
	// We only consider branches on the "origin" remote.
	REMOTE_BRANCH_PREFIX = "origin/"
)

// Commit represents a commit in a Git repo.
type Commit struct {
	Hash    string
	Parents []*Commit
}

// Repo represents an entire Git repo.
type Repo struct {
	branches []*gitinfo.GitBranch
	commits  map[string]*Commit
	mtx      sync.RWMutex
	repoUrl  string
	workdir  string
}

// NewRepo returns a Repo instance which uses the given repoUrl and workdir.
func NewRepo(repoUrl, workdir string) (*Repo, error) {
	rv := &Repo{
		commits: map[string]*Commit{},
		repoUrl: repoUrl,
		workdir: workdir,
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
	if err := rv.Update(); err != nil {
		return nil, err
	}
	return rv, nil
}

func getCommit(hash string, commits map[string]*Commit, workdir string) (*Commit, error) {
	output, err := exec.RunCwd(workdir, "git", "log", "-n", "1", "--format=format:%P", hash)
	if err != nil {
		return nil, fmt.Errorf("gitrepo.Repo: Failed to obtain Git commit details: %s", err)
	}
	parentHashes := strings.Split(strings.Trim(output, "\n"), " ")
	parents := make([]*Commit, 0, len(parentHashes))
	for _, h := range parentHashes {
		if h == "" {
			continue
		}
		p, ok := commits[h]
		if !ok {
			return nil, fmt.Errorf("gitrepo.Repo: Could not find parent commit %q", h)
		}
		parents = append(parents, p)
	}
	return &Commit{
		Hash:    hash,
		Parents: parents,
	}, nil
}

// Update syncs the local copy of the repo and loads new commits/branches into
// the Repo object.
func (r *Repo) Update() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Update the local copy.
	glog.Infof("Updating %s...", r.repoUrl)
	if _, err := exec.RunCwd(r.workdir, "git", "fetch", "origin"); err != nil {
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
			commit, err := getCommit(hash, r.commits, r.workdir)
			if err != nil {
				return err
			}
			r.commits[hash] = commit
		}
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
		return c
	}
	for _, b := range r.branches {
		if ref == b.Name {
			if c, ok := r.commits[b.Head]; ok {
				return c
			}
		}
	}
	return nil
}
