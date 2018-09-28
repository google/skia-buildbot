package test_automation

import (
	"fmt"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
)

/*
	Canned steps to be shared between test programs.
*/

// EnsureGitCheckout obtains a clean git checkout of the given repo, at the
// given commit, in the given destination dir.
func EnsureGitCheckout(s *Step, dest string, rs db.RepoState) (co *git.Checkout, err error) {
	s = s.Step().Infra().Name("Ensure Git Checkout").Start()
	defer s.Done(&err)

	if !rs.Valid() {
		return nil, fmt.Errorf("Invalid RepoState: %+v", rs)
	}

	// Is the dest dir present?
	if _, err := os.Stat(dest); err == nil {
		okay := true
		if _, err := os.Stat(path.Join(dest, ".git")); err == nil {
			// We have a git checkout, but it might not be the right
			// one. Ensure that "origin" is set to the correct URL.
			output, err := exec.RunCwd(s.Ctx(), dest, "git", "remote", "-v")
			if err != nil {
				return nil, err
			}
			remotes := strings.Split(strings.TrimSpace(output), "\n")
			for _, remote := range remotes {
				fields := strings.Fields(remote)
				if fields[0] == "origin" && fields[1] != rs.Repo {
					okay = false
				}
			}
			if !okay {
				sklog.Infof("Did not find remote %s in:\n%s", rs.Repo, output)
			}
		} else if os.IsNotExist(err) {
			// If the dest dir is present but has no .git dir,
			// assume that it's in an unusable state and delete it.
			sklog.Infof("No .git dir found in %s", path.Join(dest, ".git"))
			okay = false
		} else {
			return nil, err
		}
		if !okay {
			sklog.Infof("Removing incompatible checkout in %s", dest)
			if err := RemoveAll(s, dest); err != nil {
				return nil, err
			}
		}
	}

	// If the dest dir is missing, clone the repo into it.
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := MkdirAll(s, path.Dir(dest)); err != nil {
				return nil, err
			}
			if _, err := exec.RunCwd(s.Ctx(), path.Dir(dest), "git", "clone", rs.Repo, dest); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	// Create the checkout object.
	co = &git.Checkout{git.GitDir(dest)}

	// Now we know we have a git checkout of the correct repo in the dest
	// dir, but it could be in any state. co.Update() will forcibly clean
	// the checkout and update it to match upstream master.
	if err := co.Update(s.Ctx()); err != nil {
		return nil, err
	}

	// Apply a patch, or reset to the requested commit.
	if rs.IsTryJob() {
		if err := co.FetchRefFromRepo(s.Ctx(), rs.Repo, rs.GetPatchRef()); err != nil {
			return nil, err
		}
		if _, err := co.Git(s.Ctx(), "reset", "--hard", "FETCH_HEAD"); err != nil {
			return nil, err
		}
		if _, err := co.Git(s.Ctx(), "rebase", rs.Revision); err != nil {
			return nil, err
		}
	} else {
		if _, err := co.Git(s.Ctx(), "reset", "--hard", rs.Revision); err != nil {
			return nil, err
		}
	}
	return co, nil
}

// EnsureGitCheckoutWithDEPS obtains a clean git checkout of the given repo,
// at the given commit, in the given workdir, and syncs the DEPS as well. The
// checkout itself will be a subdirectory of the workdir.
func EnsureGitCheckoutWithDEPS(s *Step, workdir string, rs db.RepoState) (co *git.Checkout, err error) {
	s = s.Step().Infra().Name("Ensure Git Checkout (with DEPS)").Start()
	defer s.Done(&err)
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

// MkdirAll is a wrapper for os.MkdirAll.
func MkdirAll(s *Step, path string) (err error) {
	return s.Step().Infra().Name(fmt.Sprintf("MkdirAll %s", path)).Do(func(*Step) error {
		return os.MkdirAll(path, os.ModePerm)
	})
}

// RemoveAll is a wrapper for os.RemoveAll.
func RemoveAll(s *Step, path string) (err error) {
	return s.Step().Infra().Name(fmt.Sprintf("RemoveAll %s", path)).Do(func(*Step) error {
		return os.RemoveAll(path)
	})
}
