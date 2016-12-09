// poprepo is a package for populating a git repo with commits that associate
// a git commit with a buildid, a monotonically increasing number maintained
// by a external build system. This is needed because Perf only knows how
// to associate measurement values with git commits.
package poprepo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
)

const (
	BUILDID_FILENAME = "BUILDID"
)

// PopRepoI is the interface that PopRepo supports.
//
// It supports adding and reading BuildIDs from a git repo.
type PopRepoI interface {
	// GetLast returns the last committed buildid and its timestamp.
	GetLast() (int64, int64, error)

	// Add a new buildid to the repo.
	Add(buildid, ts int64) error
}

// PopRepo implements PopRepoI.
type PopRepo struct {
	checkout *git.Checkout
	workdir  string
	repo     string
}

// NewPopRepo returns a *PopRepo that writes and reads BuildIds
// into the git repo from repoUrl. The workdir is where the repo
// is to be checked out.
func NewPopRepo(repoUrl, workdir string) (*PopRepo, error) {
	checkout, err := git.NewCheckout(repoUrl, workdir)
	if err != nil {
		return nil, fmt.Errorf("Unable to create the checkout of %q at %q: %s", repoUrl, workdir, err)
	}
	if err := checkout.Update(); err != nil {
		return nil, fmt.Errorf("Unable to update the checkout of %q at %q: %s", repoUrl, workdir, err)
	}
	return &PopRepo{
		checkout: checkout,
		workdir:  checkout.Dir(),
		repo:     repoUrl,
	}, nil
}

// GetLast returns the last buildid and the timestamp of when that buildid
// was added.
func (p *PopRepo) GetLast() (int64, int64, error) {
	fullpath := filepath.Join(p.workdir, BUILDID_FILENAME)
	b, err := ioutil.ReadFile(fullpath)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to read file %q: %s", fullpath, err)
	}
	parts := strings.Split(string(b), " ")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("Unable to find just buildid and timestamp in: %q", string(b))
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("Timestamp is invalid in %q: %s", string(b), err)
	}
	buildid, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("BuildID is invalid in %q: %s", string(b), err)
	}
	return buildid, ts, nil
}

// Add a new buildid and its assocatied Unix timestamp to the repo.
func (p *PopRepo) Add(buildid int64, ts int64) error {
	rollback := false
	defer func() {
		if !rollback {
			return
		}
		if err := p.checkout.Update(); err != nil {
			glog.Errorf("While rolling back failed Add(): Unable to update the checkout of %q at %q: %s", p.repo, p.workdir, err)
		}
	}()

	// Need to set GIT_COMMITTER_DATE with commit call.
	output := bytes.Buffer{}
	cmd := exec.Command{
		Name:           "git",
		Args:           []string{"commit", "-m", fmt.Sprintf("https://android-ingest.skia.org/r/%d", buildid), fmt.Sprintf("--date=%d", ts)},
		Env:            []string{fmt.Sprintf("GIT_COMMITTER_DATE=%d", ts)},
		Dir:            p.workdir,
		CombinedOutput: &output,
	}

	// Also needs to confirm that the buildids are ascending, which means they should be ints.
	lastBuildID, _, err := p.GetLast()
	if err != nil {
		return fmt.Errorf("Couldn't get last buildid: %s", err)
	}
	if buildid <= lastBuildID {
		return fmt.Errorf("Error: buildid=%d <= lastBuildID=%d, buildid added in wrong order.", buildid, lastBuildID)
	}
	if err := ioutil.WriteFile(filepath.Join(p.workdir, BUILDID_FILENAME), []byte(fmt.Sprintf("%d %d", buildid, ts)), 0644); err != nil {
		rollback = true
		return fmt.Errorf("Failed to write updated buildid: %s", err)
	}
	if msg, err := p.checkout.Git("add", BUILDID_FILENAME); err != nil {
		rollback = true
		return fmt.Errorf("Failed to add updated file %q: %s", msg, err)
	}
	if err := cmd.Run(); err != nil {
		rollback = true
		return fmt.Errorf("Failed to commit updated file %q: %s", output.String(), err)
	}
	if msg, err := p.checkout.Git("push", "origin", "master"); err != nil {
		rollback = true
		return fmt.Errorf("Failed to push updated checkout %q: %s", msg, err)
	}

	return nil
}

// Verify that PopRepo implements PopRepoI.
var _ PopRepoI = (*PopRepo)(nil)
