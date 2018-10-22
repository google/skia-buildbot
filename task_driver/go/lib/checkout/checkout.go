package checkout

/*
   Canned steps used for checking out code in task drivers.
*/

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_scheduler/go/db"
)

// ValidateCheckout returns true if the git checkout in the given destination
// dir is in a reasonable state. Assumes that the dest dir exists.
func ValidateCheckout(ctx context.Context, dest string, rs db.RepoState) (bool, error) {
	if _, err := os_steps.Stat(ctx, filepath.Join(dest, ".git")); err == nil {
		gd := git.GitDir(dest)

		// Run "git status" and log the result, in case it's needed for
		// forensics.
		output, err := gd.Git(ctx, "status")
		if err != nil {
			// This is the first git command we've run in this
			// checkout. It could fail for a number of reasons,
			// including the checkout not actually being a checkout.
			if strings.Contains(err.Error(), "not a git repository") {
				sklog.Info("Dest dir is not a git repository.")
				return false, nil
			} else {
				return false, err
			}
		}
		sklog.Infof("Output of 'git status':\n%s", output)

		// We have a git checkout, but it might not be the right one.
		// Ensure that "origin" is set to the correct URL.
		output, err = gd.Git(ctx, "remote", "-v")
		if err != nil {
			return false, err
		}
		// Strip out any empty lines.
		lines := strings.Split(strings.TrimSpace(output), "\n")
		remotes := make([]string, 0, len(lines))
		for _, line := range lines {
			if line != "" {
				remotes = append(remotes, line)
			}
		}
		// If there's no remote, this is not the checkout we're
		// looking for.
		if len(remotes) == 0 {
			sklog.Infof("Repository has no remotes.")
			return false, nil
		} else {
			// TODO(borenet): It's possible that someone
			// (eg. bot_update) changed the remote URL to
			// point to a local cache. It would be very
			// wasteful to delete the checkout on every run.
			// Should we try to change the remote URL in
			// this case?

			// Verify that origin is set to the correct URL.
			for _, remote := range remotes {
				fields := strings.Fields(remote)
				if len(fields) != 3 {
					return false, fmt.Errorf("Got unexpected output from 'git remote -v':\n%s", output)
				}
				if fields[0] == "origin" && fields[1] != rs.Repo {
					sklog.Infof("Repository has remote 'origin' set to incorrect URL:\n%s", output)
					return false, nil
				}
			}

			// If we're still okay at this point, perform
			// some sanity checks on the checkout.
			if _, err := gd.Git(ctx, "rev-parse", "HEAD"); err != nil {
				if strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
					// Something strange is going on; take no chances.
					sklog.Infof("Unable to obtain current HEAD: %s", err)
					return false, nil
				} else {
					return false, err
				}
			}
		}
	} else if os.IsNotExist(err) {
		// If the dest dir is present but has no .git dir,
		// assume that it's in an unusable state and delete it.
		sklog.Infof("No .git dir found in %s", filepath.Join(dest, ".git"))
		return false, nil
	} else {
		return false, err
	}
	return true, nil
}

// EnsureGitCheckout obtains a clean git checkout of the given repo, at the
// given commit, in the given destination dir.
func EnsureGitCheckout(ctx context.Context, dest string, rs db.RepoState) (*git.Checkout, error) {
	ctx = td.StartStep(ctx, td.Props("Ensure Git Checkout").Infra())
	defer td.EndStep(ctx)

	if !rs.Valid() {
		return nil, td.FailStep(ctx, fmt.Errorf("Invalid RepoState: %+v", rs))
	}

	// Is the dest dir present?
	if _, err := os_steps.Stat(ctx, dest); err == nil {
		// If the dest dir is present but not in a reasonable state,
		// delete it.
		okay, err := ValidateCheckout(ctx, dest, rs)
		if err != nil {
			return nil, td.FailStep(ctx, err)
		}
		if !okay {
			sklog.Infof("Removing incompatible checkout in %s", dest)
			if err := os_steps.RemoveAll(ctx, dest); err != nil {
				return nil, td.FailStep(ctx, err)
			}
		}
	}

	// If the dest dir is not present, clone the repo into it.
	if _, err := os_steps.Stat(ctx, dest); err != nil {
		if os.IsNotExist(err) {
			sklog.Infof("Cloning %s into %s", rs.Repo, dest)
			if err := os_steps.MkdirAll(ctx, filepath.Dir(dest)); err != nil {
				return nil, td.FailStep(ctx, err)
			}
			if _, err := exec.RunCwd(ctx, filepath.Dir(dest), "git", "clone", rs.Repo, dest); err != nil {
				return nil, td.FailStep(ctx, err)
			}
		} else {
			return nil, td.FailStep(ctx, err)
		}
	}
	// Create the checkout object.
	co := &git.Checkout{GitDir: git.GitDir(dest)}

	// Now we know we have a git checkout of the correct repo in the dest
	// dir, but it could be in any state. co.Update() will forcibly clean
	// the checkout and update it to match upstream master.
	sklog.Infof("Updating git checkout")
	if err := co.Update(ctx); err != nil {
		return nil, td.FailStep(ctx, err)
	}

	// Apply a patch, or reset to the requested commit.
	if rs.IsTryJob() {
		ref := rs.GetPatchRef()
		sklog.Infof("Applying patch ref: %s", ref)
		if err := co.FetchRefFromRepo(ctx, rs.Repo, ref); err != nil {
			return nil, td.FailStep(ctx, err)
		}
		if _, err := co.Git(ctx, "reset", "--hard", "FETCH_HEAD"); err != nil {
			return nil, td.FailStep(ctx, err)
		}
		if _, err := co.Git(ctx, "rebase", rs.Revision); err != nil {
			return nil, td.FailStep(ctx, err)
		}
	} else {
		sklog.Infof("Resetting to %s", rs.Revision)
		if _, err := co.Git(ctx, "reset", "--hard", rs.Revision); err != nil {
			return nil, td.FailStep(ctx, err)
		}
	}
	return co, nil
}

// EnsureGitCheckoutWithDEPS obtains a clean git checkout of the given repo,
// at the given commit, in the given workdir, and syncs the DEPS as well. The
// checkout itself will be a subdirectory of the workdir.
func EnsureGitCheckoutWithDEPS(ctx context.Context, workdir string, rs db.RepoState) (co *git.Checkout, err error) {
	ctx = td.StartStep(ctx, td.Props("Ensure Git Checkout (with DEPS)").Infra())
	defer td.EndStep(ctx)
	// TODO(borenet): Implement this code using gclient or bot_update.
	return nil, td.FailStep(ctx, fmt.Errorf("NOT IMPLEMENTED"))
}
