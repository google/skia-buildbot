// poprepo is a package for populating a git repo with commits that associate
// a git commit with a buildid, a monotonically increasing number maintained
// by a external build system. This is needed because Perf only knows how
// to associate measurement values with git commits.
package poprepo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	BUILDID_FILENAME = "BUILDID"
)

// PopRepoI is the interface that PopRepo supports.
//
// It supports adding and reading BuildIDs from a git repo.
type PopRepoI interface {
	// GetLast returns the last committed buildid, its timestamp, and git hash.
	GetLast(ctx context.Context) (int64, int64, string, error)

	// Add a new buildid to the repo.
	Add(ctx context.Context, buildid, ts int64, branch string) error

	// LookupBuildID looks up a buildid and branch from the git hash.
	LookupBuildID(ctx context.Context, hash string) (int64, string, error)
}

// PopRepo implements PopRepoI.
type PopRepo struct {
	checkout  *git.Checkout
	workdir   string
	local     bool
	subdomain string
}

// NewPopRepo returns a *PopRepo that writes and reads BuildIds into the
// 'checkout'.
//
// If not 'local' then the HOME environment variable is set for running on the
// server.
func NewPopRepo(checkout *git.Checkout, local bool, subdomain string) *PopRepo {
	return &PopRepo{
		checkout:  checkout,
		workdir:   checkout.Dir(),
		local:     local,
		subdomain: subdomain,
	}
}

// GetLast returns the last buildid, the timestamp of when that buildid was
// added, and the git hash.
//
// The timestamp is seconds since the Unix epoch.
func (p *PopRepo) GetLast(ctx context.Context) (int64, int64, string, error) {
	fullpath := filepath.Join(p.checkout.Dir(), BUILDID_FILENAME)
	b, err := ioutil.ReadFile(fullpath)
	if err != nil {
		return 0, 0, "", fmt.Errorf("Unable to read file %q: %s", fullpath, err)
	}
	s := strings.TrimSpace(string(b))
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0, 0, "", fmt.Errorf("Unable to find just buildid and timestamp in: %q", s)
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("Timestamp is invalid in %q: %s", s, err)
	}
	buildid, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("BuildID is invalid in %q: %s", s, err)
	}
	hash, err := p.checkout.RevParse(ctx, "HEAD")
	if err != nil {
		return 0, 0, "", fmt.Errorf("Unable to retrieve git hash: %s", err)
	}
	return buildid, ts, hash, nil
}

// See PopRepoI.
func (p *PopRepo) LookupBuildID(ctx context.Context, hash string) (int64, string, error) {
	commit, err := p.checkout.Details(ctx, hash)
	if err != nil {
		return -1, "", fmt.Errorf("Failed looking up buildid: %s", err)
	}
	u, err := url.Parse(commit.Subject)
	if err != nil {
		return -1, "", fmt.Errorf("Commit subject was not a valid URL: %s", err)
	}
	branch := u.Query().Get("branch")
	if branch == "" {
		branch = "git_master"
	}

	id, err := strconv.ParseInt(filepath.Base(u.Path), 10, 64)
	return id, branch, err
}

// Add a new buildid and its assocatied Unix timestamp to the repo.
//
func (p *PopRepo) Add(ctx context.Context, buildid int64, ts int64, branch string) error {
	rollback := false
	defer func() {
		if !rollback {
			return
		}
		if err := p.checkout.Update(ctx); err != nil {
			sklog.Errorf("While rolling back failed Add(): Unable to update the checkout at %q: %s", p.checkout.Dir(), err)
		}
	}()

	// Need to set GIT_COMMITTER_DATE with commit call.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	output := bytes.Buffer{}
	cmd := exec.Command{
		Name:           gitExec,
		Args:           []string{"commit", "-m", fmt.Sprintf("https://%s.skia.org/r/%d?branch=%s", p.subdomain, buildid, branch), fmt.Sprintf("--date=%d", ts)},
		Env:            []string{fmt.Sprintf("GIT_COMMITTER_DATE=%d", ts)},
		Dir:            p.checkout.Dir(),
		InheritEnv:     true,
		CombinedOutput: &output,
	}
	if !p.local {
		// Should be set by server.
		cmd.Env = append(cmd.Env, fmt.Sprintf("HOME=/home/skia"))
	}

	// Also needs to confirm that the buildids are ascending, which means they should be ints.
	lastBuildID, _, _, err := p.GetLast(ctx)
	if err != nil {
		return fmt.Errorf("Couldn't get last buildid: %s", err)
	}
	if buildid <= lastBuildID {
		return fmt.Errorf("Error: buildid=%d <= lastBuildID=%d, buildid added in wrong order.", buildid, lastBuildID)
	}
	if err := ioutil.WriteFile(filepath.Join(p.checkout.Dir(), BUILDID_FILENAME), []byte(fmt.Sprintf("%d %d", buildid, ts)), 0644); err != nil {
		rollback = true
		return fmt.Errorf("Failed to write updated buildid: %s", err)
	}
	if msg, err := p.checkout.Git(ctx, "add", BUILDID_FILENAME); err != nil {
		rollback = true
		return fmt.Errorf("Failed to add updated file %q: %s", msg, err)
	}
	if err := exec.Run(ctx, &cmd); err != nil {
		rollback = true
		return fmt.Errorf("Failed to commit updated file %q: %s", output.String(), err)
	}
	fmt.Printf("git commit: %q", output.String())
	if msg, err := p.checkout.Git(ctx, "push", git.DefaultRemote, git.DefaultBranch); err != nil {
		rollback = true
		return fmt.Errorf("Failed to push updated checkout %q: %s", msg, err)
	}

	return nil
}

// Verify that PopRepo implements PopRepoI.
var _ PopRepoI = (*PopRepo)(nil)
